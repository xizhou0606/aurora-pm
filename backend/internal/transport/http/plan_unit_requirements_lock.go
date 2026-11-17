package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
)

// lockPlanUnitRequirements 是「纯锁版」端点：给已过需求录入阶段但仍未锁版
// 的存量项目/版本（历史实例）直接落锁版事实，工作流状态不动、不推进任何
// 节点。与录入节点的复合锁版共用同一锁事实、防呆口径和审计；重复调用是
// 幂等的，首次锁版人与时间不会被改写。
func (s *Server) lockPlanUnitRequirements(c *gin.Context) {
	unit, project, found := s.planUnitByParam(c)
	if !found {
		return
	}
	var scopeCount int64
	if err := s.db.Model(&requirementgorm.Requirement{}).
		Where("workspace_id = ? AND version_task_new_id = ?", unit.WorkspaceID, unit.ID).
		Count(&scopeCount).Error; err != nil {
		dbFail(c)
		return
	}
	if scopeCount == 0 {
		failCode(c, http.StatusConflict, apiErrorRequirementScopeEmpty,
			"此项目还没有关联需求，请先录入或移入需求，再锁版")
		return
	}
	now := time.Now().UTC()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		locked, err := projectgorm.MarkPlanUnitRequirementsLockedTx(tx, unit.WorkspaceID, unit.ID, currentFactUserID(c), now)
		if err != nil {
			return err
		}
		if !locked {
			return nil
		}
		return s.recordMutation(tx, c, project.ID, "plan_unit", unit.ID, "requirements_locked", "需求锁版", gin.H{
			"plan_unit_id": unit.ID,
			"locked_at":    now,
			"policy":       "new_requirements_only",
			"source":       "manual_lock",
		})
	})
	if err != nil {
		if errors.Is(err, projectgorm.ErrPlanUnitMutationInProgress) {
			fail(c, http.StatusConflict, "plan unit is being modified; reload and retry")
			return
		}
		dbFail(c)
		return
	}
	var fresh projectgorm.PlanUnit
	if err := s.db.Select("id", "requirements_locked_at", "requirements_locked_by_user_id").
		Where("workspace_id = ? AND id = ?", unit.WorkspaceID, unit.ID).First(&fresh).Error; err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{
		"id":                             fresh.ID,
		"requirements_locked":            fresh.RequirementsLockedAt != nil,
		"requirements_locked_at":         fresh.RequirementsLockedAt,
		"requirements_locked_by_user_id": fresh.RequirementsLockedByUserID,
	})
}
