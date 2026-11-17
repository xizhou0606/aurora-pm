package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai/docsource"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	qatestcase "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/testcase"
)

func (s *Server) listAICapabilities(c *gin.Context) {
	ok(c, gin.H{"enabled": s.cfg.AI.Enabled, "model": s.cfg.AI.Model, "capabilities": aiCapabilitiesForResponse()})
}

// applyProjectDataFilterToAIRuns restricts an AI run query to project-scoped
// subjects the current user can access. It applies the same BuildMemberFilter
// through EXISTS subqueries on the subject tables. Project-scoped runs
// (subject_type=requirement/plan_unit) are filtered; unscoped runs are not
// returned for workspace-wide queries without a specific subject filter.
func (s *Server) applyProjectDataFilterToAIRuns(c *gin.Context, query *gorm.DB, workspaceID uint) bool {
	if s.projectAccess == nil {
		*query = *query.Where("1 = 0")
		return true
	}
	builder, ok := s.projectAccess.(project.AccessFilterBuilder)
	if !ok {
		*query = *query.Where("1 = 0")
		return true
	}
	userID := currentProjectAccessUserID(c)
	if userID == 0 {
		*query = *query.Where("1 = 0")
		return true
	}
	filterReq, err := builder.BuildMemberFilter(c.Request.Context(), "requirements", "r.project_new_id", workspaceID, userID)
	if err != nil {
		*query = *query.Where("1 = 0")
		return true
	}
	// The plan_units filter branch derives its inner alias from the caller's
	// qualifier, so use a subject-specific outer alias to avoid shadowing.
	filterPU, errPU := builder.BuildMemberFilter(c.Request.Context(), "plan_units", "pu_subject.project_new_id", workspaceID, userID)
	if errPU != nil {
		*query = *query.Where("1 = 0")
		return true
	}
	// Empty filter SQL means the user has access to all projects (admin).
	if filterReq.SQL == "" && filterPU.SQL == "" {
		return true
	}
	// Build a combined EXISTS filter: a run is accessible when its subject
	// exists and the user can access the subject's project. The subject_type
	// branch matches the one used in BuildMemberFilter's project_participants +
	// project_sets UNION.
	*query = *query.Where(`(
		(subject_type = 'requirement' AND EXISTS (
			SELECT 1 FROM requirements r
			WHERE r.id = ai_runs.subject_id AND r.workspace_id = ai_runs.workspace_id
			AND (`+filterReq.SQL+`)
		))
		OR
		(subject_type = 'plan_unit' AND EXISTS (
			SELECT 1 FROM plan_units pu_subject
			WHERE pu_subject.id = ai_runs.subject_id AND pu_subject.workspace_id = ai_runs.workspace_id
			AND (`+filterPU.SQL+`)
		))
	)`, append(filterReq.Args, filterPU.Args...)...)
	return true
}

func (s *Server) listAIRuns(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	query := s.db.Where("workspace_id = ?", workspace.ID)
	if subjectID := queryUint(c, "subject_id"); subjectID > 0 {
		query = query.Where("subject_id = ?", subjectID)
	}
	if subjectType := strings.TrimSpace(c.Query("subject_type")); subjectType != "" {
		query = query.Where("subject_type = ?", subjectType)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	if !s.applyProjectDataFilterToAIRuns(c, query, workspace.ID) {
		return
	}
	limit, offset := pagination(c, 50)
	var total int64
	query.Model(&aigorm.AIRun{}).Count(&total)
	var rows []aigorm.AIRun
	if query.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error != nil {
		dbFail(c)
		return
	}
	data := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		data = append(data, aiRunDTO(row, false))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total})
}

var errAIRuntimeDisabled = errors.New("AI runtime is disabled")

func (s *Server) createAIRun(c *gin.Context) {
	if !s.cfg.AI.Enabled {
		fail(c, http.StatusServiceUnavailable, errAIRuntimeDisabled.Error())
		return
	}
	var body struct {
		Capability         string              `json:"capability"`
		SubjectType        string              `json:"subject_type"`
		SubjectID          uint                `json:"subject_id"`
		UserPrompt         string              `json:"user_prompt"`
		WorkflowDefinition *workflowDefinition `json:"workflow_definition"`
		TestCaseDelivery   *qatestcase.Target  `json:"test_case_delivery"`
	}
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	if body.TestCaseDelivery != nil && strings.TrimSpace(body.Capability) == "qa.test_cases" {
		if !s.cfg.ObjectStorage.Enabled {
			failCode(c, http.StatusServiceUnavailable, apiErrorObjectStorageUnavailable, "object storage is unavailable")
			return
		}
		if !s.requireObjectStorageReady(c) {
			return
		}
	}
	var run aigorm.AIRun
	workspace, _ := currentWorkspace(c)
	projectID := uint(0)
	if body.SubjectType == "requirement" || body.SubjectType == "plan_unit" {
		// Resolve only the candidate lock target here. The transaction validates
		// current ownership with a locking read after acquiring the project lock.
		projectID = s.workflowSubjectProjectID(s.db, workspace.ID, body.SubjectType, body.SubjectID)
		if projectID == 0 {
			fail(c, http.StatusNotFound, "subject not found")
			return
		}
		if !s.requireProjectAccess(c, workspace.ID, projectID) {
			return
		}
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		subjectSnapshot := aiRunSubjectSnapshot{}
		if body.SubjectType == "requirement" || body.SubjectType == "plan_unit" {
			if projectID == 0 {
				return fmt.Errorf("AI run subject project not found")
			}
			if _, err := lockProjectSetTx(tx, workspace.ID, projectID); err != nil {
				return err
			}
			var err error
			subjectSnapshot, err = lockAIRunSubjectTx(
				tx, workspace.ID, body.SubjectType, body.SubjectID, projectID,
			)
			if err != nil {
				return err
			}
		}
		var createErr error
		run, createErr = s.createAIRunTxWithContext(tx, c, body.Capability, body.SubjectType, body.SubjectID, aiRunContext{
			UserPrompt: body.UserPrompt, WorkflowDefinition: body.WorkflowDefinition,
			TestCaseDelivery: body.TestCaseDelivery, SubjectSnapshot: subjectSnapshot,
		})
		return createErr
	})
	if err != nil {
		if errors.Is(err, errAIRunSubjectProjectChanged) {
			fail(c, http.StatusConflict, err.Error())
		} else if strings.Contains(err.Error(), "not found") {
			fail(c, http.StatusNotFound, err.Error())
		} else {
			bad(c, err.Error())
		}
		return
	}
	created(c, aiRunDTO(run, true))
}

var errAIRunSubjectProjectChanged = errors.New("AI run subject project changed; retry")

func (s *Server) createAIRunTx(tx *gorm.DB, c *gin.Context, capabilityKey, subjectType string, subjectID uint) (aigorm.AIRun, error) {
	return s.createAIRunTxWithContext(tx, c, capabilityKey, subjectType, subjectID, aiRunContext{})
}

type aiRunContext struct {
	RetryRun           *aigorm.AIRun
	UserPrompt         string
	WorkflowDefinition *workflowDefinition
	TestCaseDelivery   *qatestcase.Target
	SubjectSnapshot    aiRunSubjectSnapshot
}

func (s *Server) createAIRunTxWithContext(tx *gorm.DB, c *gin.Context, capabilityKey, subjectType string, subjectID uint, runContext aiRunContext) (aigorm.AIRun, error) {
	if !s.cfg.AI.Enabled {
		return aigorm.AIRun{}, errAIRuntimeDisabled
	}
	workspace, _ := currentWorkspace(c)
	capability, exists := aiCapabilityByKey(strings.TrimSpace(capabilityKey))
	if !exists || !containsString(capability.SubjectTypes, subjectType) {
		return aigorm.AIRun{}, fmt.Errorf("capability does not support subject")
	}
	userPrompt := strings.TrimSpace(runContext.UserPrompt)
	if len([]rune(userPrompt)) > 4000 {
		return aigorm.AIRun{}, fmt.Errorf("user_prompt exceeds 4000 characters")
	}
	if runContext.WorkflowDefinition != nil && (subjectType != "workflow_template" || !strings.HasPrefix(capability.Key, "workflow.")) {
		return aigorm.AIRun{}, fmt.Errorf("workflow_definition is only supported for workflow capabilities")
	}
	input := map[string]any{"capability": capability.Key, "workspace_id": workspace.ID}
	if userPrompt != "" {
		input["user_prompt"] = userPrompt
	}
	subjectVersion := uint(1)
	sourceURLs := make([]string, 0)
	projectID := uint(0)
	switch subjectType {
	case "requirement":
		var requirement requirementgorm.Requirement
		if runContext.SubjectSnapshot.requirement != nil {
			requirement = *runContext.SubjectSnapshot.requirement
		} else if tx.Where("id = ? AND workspace_id = ?", subjectID, workspace.ID).First(&requirement).Error != nil {
			return aigorm.AIRun{}, fmt.Errorf("requirement not found")
		}
		if requirement.ID != subjectID || requirement.WorkspaceID != workspace.ID {
			return aigorm.AIRun{}, fmt.Errorf("requirement not found")
		}
		subjectVersion = requirement.Version
		projectID = requirement.ProjectSetID
		input["subject"] = requirementAISnapshot(requirement)
		sourceURLs = appendUniqueNonempty(sourceURLs, requirement.ProductDocURL)
	case "plan_unit":
		var unit projectgorm.PlanUnit
		if runContext.SubjectSnapshot.planUnit != nil {
			unit = *runContext.SubjectSnapshot.planUnit
		} else if tx.Where("id = ? AND workspace_id = ?", subjectID, workspace.ID).First(&unit).Error != nil {
			return aigorm.AIRun{}, fmt.Errorf("plan unit not found")
		}
		if unit.ID != subjectID || unit.WorkspaceID != workspace.ID {
			return aigorm.AIRun{}, fmt.Errorf("plan unit not found")
		}
		input["subject"] = unit
		projectID = unit.ProjectSetID
	case "workflow_template":
		var template workflowgorm.WorkflowTemplate
		if tx.Where("id = ? AND workspace_id = ?", subjectID, workspace.ID).First(&template).Error != nil {
			return aigorm.AIRun{}, fmt.Errorf("workflow template not found")
		}
		subjectVersion = template.PublishedVersion + 1
		input["subject"] = workflowTemplateDTO(template, true)
		var definition workflowDefinition
		if runContext.WorkflowDefinition != nil {
			definition = *runContext.WorkflowDefinition
		} else {
			_ = json.Unmarshal([]byte(template.DraftDefinition), &definition)
		}
		stripWorkflowTemplateAssignees(&definition)
		input["workflow_definition"] = definition
	case "workspace":
		subjectID = workspace.ID
		input["subject"] = workspace
	default:
		return aigorm.AIRun{}, fmt.Errorf("unsupported subject_type")
	}
	if capability.Key == "qa.test_cases" {
		context, contextErr := s.aiTestCaseContext(tx, workspace.ID, subjectType, subjectID)
		if contextErr != nil {
			return aigorm.AIRun{}, contextErr
		}
		input["test_case_context"] = context
		contextRaw, marshalErr := json.Marshal(context)
		if marshalErr != nil {
			return aigorm.AIRun{}, fmt.Errorf("encode test case context: %w", marshalErr)
		}
		contextHash := sha256.Sum256(contextRaw)
		input["test_case_context_sha256"] = fmt.Sprintf("%x", contextHash[:])
		if requirements, ok := context["requirements"].([]gin.H); ok {
			for _, requirement := range requirements {
				if sourceURL, ok := requirement["product_doc_url"].(string); ok {
					sourceURLs = appendUniqueNonempty(sourceURLs, sourceURL)
				}
			}
		}
	}
	if runContext.TestCaseDelivery != nil {
		if capability.Key != "qa.test_cases" {
			return aigorm.AIRun{}, fmt.Errorf("test_case_delivery is only supported for qa.test_cases")
		}
		target := *runContext.TestCaseDelivery
		target.NodeID = strings.TrimSpace(target.NodeID)
		if err := target.Validate(); err != nil {
			return aigorm.AIRun{}, err
		}
		if err := s.validateTestCaseDeliveryTarget(tx, workspace.ID, subjectType, subjectID, target); err != nil {
			return aigorm.AIRun{}, err
		}
		input["test_case_delivery"] = target
	}
	var manifest *docsource.Manifest
	if s.documentSources != nil && docsource.Supported(capability.Key) {
		service, err := s.documentSources(tx)
		if err != nil {
			return aigorm.AIRun{}, err
		}
		principal, _ := currentPrincipal(c)
		manifest, err = service.Prepare(c.Request.Context(), docsource.ReadContext{WorkspaceID: workspace.ID, UserID: principal.UserID, FactUserID: currentFactUserID(c), Method: c.Request.Method, Route: c.FullPath()}, input)
		if err != nil {
			return aigorm.AIRun{}, err
		}
		input[docsource.ManifestKey] = manifest
	}
	raw, marshalErr := json.Marshal(input)
	if marshalErr != nil {
		return aigorm.AIRun{}, fmt.Errorf("encode AI run input: %w", marshalErr)
	}
	if capability.Key == "qa.test_cases" && len(raw) > 2<<20 {
		return aigorm.AIRun{}, fmt.Errorf("test case run input exceeds the supported 2 MiB limit; split the plan unit into smaller runs")
	}
	run := aigorm.AIRun{
		WorkspaceID: workspace.ID, Capability: capability.Key, SubjectType: subjectType,
		SubjectID: subjectID, SubjectVersion: subjectVersion, Status: "queued",
		Model: s.cfg.AI.Model, InputJSON: string(raw), RequestedByUser: currentFactUserID(c),
	}
	if previous := runContext.RetryRun; previous != nil {
		if err := lockAIRunTx(tx, workspace.ID, previous.ID); err != nil {
			return aigorm.AIRun{}, err
		}
		result := tx.Model(&aigorm.AIRun{}).Where("id = ? AND workspace_id = ? AND status IN ?", previous.ID, workspace.ID, []string{"failed", "cancelled"}).Updates(map[string]any{"input_json": run.InputJSON, "model": run.Model, "subject_version": run.SubjectVersion, "requested_by_user": run.RequestedByUser, "status": "queued", "output_json": "", "error_message": "", "completed_at": nil, "started_at": nil})
		if result.Error != nil {
			return aigorm.AIRun{}, result.Error
		}
		if result.RowsAffected != 1 {
			return aigorm.AIRun{}, errAITransition
		}
		run.ID = previous.ID
		run.CreatedAt = previous.CreatedAt
		if err := tx.Where("workspace_id = ? AND run_id = ?", workspace.ID, run.ID).Delete(&aigorm.AIRunSource{}).Error; err != nil {
			return aigorm.AIRun{}, err
		}
		if err := s.recordMutation(tx, c, projectID, "ai_run", run.ID, "retry_snapshot", capability.Name, gin.H{"previous_input": json.RawMessage(previous.InputJSON), "previous_requested_by_user_id": previous.RequestedByUser}); err != nil {
			return aigorm.AIRun{}, err
		}
	} else if err := tx.Create(&run).Error; err != nil {
		return aigorm.AIRun{}, err
	}
	if manifest != nil {
		if err := aigorm.SaveDocumentSources(tx, run, manifest); err != nil {
			return aigorm.AIRun{}, err
		}
	} else {
		for _, sourceURL := range sourceURLs {
			source := aigorm.AIRunSource{WorkspaceID: workspace.ID, RunID: run.ID, SourceType: "feishu_link", SourceURL: sourceURL}
			if err := tx.Create(&source).Error; err != nil {
				return aigorm.AIRun{}, err
			}
		}
	}
	if runContext.RetryRun != nil {
		if err := tx.Where("id = ? AND workspace_id = ?", run.ID, workspace.ID).First(&run).Error; err != nil {
			return aigorm.AIRun{}, err
		}
		return run, nil
	}
	if err := s.recordMutation(tx, c, projectID, "ai_run", run.ID, "created", capability.Name, gin.H{"capability": capability.Key, "subject_type": subjectType, "subject_id": subjectID}); err != nil {
		return aigorm.AIRun{}, err
	}
	return run, nil
}

func appendUniqueNonempty(values []string, candidate string) []string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" || containsString(values, candidate) {
		return values
	}
	return append(values, candidate)
}

func requirementAISnapshot(requirement requirementgorm.Requirement) gin.H {
	var planUnitID any
	if requirement.PlanUnitID > 0 {
		planUnitID = requirement.PlanUnitID
	}
	return gin.H{
		"id": requirement.ID, "workspace_id": requirement.WorkspaceID,
		"project_new_id": requirement.ProjectSetID, "version_task_new_id": planUnitID,
		"version": requirement.Version, "title": requirement.Title,
		"description": requirement.Description, "product_doc_url": requirement.ProductDocURL,
		"acceptance_criteria": requirement.AcceptanceCriteria,
		"priority":            requirement.Priority, "status": requirement.Status,
	}
}

func (s *Server) getAIRun(c *gin.Context) {
	run, okRow := s.aiRunByID(c)
	if !okRow {
		return
	}
	ok(c, aiRunDTO(run, true))
}

func (s *Server) streamAIRunEvents(c *gin.Context) {
	run, okRow := s.aiRunByID(c)
	if !okRow {
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	deadline := time.NewTimer(2 * time.Minute)
	ticker := time.NewTicker(time.Second)
	defer deadline.Stop()
	defer ticker.Stop()
	lastUpdated := time.Time{}
	for {
		if run.UpdatedAt.After(lastUpdated) {
			payload, _ := json.Marshal(aiRunDTO(run, true))
			if _, err := c.Writer.Write([]byte("event: run\ndata: " + string(payload) + "\n\n")); err != nil {
				return
			}
			c.Writer.Flush()
			lastUpdated = run.UpdatedAt
		}
		if containsString([]string{"completed", "applied", "failed", "cancelled"}, run.Status) {
			return
		}
		select {
		case <-c.Request.Context().Done():
			return
		case <-deadline.C:
			return
		case <-ticker.C:
			if s.db.Where("id = ? AND workspace_id = ?", run.ID, run.WorkspaceID).First(&run).Error != nil {
				return
			}
		}
	}
}
