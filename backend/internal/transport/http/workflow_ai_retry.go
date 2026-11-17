package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	aimodule "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

var errWorkflowAIRecheckChanged = errors.New("workflow AI retry or recheck is no longer available")

func (s *Server) recheckWorkflowAINode(c *gin.Context, instance workflowgorm.WorkflowInstance, definition workflowDefinition, node workflowNode, action string) {
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
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.lockWorkflowSubjectProjectTx(tx, instance, expectedProjectID); err != nil {
			return err
		}
		active := containsString(currentWorkflowNodes(instance.CurrentNodeIDs), node.ID)
		if active {
			claimed := tx.Model(&workflowgorm.WorkflowToken{}).
				Where(
					"workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
					instance.WorkspaceID, instance.ID, node.ID, "active",
				).
				Update("status", "processing")
			if claimed.Error != nil {
				return claimed.Error
			}
			if claimed.RowsAffected != 1 {
				return errWorkflowAIRecheckChanged
			}
		}
		restoreActiveToken := func() error {
			if !active {
				return nil
			}
			restored := tx.Model(&workflowgorm.WorkflowToken{}).
				Where(
					"workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
					instance.WorkspaceID, instance.ID, node.ID, "processing",
				).
				Update("status", "active")
			if restored.Error != nil {
				return restored.Error
			}
			if restored.RowsAffected != 1 {
				return errWorkflowAIRecheckChanged
			}
			return nil
		}
		var waiting workflowgorm.WorkflowNodeRun
		lookup := tx.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", instance.WorkspaceID, instance.ID, node.ID, "waiting").Order("id DESC").Limit(1).Find(&waiting)
		if lookup.Error != nil {
			return lookup.Error
		}
		if lookup.RowsAffected == 0 {
			if action != "recheck_ai" && action != "retry_ai" {
				return errWorkflowAIRecheckChanged
			}
			_, _, executed, err := latestWorkflowAINodeExecutionTx(tx, instance, node.ID)
			if err != nil {
				return err
			}
			if executed {
				if err := s.queueHistoricalWorkflowAINodeRecheckTx(tx, c, instance, node, now); err != nil {
					return err
				}
			} else if action != "retry_ai" {
				return errWorkflowAIRecheckChanged
			} else if _, err := s.queueWorkflowAINodeTx(tx, c, instance, node, now); err != nil {
				return err
			}
			if err := touchWorkflowInstanceForRunMutationTx(tx, instance, now); err != nil {
				return err
			}
			return restoreActiveToken()
		}
		var job workflowgorm.WorkflowJob
		if err := tx.Where("workspace_id = ? AND instance_id = ? AND node_run_id = ?", instance.WorkspaceID, instance.ID, waiting.ID).First(&job).Error; err != nil {
			return err
		}
		var payload struct {
			AIRunID uint `json:"ai_run_id"`
		}
		if json.Unmarshal([]byte(job.PayloadJSON), &payload) != nil || payload.AIRunID == 0 {
			return errors.New("workflow AI job payload is invalid")
		}
		var aiRun aigorm.AIRun
		if err := tx.Where("id = ? AND workspace_id = ?", payload.AIRunID, instance.WorkspaceID).First(&aiRun).Error; err != nil {
			return err
		}
		if aiRun.Status == "queued" || aiRun.Status == "running" {
			return errWorkflowAIWaiting
		}
		if aiRun.Status != "completed" && aiRun.Status != "applied" && aiRun.Status != "failed" && aiRun.Status != "cancelled" {
			return errors.New("workflow AI run has an unsupported status")
		}
		if action == "retry_ai" && aiRun.Status != "failed" && aiRun.Status != "cancelled" {
			return errWorkflowAIRecheckChanged
		}
		if action == "recheck_ai" && aiRun.Status != "completed" && aiRun.Status != "applied" {
			return errWorkflowAIRecheckChanged
		}
		if err := claimWorkflowAIRecheckTx(tx, instance, waiting, job, now); err != nil {
			return err
		}
		finishedRun := tx.Model(&workflowgorm.WorkflowNodeRun{}).
			Where("id = ? AND workspace_id = ? AND instance_id = ? AND status = ?", waiting.ID, instance.WorkspaceID, instance.ID, "processing").
			Updates(map[string]any{"status": aiRun.Status, "completed_at": now, "error_message": aiRun.ErrorMessage})
		if finishedRun.Error != nil {
			return finishedRun.Error
		}
		if finishedRun.RowsAffected != 1 {
			return errWorkflowAIRecheckChanged
		}
		finishedJob := tx.Model(&workflowgorm.WorkflowJob{}).
			Where("id = ? AND workspace_id = ? AND instance_id = ? AND status = ?", job.ID, instance.WorkspaceID, instance.ID, "processing").
			Updates(map[string]any{"status": aiRun.Status, "last_error": aiRun.ErrorMessage})
		if finishedJob.Error != nil {
			return finishedJob.Error
		}
		if finishedJob.RowsAffected != 1 {
			return errWorkflowAIRecheckChanged
		}
		if _, err := s.queueWorkflowAINodeTx(tx, c, instance, node, now); err != nil {
			return err
		}
		if err := touchWorkflowInstanceForRunMutationTx(tx, instance, now); err != nil {
			return err
		}
		return restoreActiveToken()
	})
	if err != nil {
		if respondProjectChildScopeConflict(c, err) {
			return
		}
		if errors.Is(err, errWorkflowAIWaiting) {
			fail(c, http.StatusConflict, "AI run is still processing")
			return
		}
		if errors.Is(err, errWorkflowAIRecheckChanged) {
			fail(c, http.StatusConflict, errWorkflowAIRecheckChanged.Error())
			return
		}
		if errors.Is(err, errAIRuntimeDisabled) {
			fail(c, http.StatusServiceUnavailable, errAIRuntimeDisabled.Error())
			return
		}
		if errors.Is(err, aimodule.ErrTaskPlanOwnersRequired) {
			bad(c, err.Error())
			return
		}
		dbFail(c)
		return
	}
	_ = s.db.Where("id = ? AND workspace_id = ?", instance.ID, instance.WorkspaceID).First(&instance).Error
	var runs []workflowgorm.WorkflowNodeRun
	var approvals []workflowgorm.WorkflowApproval
	_ = s.db.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).Order("id").Find(&runs).Error
	_ = s.db.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).Order("id").Find(&approvals).Error
	var template workflowgorm.WorkflowTemplate
	var version workflowgorm.WorkflowTemplateVersion
	_ = s.db.Where("id = ? AND workspace_id = ?", instance.TemplateID, instance.WorkspaceID).First(&template).Error
	_ = s.db.Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, instance.WorkspaceID).First(&version).Error
	ok(c, attachWorkflowTemplateMeta(s.workflowInstanceDTOWithDefinition(instance, runs, approvals, definition), template, version))
}

func touchWorkflowInstanceForRunMutationTx(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
	now time.Time,
) error {
	updated := tx.Model(&workflowgorm.WorkflowInstance{}).Where(
		"id = ? AND workspace_id = ? AND status = ? AND current_node_ids = ? AND node_plans_json = ?",
		instance.ID, instance.WorkspaceID, instance.Status, instance.CurrentNodeIDs, instance.NodePlansJSON,
	).Update("updated_at", now)
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return errWorkflowAIRecheckChanged
	}
	return nil
}

func claimWorkflowAIRecheckTx(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
	waiting workflowgorm.WorkflowNodeRun,
	job workflowgorm.WorkflowJob,
	now time.Time,
) error {
	claimedRun := tx.Model(&workflowgorm.WorkflowNodeRun{}).
		Where("id = ? AND workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", waiting.ID, instance.WorkspaceID, instance.ID, waiting.NodeID, "waiting").
		Update("status", "processing")
	if claimedRun.Error != nil {
		return claimedRun.Error
	}
	if claimedRun.RowsAffected != 1 {
		return errWorkflowAIRecheckChanged
	}
	claimedJob := tx.Model(&workflowgorm.WorkflowJob{}).
		Where("id = ? AND workspace_id = ? AND instance_id = ? AND node_run_id = ? AND status = ?", job.ID, instance.WorkspaceID, instance.ID, waiting.ID, "pending").
		Updates(map[string]any{"status": "processing", "claimed_at": now, "claimed_by": "workflow-action"})
	if claimedJob.Error != nil {
		return claimedJob.Error
	}
	if claimedJob.RowsAffected != 1 {
		return errWorkflowAIRecheckChanged
	}
	return nil
}

func (s *Server) queueHistoricalWorkflowAINodeRecheckTx(
	tx *gorm.DB,
	c *gin.Context,
	instance workflowgorm.WorkflowInstance,
	node workflowNode,
	now time.Time,
) error {
	var previous workflowgorm.WorkflowNodeRun
	lookup := tx.Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ? AND status IN ?",
		instance.WorkspaceID, instance.ID, node.ID, []string{"completed", "failed", "cancelled", "applied"},
	).Order("id DESC").Limit(1).Find(&previous)
	if lookup.Error != nil {
		return lookup.Error
	}
	if lookup.RowsAffected != 1 {
		return errWorkflowAIRecheckChanged
	}
	var previousJob workflowgorm.WorkflowJob
	if err := tx.Where(
		"workspace_id = ? AND instance_id = ? AND node_run_id = ?",
		instance.WorkspaceID, instance.ID, previous.ID,
	).First(&previousJob).Error; err != nil {
		return err
	}
	claimed := tx.Model(&workflowgorm.WorkflowJob{}).
		Where(
			"id = ? AND workspace_id = ? AND instance_id = ? AND status IN ?",
			previousJob.ID, instance.WorkspaceID, instance.ID,
			[]string{"completed", "failed", "cancelled", "applied"},
		).
		Updates(map[string]any{"status": "processing", "claimed_at": now, "claimed_by": "workflow-recheck"})
	if claimed.Error != nil {
		return claimed.Error
	}
	if claimed.RowsAffected != 1 {
		return errWorkflowAIRecheckChanged
	}
	if _, err := s.queueWorkflowAINodeTx(tx, c, instance, node, now); err != nil {
		return err
	}
	completed := tx.Model(&workflowgorm.WorkflowJob{}).
		Where("id = ? AND workspace_id = ? AND status = ?", previousJob.ID, instance.WorkspaceID, "processing").
		Update("status", "rechecked")
	if completed.Error != nil {
		return completed.Error
	}
	if completed.RowsAffected != 1 {
		return errWorkflowAIRecheckChanged
	}
	return nil
}
