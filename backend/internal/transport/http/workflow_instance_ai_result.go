package httpapi

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

type workflowAIJobPayload struct {
	AIRunID uint   `json:"ai_run_id"`
	NodeID  string `json:"node_id"`
}

// workflowAIExecutions joins workflow jobs back to their AI runs so the
// workflow detail can present the result at the node that produced it.
func (s *Server) workflowAIExecutions(instance workflowgorm.WorkflowInstance) map[string]gin.H {
	var jobs []workflowgorm.WorkflowJob
	if s.db.Where("workspace_id = ? AND instance_id = ? AND kind = ?", instance.WorkspaceID, instance.ID, "ai_run").
		Order("id").Find(&jobs).Error != nil || len(jobs) == 0 {
		return nil
	}
	type executionLink struct {
		Payload workflowAIJobPayload
	}
	links := make(map[uint]executionLink, len(jobs))
	ids := make([]uint, 0, len(jobs))
	for _, job := range jobs {
		var payload workflowAIJobPayload
		if json.Unmarshal([]byte(job.PayloadJSON), &payload) != nil || payload.AIRunID == 0 || strings.TrimSpace(payload.NodeID) == "" {
			continue
		}
		payload.NodeID = strings.TrimSpace(payload.NodeID)
		links[payload.AIRunID] = executionLink{Payload: payload}
		ids = append(ids, payload.AIRunID)
	}
	if len(ids) == 0 {
		return nil
	}
	var runs []aigorm.AIRun
	if s.db.Where("workspace_id = ? AND id IN ?", instance.WorkspaceID, ids).Order("id").Find(&runs).Error != nil {
		return nil
	}
	result := make(map[string]gin.H, len(runs))
	for _, run := range runs {
		link, ok := links[run.ID]
		if !ok {
			continue
		}
		execution := gin.H{
			"run_id": run.ID, "status": run.Status, "model": run.Model, "capability": run.Capability,
			"subject_version": run.SubjectVersion, "error_message": run.ErrorMessage, "started_at": run.StartedAt,
			"completed_at": run.CompletedAt, "updated_at": run.UpdatedAt,
		}
		if strings.TrimSpace(run.OutputJSON) != "" {
			var output any
			if json.Unmarshal([]byte(run.OutputJSON), &output) == nil {
				execution["output"] = output
			}
		}
		result[link.Payload.NodeID] = execution
	}
	return result
}
