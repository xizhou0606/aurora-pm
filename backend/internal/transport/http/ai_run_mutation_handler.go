package httpapi

import (
	"encoding/json"
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai/docsource"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task/plan"
	qatestcase "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/testcase"
)

func (s *Server) applyAIRun(c *gin.Context) {
	run, okRow := s.aiRunByID(c)
	if !okRow {
		return
	}
	if run.Status != "completed" {
		fail(c, http.StatusConflict, "AI run is not completed")
		return
	}
	if run.SubjectType != "requirement" {
		fail(c, http.StatusConflict, "this artifact has no directly applicable fields")
		return
	}
	if run.Capability == taskplan.Capability {
		s.applyTaskPlanAIRun(c, run)
		return
	}
	capability, _ := aiCapabilityByKey(run.Capability)
	allowed := map[string]bool{}
	for _, field := range capability.AllowedFields {
		allowed[field] = true
	}
	var output struct {
		Changes []struct {
			Field string `json:"field"`
			Value any    `json:"value"`
		} `json:"changes"`
	}
	if json.Unmarshal([]byte(run.OutputJSON), &output) != nil {
		fail(c, http.StatusConflict, "AI output is invalid")
		return
	}
	updates := map[string]any{}
	for _, change := range output.Changes {
		if change.Field == "status" {
			fail(c, http.StatusUnprocessableEntity, "AI output contains a workflow-managed field")
			return
		}
		if !allowed[change.Field] {
			fail(c, http.StatusUnprocessableEntity, "AI output contains a forbidden field")
			return
		}
		value, valid := validAIRequirementField(change.Field, change.Value)
		if !valid {
			fail(c, http.StatusUnprocessableEntity, "AI output contains an invalid field value")
			return
		}
		updates[change.Field] = value
	}
	if len(updates) == 0 {
		fail(c, http.StatusUnprocessableEntity, "AI output contains no applicable changes")
		return
	}
	updates["version"] = gorm.Expr("version + 1")
	now, userID := time.Now().UTC(), currentFactUserID(c)
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&requirementgorm.Requirement{}).Where("id = ? AND workspace_id = ? AND version = ?", run.SubjectID, run.WorkspaceID, run.SubjectVersion).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errRequirementConflict
		}
		raw, _ := json.Marshal(updates)
		var artifact aigorm.AIArtifact
		if err := tx.Where("workspace_id = ? AND run_id = ?", run.WorkspaceID, run.ID).Order("id DESC").First(&artifact).Error; err != nil {
			return err
		}
		application := aigorm.AIArtifactApplication{WorkspaceID: run.WorkspaceID, RunID: run.ID, ArtifactID: artifact.ID, SubjectVersion: run.SubjectVersion, ChangesJSON: string(raw), AppliedByUser: userID}
		if err := tx.Create(&application).Error; err != nil {
			return err
		}
		updatedRun := tx.Model(&aigorm.AIRun{}).
			Where("id = ? AND workspace_id = ? AND status = ?", run.ID, run.WorkspaceID, "completed").
			Updates(map[string]any{"status": "applied", "applied_by_user_id": userID, "applied_at": now})
		if updatedRun.Error != nil {
			return updatedRun.Error
		}
		if updatedRun.RowsAffected != 1 {
			return errAITransition
		}
		var requirement requirementgorm.Requirement
		if err := tx.Where("id = ? AND workspace_id = ?", run.SubjectID, run.WorkspaceID).First(&requirement).Error; err != nil {
			return err
		}
		return s.recordMutation(tx, c, requirement.ProjectSetID, "requirement", requirement.ID, "ai_applied", requirement.Title, gin.H{"run_id": run.ID, "capability": run.Capability, "fields": allowedFieldKeys(updates)})
	}); err != nil {
		if errors.Is(err, errRequirementConflict) {
			fail(c, http.StatusConflict, "subject changed since AI generation; rerun before applying")
		} else if errors.Is(err, errAITransition) {
			fail(c, http.StatusConflict, "AI run changed before the result was applied")
		} else {
			dbFail(c)
		}
		return
	}
	_ = s.db.Where("id = ? AND workspace_id = ?", run.ID, run.WorkspaceID).First(&run).Error
	ok(c, aiRunDTO(run, true))
}

func validAIRequirementField(field string, value any) (any, bool) {
	text, ok := value.(string)
	if !ok {
		return nil, false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, false
	}
	if field == "priority" && !containsString([]string{"low", "normal", "high", "urgent"}, text) {
		return nil, false
	}
	return text, true
}

func allowedFieldKeys(updates map[string]any) []string {
	keys := make([]string, 0, len(updates))
	for key := range updates {
		if key != "version" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func (s *Server) feedbackAIRun(c *gin.Context) {
	run, okRow := s.aiRunByID(c)
	if !okRow {
		return
	}
	var body struct {
		Rating  string `json:"rating"`
		Comment string `json:"comment"`
	}
	if c.ShouldBindJSON(&body) != nil || (body.Rating != "up" && body.Rating != "down") {
		bad(c, "rating must be up or down")
		return
	}
	row := aigorm.AIFeedback{WorkspaceID: run.WorkspaceID, RunID: run.ID, UserID: currentFactUserID(c), Rating: body.Rating, Comment: strings.TrimSpace(body.Comment)}
	if s.db.Transaction(func(tx *gorm.DB) error {
		if err := lockAIRunTx(tx, run.WorkspaceID, run.ID); err != nil {
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return s.recordMutation(tx, c, aiRunProjectID(tx, run), "ai_run", run.ID, "feedback", run.Capability, gin.H{"rating": row.Rating})
	}) != nil {
		dbFail(c)
		return
	}
	created(c, row)
}

func (s *Server) retryAIRun(c *gin.Context) {
	run, okRow := s.aiRunByID(c)
	if !okRow {
		return
	}
	if !containsString([]string{"failed", "cancelled"}, run.Status) {
		fail(c, http.StatusConflict, "AI run cannot transition from current status")
		return
	}
	var deliveryTarget *qatestcase.Target
	if run.Capability == "qa.test_cases" {
		target, found, err := qatestcase.ParseTarget(run.InputJSON)
		if err != nil {
			fail(c, http.StatusConflict, "AI run delivery target is invalid")
			return
		}
		if found {
			deliveryTarget = &target
			if !s.cfg.ObjectStorage.Enabled {
				failCode(c, http.StatusServiceUnavailable, apiErrorObjectStorageUnavailable, "object storage is unavailable")
				return
			}
			if !s.requireObjectStorageReady(c) {
				return
			}
		}
	}
	if s.documentSources != nil && docsource.Supported(run.Capability) {
		var previousInput struct {
			UserPrompt string `json:"user_prompt"`
		}
		if json.Unmarshal([]byte(run.InputJSON), &previousInput) != nil {
			bad(c, "AI run input is invalid")
			return
		}
		var retried aigorm.AIRun
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var err error
			retried, err = s.createAIRunTxWithContext(tx, c, run.Capability, run.SubjectType, run.SubjectID, aiRunContext{UserPrompt: previousInput.UserPrompt, TestCaseDelivery: deliveryTarget, RetryRun: &run})
			return err
		})
		if err != nil {
			fail(c, http.StatusConflict, err.Error())
			return
		}
		ok(c, aiRunDTO(retried, true))
		return
	}
	s.updateAIRunStatusRow(c, run, []string{"failed", "cancelled"}, "queued", deliveryTarget)
}
func (s *Server) cancelAIRun(c *gin.Context) {
	s.updateAIRunStatus(c, []string{"queued", "running"}, "cancelled")
}

func (s *Server) updateAIRunStatus(c *gin.Context, from []string, to string) {
	run, okRow := s.aiRunByID(c)
	if !okRow {
		return
	}
	s.updateAIRunStatusRow(c, run, from, to, nil)
}

func (s *Server) updateAIRunStatusRow(c *gin.Context, run aigorm.AIRun, from []string, to string, deliveryTarget *qatestcase.Target) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if deliveryTarget != nil {
			if validationErr := s.validateTestCaseDeliveryTarget(
				tx, run.WorkspaceID, run.SubjectType, run.SubjectID, *deliveryTarget,
			); validationErr != nil {
				return &aiDeliveryRetryValidationError{cause: validationErr}
			}
		}
		updates := map[string]any{"status": to, "error_message": ""}
		if to == "queued" {
			updates["completed_at"], updates["output_json"] = nil, ""
		}
		if to == "cancelled" {
			updates["completed_at"] = time.Now().UTC()
		}
		result := tx.Model(&aigorm.AIRun{}).Where("id = ? AND workspace_id = ? AND status IN ?", run.ID, run.WorkspaceID, from).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errAITransition
		}
		return s.recordMutation(tx, c, aiRunProjectID(tx, run), "ai_run", run.ID, to, run.Capability, gin.H{"from": run.Status, "to": to})
	})
	var validationErr *aiDeliveryRetryValidationError
	if errors.As(err, &validationErr) {
		fail(c, http.StatusConflict, "test case delivery target is no longer valid")
		return
	}
	if err != nil && !errors.Is(err, errAITransition) {
		dbFail(c)
		return
	}
	if errors.Is(err, errAITransition) {
		fail(c, http.StatusConflict, "AI run cannot transition from current status")
		return
	}
	_ = s.db.Where("id = ? AND workspace_id = ?", run.ID, run.WorkspaceID).First(&run).Error
	ok(c, aiRunDTO(run, true))
}

type aiDeliveryRetryValidationError struct {
	cause error
}

func (err *aiDeliveryRetryValidationError) Error() string {
	return "test case delivery retry validation failed: " + err.cause.Error()
}

func aiRunProjectID(tx *gorm.DB, run aigorm.AIRun) uint {
	if run.SubjectType == "requirement" {
		var row requirementgorm.Requirement
		if tx.Select("project_new_id").Where("id = ? AND workspace_id = ?", run.SubjectID, run.WorkspaceID).First(&row).Error == nil {
			return row.ProjectSetID
		}
	}
	if run.SubjectType == "plan_unit" {
		var row projectgorm.PlanUnit
		if tx.Select("project_new_id").Where("id = ? AND workspace_id = ?", run.SubjectID, run.WorkspaceID).First(&row).Error == nil {
			return row.ProjectSetID
		}
	}
	return 0
}

var errAITransition = errors.New("AI run transition conflict")

func (s *Server) aiRunByID(c *gin.Context) (aigorm.AIRun, bool) {
	id, okID := paramID(c, "id")
	if !okID {
		return aigorm.AIRun{}, false
	}
	workspace, _ := currentWorkspace(c)
	var run aigorm.AIRun
	if s.db.Where("id = ? AND workspace_id = ?", id, workspace.ID).First(&run).Error != nil {
		fail(c, http.StatusNotFound, "AI run not found")
		return aigorm.AIRun{}, false
	}
	if !s.aiRunAccessible(c, run) {
		return aigorm.AIRun{}, false
	}
	return run, true
}

// aiRunAccessible verifies the current user can access the AI run's project
// scope when the subject is project-scoped. Non-project subject types pass
// through (they are visible to workspace members).
func (s *Server) aiRunAccessible(c *gin.Context, run aigorm.AIRun) bool {
	if run.SubjectType != "requirement" && run.SubjectType != "plan_unit" {
		return true
	}
	workspace, _ := currentWorkspace(c)
	projectID := aiRunProjectID(s.db, run)
	if projectID == 0 {
		fail(c, http.StatusNotFound, "AI run subject not found")
		return false
	}
	return s.requireProjectAccess(c, workspace.ID, projectID)
}

func aiRunDTO(run aigorm.AIRun, includePayload bool) gin.H {
	dto := gin.H{"id": run.ID, "workspace_id": run.WorkspaceID, "capability": run.Capability, "subject_type": run.SubjectType, "subject_id": run.SubjectID, "subject_version": run.SubjectVersion, "status": run.Status, "model": run.Model, "requested_by_user_id": run.RequestedByUser, "error_message": run.ErrorMessage, "created_at": run.CreatedAt, "updated_at": run.UpdatedAt, "started_at": run.StartedAt, "completed_at": run.CompletedAt, "applied_at": run.AppliedAt}
	if run.Capability == "qa.test_cases" {
		dto["test_case_delivery"] = nil
		if target, found, err := qatestcase.ParseTarget(run.InputJSON); err == nil && found {
			dto["test_case_delivery"] = target
		}
	}
	if includePayload {
		var input, output any
		_ = json.Unmarshal([]byte(run.InputJSON), &input)
		_ = json.Unmarshal([]byte(run.OutputJSON), &output)
		if values, ok := input.(map[string]any); ok {
			delete(values, docsource.ManifestKey)
		}
		dto["input"] = input
		dto["output"] = output
	}
	return dto
}
