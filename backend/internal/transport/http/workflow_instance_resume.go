package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

var errWorkflowAlreadyResumed = errors.New("workflow is no longer rejected")

// resumeRejectedWorkflowInstance repairs instances terminated by the legacy reject behavior.
// It preserves every historical run and approval, then follows the immutable version's reject edge.
func (s *Server) resumeRejectedWorkflowInstance(c *gin.Context, instance workflowgorm.WorkflowInstance, definition workflowDefinition) {
	var rejectedRun workflowgorm.WorkflowNodeRun
	lookup := s.db.Where(
		"workspace_id = ? AND instance_id = ? AND status = ?",
		instance.WorkspaceID, instance.ID, "rejected",
	).Order("id DESC").First(&rejectedRun)
	if lookup.Error != nil {
		if errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			fail(c, http.StatusConflict, "rejected workflow has no rejection history")
			return
		}
		dbFail(c)
		return
	}
	if !s.requireWorkflowNodeActive(c, instance, rejectedRun.NodeID, false) {
		return
	}
	targets, err := s.workflowNextNodesForInstance(definition, instance, rejectedRun.NodeID, "reject")
	if err != nil {
		fail(c, http.StatusConflict, err.Error())
		return
	}
	if len(targets) == 0 {
		fail(c, http.StatusConflict, "rejected workflow has no configured return path")
		return
	}

	expectedProjectID, projectErr := workflowSubjectProjectIDForGate(
		s.db, instance.WorkspaceID, instance.SubjectType, instance.SubjectID,
	)
	if projectErr != nil {
		if !respondProjectChildScopeConflict(c, projectErr) {
			dbFail(c)
		}
		return
	}
	now := time.Now().UTC()
	expectedStatus, expectedCurrent, expectedPlans := instance.Status, instance.CurrentNodeIDs, instance.NodePlansJSON
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.lockWorkflowSubjectProjectTx(tx, instance, expectedProjectID); err != nil {
			return err
		}
		instance.Status = "running"
		instance.CompletedAt = nil
		current, err := s.advanceAutomaticWorkflowNodesTx(tx, &instance, definition, uniqueStrings(targets), now)
		if err != nil {
			return err
		}
		encoded, _ := json.Marshal(current)
		updated := tx.Model(&workflowgorm.WorkflowInstance{}).
			Where(
				"id = ? AND workspace_id = ? AND status = ? AND current_node_ids = ? AND node_plans_json = ?",
				instance.ID, instance.WorkspaceID, expectedStatus, expectedCurrent, expectedPlans,
			).
			Updates(map[string]any{"status": instance.Status, "current_node_ids": string(encoded), "completed_at": nil})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errWorkflowAlreadyResumed
		}
		for _, nodeID := range current {
			var active int64
			if err := tx.Model(&workflowgorm.WorkflowToken{}).
				Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status IN ?", instance.WorkspaceID, instance.ID, nodeID, []string{"active", "waiting"}).
				Count(&active).Error; err != nil {
				return err
			}
			if active == 0 {
				status := "active"
				if node, found := workflowNodeByID(definition, nodeID); found && node.Type == "parallel_join" && !s.workflowJoinReady(tx, definition, instance, nodeID) {
					status = "waiting"
				}
				if err := createWorkflowTokenTx(tx, instance, definition, nodeID, status, true); err != nil {
					return err
				}
			}
		}
		if err := syncWorkflowSubjectStatus(tx, instance, definition, current); err != nil {
			return err
		}
		if err := s.queueEnteredWorkflowAINodesTx(tx, c, instance, definition, current, now); err != nil {
			return err
		}
		return s.recordMutation(tx, c, expectedProjectID, "workflow_instance", instance.ID, "resumed", "恢复已驳回流程", gin.H{
			"rejected_node_id": rejectedRun.NodeID,
			"target_node_ids":  current,
		})
	}); err != nil {
		if respondProjectChildScopeConflict(c, err) {
			return
		}
		if errors.Is(err, errWorkflowAlreadyResumed) {
			fail(c, http.StatusConflict, err.Error())
			return
		}
		if workflowNodePlanDatabaseConflict(err) {
			fail(c, http.StatusConflict, "workflow changed before resume")
			return
		}
		dbFail(c)
		return
	}

	_ = s.db.Where("id = ? AND workspace_id = ?", instance.ID, instance.WorkspaceID).First(&instance).Error
	var runs []workflowgorm.WorkflowNodeRun
	var approvals []workflowgorm.WorkflowApproval
	var template workflowgorm.WorkflowTemplate
	var version workflowgorm.WorkflowTemplateVersion
	_ = s.db.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).Order("id").Find(&runs).Error
	_ = s.db.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).Order("id").Find(&approvals).Error
	_ = s.db.Where("id = ? AND workspace_id = ?", instance.TemplateID, instance.WorkspaceID).First(&template).Error
	_ = s.db.Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, instance.WorkspaceID).First(&version).Error
	ok(c, attachWorkflowTemplateMeta(s.workflowInstanceDTOWithDefinition(instance, runs, approvals, definition), template, version))
}
