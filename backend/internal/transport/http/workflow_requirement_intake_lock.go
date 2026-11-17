package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

// workflowNodeRequiresRequirementIntakeLock reports whether completing the
// node is a project requirement-intake lock confirmation. Tri-state: explicit
// requirement_intake_lock wins; when unset, historical name compatibility
// applies (requirement_input or a 需求录入/需求收集 name), and an explicit
// false opts a node out of the name fallback.
func workflowNodeRequiresRequirementIntakeLock(instance workflowgorm.WorkflowInstance, node workflowNode) bool {
	if instance.SubjectType != "plan_unit" {
		return false
	}
	if node.Type != "human_task" && node.Type != "form" {
		return false
	}
	if raw, exists := node.Config["requirement_intake_lock"]; exists {
		required, _ := raw.(bool)
		return required
	}
	return workflowNodeCollectsRequirementInput(node)
}

// guardWorkflowRequirementIntakeLock enforces the second-confirmation contract
// before any business data changes: a qualifying pending intake node must
// carry confirm_requirement_lock=true, and the flag is rejected everywhere
// else so it can never be smuggled through unrelated actions or nodes.
func (s *Server) guardWorkflowRequirementIntakeLock(
	c *gin.Context,
	instance workflowgorm.WorkflowInstance,
	node workflowNode,
	action string,
	confirm bool,
) bool {
	qualifies := action == "complete" && workflowNodeRequiresRequirementIntakeLock(instance, node)
	if confirm && !qualifies {
		bad(c, "当前节点不支持锁版确认，请勿携带 confirm_requirement_lock")
		return false
	}
	if !qualifies {
		return true
	}
	var unit projectgorm.PlanUnit
	if s.db.Select("requirements_locked_at").
		Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).
		First(&unit).Error != nil {
		fail(c, http.StatusConflict, "plan unit is unavailable")
		return false
	}
	if unit.RequirementsLockedAt == nil && !confirm {
		failCode(c, http.StatusConflict, apiErrorRequirementLockConfirmation,
			"请确认锁版后再完成需求录入；确认后该项目不能新建需求，已有需求仍可移入或排期")
		return false
	}
	// 空范围防呆：完成需求录入即声明"本次录入结束"，范围为空时锁版会让
	// 后续阶段门禁永远等待空集合，直接拦截并提示先录入需求。
	var scopeCount int64
	if err := s.db.Model(&requirementgorm.Requirement{}).
		Where("workspace_id = ? AND version_task_new_id = ?", instance.WorkspaceID, instance.SubjectID).
		Count(&scopeCount).Error; err != nil {
		dbFail(c)
		return false
	}
	if scopeCount == 0 {
		failCode(c, http.StatusConflict, apiErrorRequirementScopeEmpty,
			"此项目还没有关联需求，请先录入或移入需求，再完成需求录入并锁版")
		return false
	}
	return true
}

// lockRequirementIntakeTx commits the plan-unit lock inside the node
// completion transaction. The plan-unit row FOR UPDATE is the shared
// serialization point with requirement creation: whichever transaction commits
// first decides the outcome. An already-locked plan unit is left untouched so
// rollbacks and re-completions keep the original locker and time.
func (s *Server) lockRequirementIntakeTx(
	tx *gorm.DB,
	c *gin.Context,
	instance workflowgorm.WorkflowInstance,
	node workflowNode,
	status string,
	confirm bool,
	projectID uint,
	now time.Time,
) error {
	if !confirm || status != "completed" || !workflowNodeRequiresRequirementIntakeLock(instance, node) {
		return nil
	}
	locked, err := projectgorm.MarkPlanUnitRequirementsLockedTx(tx, instance.WorkspaceID, instance.SubjectID, currentFactUserID(c), now)
	if err != nil {
		return err
	}
	if !locked {
		return nil
	}
	return s.recordMutation(tx, c, projectID, "plan_unit", instance.SubjectID, "requirements_locked", node.Name, gin.H{
		"plan_unit_id":         instance.SubjectID,
		"workflow_instance_id": instance.ID,
		"node_id":              node.ID,
		"locked_at":            now,
		"policy":               "new_requirements_only",
	})
}
