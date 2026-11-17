package httpapi

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	qatestcase "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/testcase"
)

func (s *Server) materializeWorkflowTestCaseArtifact(c *gin.Context) {
	instance, node, _, valid := s.workflowTestCaseArtifactNode(c, false)
	if !valid {
		return
	}
	if !s.requireWorkflowNodeActive(c, instance, node.ID, containsString(currentWorkflowNodes(instance.CurrentNodeIDs), node.ID)) {
		return
	}
	var body struct {
		AIRunID uint `json:"ai_run_id"`
	}
	if c.ShouldBindJSON(&body) != nil || body.AIRunID == 0 {
		bad(c, "valid ai_run_id is required")
		return
	}
	var run aigorm.AIRun
	if s.db.Where(
		"id = ? AND workspace_id = ? AND subject_type = ? AND subject_id = ? AND capability = ?",
		body.AIRunID, instance.WorkspaceID, instance.SubjectType, instance.SubjectID, "qa.test_cases",
	).First(&run).Error != nil {
		fail(c, http.StatusConflict, "test case AI run not found")
		return
	}
	target, found, targetErr := qatestcase.ParseTarget(run.InputJSON)
	if targetErr != nil || !found || target.InstanceID != instance.ID || target.NodeID != node.ID {
		fail(c, http.StatusConflict, "AI run delivery target does not match this workflow node")
		return
	}
	if !s.requireObjectStorageReady(c) {
		return
	}
	row, createdRow, materializeErr := s.testCaseDelivery.Materialize(c.Request.Context(), run)
	if materializeErr != nil {
		switch {
		case errors.Is(materializeErr, qatestcase.ErrStorageUnavailable):
			fail(c, http.StatusServiceUnavailable, "system object storage is required for test case delivery")
		case errors.Is(materializeErr, qatestcase.ErrStorageUpload):
			log.Printf(
				"AI test-case delivery failed workspace_id=%d instance_id=%d node_id=%q ai_run_id=%d error=%q",
				instance.WorkspaceID, instance.ID, node.ID, run.ID,
				safeWorkflowArtifactUploadError(materializeErr, s.cfg.ObjectStorage.AccessKey, s.cfg.ObjectStorage.SecretKey),
			)
			failCode(c, http.StatusBadGateway, apiErrorObjectStorageUploadFailed, "test case delivery failed")
		case errors.Is(materializeErr, qatestcase.ErrRunNotEligible),
			errors.Is(materializeErr, qatestcase.ErrTargetInvalid),
			errors.Is(materializeErr, qatestcase.ErrOutputInvalid),
			errors.Is(materializeErr, qatestcase.ErrSubjectStale):
			fail(c, http.StatusConflict, materializeErr.Error())
		default:
			dbFail(c)
		}
		return
	}
	status := http.StatusOK
	if createdRow {
		status = http.StatusCreated
		_ = s.recordMutation(s.db, c, s.workflowSubjectProjectID(s.db, instance.WorkspaceID, instance.SubjectType, instance.SubjectID),
			"workflow_node_artifact", row.ID, "uploaded", row.Name, gin.H{
				"workflow_instance_id": instance.ID, "node_id": node.ID, "kind": row.Kind, "ai_run_id": run.ID,
			})
	}
	if instance.SubjectType == "requirement" {
		_, _ = s.saveRequirementMaterial(requirement.MaterialInput{
			WorkspaceID: instance.WorkspaceID, RequirementID: instance.SubjectID,
			Kind: requirement.MaterialKindTestCase, Source: requirement.MaterialSourceAIGenerated,
			Name: row.Name, ContentType: row.ContentType, Size: row.Size,
			ObjectKey: row.ObjectKey, URL: row.URL, AIRunID: row.AIRunID, UploaderUserID: row.UploaderUserID,
		})
	}
	c.JSON(status, gin.H{"data": workflowNodeArtifactDTO(row)})
}
