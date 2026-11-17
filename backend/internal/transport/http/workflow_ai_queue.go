package httpapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task/plan"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func (s *Server) queueWorkflowAINodeTx(tx *gorm.DB, c *gin.Context, instance workflowgorm.WorkflowInstance, node workflowNode, now time.Time) (workflowgorm.WorkflowNodeRun, error) {
	run := workflowgorm.WorkflowNodeRun{
		WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeID: node.ID,
		NodeType: node.Type, Status: "waiting", StartedAt: now,
	}
	if err := tx.Create(&run).Error; err != nil {
		return workflowgorm.WorkflowNodeRun{}, err
	}
	aiRun, err := s.createAIRunTx(tx, c, node.Capability, instance.SubjectType, instance.SubjectID)
	if err != nil {
		return workflowgorm.WorkflowNodeRun{}, err
	}
	payload, _ := json.Marshal(gin.H{"ai_run_id": aiRun.ID, "node_id": node.ID})
	job := workflowgorm.WorkflowJob{
		WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeRunID: run.ID,
		Kind: "ai_run", Status: "pending", PayloadJSON: string(payload),
		IdempotencyKey: fmt.Sprintf("workflow:%d:%s:%d", instance.ID, node.ID, run.ID), AvailableAt: now,
	}
	if err := tx.Create(&job).Error; err != nil {
		return workflowgorm.WorkflowNodeRun{}, err
	}
	projectID := s.workflowSubjectProjectID(tx, instance.WorkspaceID, instance.SubjectType, instance.SubjectID)
	if err := s.recordMutation(tx, c, projectID, "workflow_instance", instance.ID, "ai_queued", node.Name, gin.H{"node_id": node.ID, "ai_run_id": aiRun.ID}); err != nil {
		return workflowgorm.WorkflowNodeRun{}, err
	}
	return run, nil
}

func latestWorkflowAINodeExecutionTx(tx *gorm.DB, instance workflowgorm.WorkflowInstance, nodeID string) (workflowgorm.WorkflowJob, aigorm.AIRun, bool, error) {
	var nodeRuns []workflowgorm.WorkflowNodeRun
	if err := tx.Select("id").Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ?",
		instance.WorkspaceID, instance.ID, nodeID,
	).Order("id DESC").Find(&nodeRuns).Error; err != nil {
		return workflowgorm.WorkflowJob{}, aigorm.AIRun{}, false, err
	}
	for _, nodeRun := range nodeRuns {
		var job workflowgorm.WorkflowJob
		lookup := tx.Where(
			"workspace_id = ? AND instance_id = ? AND node_run_id = ? AND kind = ?",
			instance.WorkspaceID, instance.ID, nodeRun.ID, "ai_run",
		).Limit(1).Find(&job)
		if lookup.Error != nil {
			return workflowgorm.WorkflowJob{}, aigorm.AIRun{}, false, lookup.Error
		}
		if lookup.RowsAffected == 0 {
			continue
		}
		var payload workflowAIJobPayload
		if json.Unmarshal([]byte(job.PayloadJSON), &payload) != nil || payload.AIRunID == 0 || strings.TrimSpace(payload.NodeID) != nodeID {
			return workflowgorm.WorkflowJob{}, aigorm.AIRun{}, false, fmt.Errorf("workflow AI job payload is invalid")
		}
		var aiRun aigorm.AIRun
		if err := tx.Where("id = ? AND workspace_id = ?", payload.AIRunID, instance.WorkspaceID).First(&aiRun).Error; err != nil {
			return workflowgorm.WorkflowJob{}, aigorm.AIRun{}, false, err
		}
		return job, aiRun, true, nil
	}
	return workflowgorm.WorkflowJob{}, aigorm.AIRun{}, false, nil
}

func linkReusedWorkflowAIExecutionTx(tx *gorm.DB, instance workflowgorm.WorkflowInstance, node workflowNode, sourceJob workflowgorm.WorkflowJob, now time.Time) (workflowgorm.WorkflowNodeRun, workflowgorm.WorkflowJob, error) {
	run := workflowgorm.WorkflowNodeRun{
		WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeID: node.ID,
		NodeType: node.Type, Status: "processing", StartedAt: now,
	}
	if err := tx.Create(&run).Error; err != nil {
		return workflowgorm.WorkflowNodeRun{}, workflowgorm.WorkflowJob{}, err
	}
	job := workflowgorm.WorkflowJob{
		WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeRunID: run.ID,
		Kind: "ai_run", Status: "processing", PayloadJSON: sourceJob.PayloadJSON,
		IdempotencyKey: fmt.Sprintf("workflow:%d:%s:%d:reuse", instance.ID, node.ID, run.ID),
		AvailableAt:    now, ClaimedAt: &now, ClaimedBy: "workflow-result-reuse",
	}
	if err := tx.Create(&job).Error; err != nil {
		return workflowgorm.WorkflowNodeRun{}, workflowgorm.WorkflowJob{}, err
	}
	return run, job, nil
}

// The first visit queues AI automatically. Returning to a node keeps its
// latest execution visible until the user explicitly retries or rechecks it.
func (s *Server) queueEnteredWorkflowAINodesTx(tx *gorm.DB, c *gin.Context, instance workflowgorm.WorkflowInstance, definition workflowDefinition, current []string, now time.Time) error {
	for _, nodeID := range current {
		node, found := workflowNodeByID(definition, nodeID)
		if !found || !strings.HasPrefix(node.Type, "ai_") {
			continue
		}
		_, _, executed, err := latestWorkflowAINodeExecutionTx(tx, instance, node.ID)
		if err != nil {
			return err
		}
		if executed {
			continue
		}
		if node.Capability == taskplan.Capability {
			ready, err := requirementgorm.TaskPlanReadyTx(tx, instance.WorkspaceID, instance.SubjectType, instance.SubjectID)
			if err != nil {
				return err
			}
			if !ready {
				continue
			}
		}
		if _, err := s.queueWorkflowAINodeTx(tx, c, instance, node, now); err != nil {
			return err
		}
	}
	return nil
}

func workflowAIResultPath(capability, raw string) string {
	if capability != "requirement.completeness" {
		return "success"
	}
	var output struct {
		CompletenessReport *struct {
			CanProceed bool `json:"canProceed"`
		} `json:"completenessReport"`
	}
	if json.Unmarshal([]byte(raw), &output) != nil || output.CompletenessReport == nil || !output.CompletenessReport.CanProceed {
		return "failure"
	}
	return "success"
}
