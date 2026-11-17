package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task/plan"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

var (
	errTaskPlanApplyInvalid  = errors.New("task plan output cannot be applied")
	errTaskPlanApplyStale    = errors.New("task plan subject changed")
	errTaskPlanWorkflowBound = errors.New("task plan run belongs to a workflow")
)

// taskPlanApplyBody carries the manually chosen per-lane assignees. Tasks are
// never auto-assigned from requirement lane owners on this standalone path;
// lanes without an override (or with null) stay unassigned.
type taskPlanApplyBody struct {
	Assignees *struct {
		Frontend *uint `json:"frontend"`
		Backend  *uint `json:"backend"`
		Other    *uint `json:"other"`
	} `json:"assignees"`
}

func parseTaskPlanApplyBody(c *gin.Context) (taskPlanApplyBody, error) {
	var body taskPlanApplyBody
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return body, nil
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		return body, err
	}
	return body, nil
}

func (s *Server) validateTaskPlanAssignees(assignees map[string]*uint) error {
	unique := make(map[uint]struct{}, len(assignees))
	for _, assignee := range assignees {
		if assignee != nil && *assignee > 0 {
			unique[*assignee] = struct{}{}
		}
	}
	if len(unique) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	var count int64
	if err := s.db.Model(&identitygorm.User{}).
		Where("id IN ? AND active = ? AND status = ?", ids, true, "active").
		Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(ids)) {
		return errors.New("task plan assignee is not an active user")
	}
	return nil
}

func (s *Server) applyTaskPlanAIRun(c *gin.Context, run aigorm.AIRun) {
	body, err := parseTaskPlanApplyBody(c)
	if err != nil {
		bad(c, "invalid apply body")
		return
	}
	overrides := map[string]*uint{}
	if body.Assignees != nil {
		overrides["frontend"] = body.Assignees.Frontend
		overrides["backend"] = body.Assignees.Backend
		overrides["other"] = body.Assignees.Other
	}
	if err := s.validateTaskPlanAssignees(overrides); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	// The standalone path never auto-assigns from requirement lane owners;
	// only manually picked assignees land on created tasks.
	laneAssignee := func(lane string) *uint { return overrides[lane] }
	projectID := aiRunProjectID(s.db, run)
	if projectID == 0 {
		fail(c, http.StatusConflict, "task plan subject changed; rerun before applying")
		return
	}
	now := time.Now().UTC()
	var applied aigorm.AIRun
	err = s.db.Transaction(func(tx *gorm.DB) error {
		project, err := lockProjectSetTx(tx, run.WorkspaceID, projectID)
		if err != nil {
			return fmt.Errorf("%w: project is unavailable", errTaskPlanApplyStale)
		}
		workflowBound, err := taskPlanRunBoundToWorkflowTx(tx, run.WorkspaceID, run.ID)
		if err != nil {
			return err
		}
		if workflowBound {
			return errTaskPlanWorkflowBound
		}
		requirement, err := lockProjectRequirementTx(tx, run.WorkspaceID, run.SubjectID, project.ID)
		if err != nil || !requirementMatchesProject(requirement, project) {
			return fmt.Errorf("%w: requirement scope changed", errTaskPlanApplyStale)
		}
		applied, err = lockTaskPlanAIRunRowTx(tx, run.WorkspaceID, run.ID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errAITransition
		}
		if err != nil {
			return err
		}
		if applied.Status != "completed" {
			return errAITransition
		}
		_, err = s.applyTaskPlanRunTx(tx, c, &applied, project, requirement, now, laneAssignee)
		return err
	})
	if respondTaskPlanApplyError(c, err) {
		return
	}
	_ = s.db.Where("id = ? AND workspace_id = ?", applied.ID, applied.WorkspaceID).First(&applied).Error
	ok(c, aiRunDTO(applied, true))
}

func lockTaskPlanAIRunRowTx(tx *gorm.DB, workspaceID, runID uint) (aigorm.AIRun, error) {
	var run aigorm.AIRun
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND workspace_id = ?", runID, workspaceID).
		First(&run).Error
	return run, err
}

func taskPlanRunBoundToWorkflowTx(tx *gorm.DB, workspaceID, runID uint) (bool, error) {
	var jobs []workflowgorm.WorkflowJob
	if err := tx.Select("payload_json").
		Where("workspace_id = ? AND kind = ?", workspaceID, "ai_run").
		Find(&jobs).Error; err != nil {
		return false, err
	}
	for _, job := range jobs {
		var payload workflowAIJobPayload
		if json.Unmarshal([]byte(job.PayloadJSON), &payload) == nil && payload.AIRunID == runID {
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) applyTaskPlanRunTx(
	tx *gorm.DB,
	c *gin.Context,
	run *aigorm.AIRun,
	project projectgorm.ProjectSet,
	requirement requirementgorm.Requirement,
	now time.Time,
	laneAssignee func(lane string) *uint,
) ([]taskplan.CreatedTaskRef, error) {
	if run.Capability != taskplan.Capability || run.SubjectType != "requirement" || run.SubjectID != requirement.ID ||
		run.WorkspaceID != requirement.WorkspaceID || run.Status != "completed" {
		return nil, errAITransition
	}
	parsed, err := taskplan.ParseForSubject(run.Capability, run.SubjectType, run.SubjectID, json.RawMessage(run.OutputJSON))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errTaskPlanApplyInvalid, err)
	}
	if len(parsed.CreatedTasks) == 0 && run.SubjectVersion != requirement.Version {
		return nil, fmt.Errorf("%w: requirement version changed", errTaskPlanApplyStale)
	}
	var artifact aigorm.AIArtifact
	if err := tx.Where("workspace_id = ? AND run_id = ?", run.WorkspaceID, run.ID).Order("id DESC").First(&artifact).Error; err != nil {
		return nil, fmt.Errorf("%w: result artifact is unavailable", errTaskPlanApplyInvalid)
	}
	refs := parsed.CreatedTasks
	persistedOutput := json.RawMessage(run.OutputJSON)
	if len(refs) == 0 {
		refs, err = s.materializeTaskPlanTx(tx, c, *run, project, requirement, parsed.Plan, laneAssignee)
		if err != nil {
			return nil, err
		}
		persistedOutput, err = taskplan.AttachCreatedTasks(persistedOutput, refs)
		if err != nil {
			return nil, fmt.Errorf("%w: created task references cannot be encoded", errTaskPlanApplyInvalid)
		}
	}
	if err := tx.Model(&aigorm.AIArtifact{}).
		Where("id = ? AND workspace_id = ? AND run_id = ?", artifact.ID, run.WorkspaceID, run.ID).
		Update("content_json", string(persistedOutput)).Error; err != nil {
		return nil, err
	}
	changes, _ := json.Marshal(map[string]any{"createdTasks": refs})
	application := aigorm.AIArtifactApplication{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, ArtifactID: artifact.ID,
		SubjectVersion: run.SubjectVersion, ChangesJSON: string(changes), AppliedByUser: currentFactUserID(c),
	}
	if err := tx.Create(&application).Error; err != nil {
		return nil, err
	}
	userID := currentFactUserID(c)
	updated := tx.Model(&aigorm.AIRun{}).
		Where("id = ? AND workspace_id = ? AND status = ?", run.ID, run.WorkspaceID, "completed").
		Updates(map[string]any{
			"status": "applied", "output_json": string(persistedOutput),
			"applied_by_user_id": userID, "applied_at": now,
		})
	if updated.Error != nil {
		return nil, updated.Error
	}
	if updated.RowsAffected != 1 {
		return nil, errAITransition
	}
	run.Status, run.OutputJSON, run.AppliedByUserID, run.AppliedAt = "applied", string(persistedOutput), &userID, &now
	if err := s.recordMutation(tx, c, project.ID, "requirement", requirement.ID, "ai_applied", requirement.Title, gin.H{
		"run_id": run.ID, "capability": run.Capability, "created_task_count": len(refs),
	}); err != nil {
		return nil, err
	}
	return refs, nil
}

func (s *Server) materializeTaskPlanTx(
	tx *gorm.DB,
	c *gin.Context,
	run aigorm.AIRun,
	project projectgorm.ProjectSet,
	requirement requirementgorm.Requirement,
	plan taskplan.Plan,
	laneAssignee func(lane string) *uint,
) ([]taskplan.CreatedTaskRef, error) {
	authActor, ok := currentActor(c)
	if !ok {
		return nil, errors.New("task plan actor is unavailable")
	}
	priority := strings.ToLower(strings.TrimSpace(requirement.Priority))
	if !containsString([]string{"low", "normal", "high", "urgent"}, priority) {
		priority = "normal"
	}
	reporterID := factUserID(authActor.User)
	userIDs := []uint{reporterID}
	for _, chain := range plan.Chains {
		if assignee := laneAssignee(chain.Lane); assignee != nil {
			userIDs = append(userIDs, *assignee)
		}
	}
	notificationContext := notificationUsersTx(tx, userIDs...)
	refs := make([]taskplan.CreatedTaskRef, 0, 2+len(plan.Chains)*2)
	for _, chain := range plan.Chains {
		laneName := map[string]string{"frontend": "前端", "backend": "后端"}[chain.Lane]
		assignee := laneAssignee(chain.Lane)
		parent := taskgorm.WorkTask{
			WorkspaceID: run.WorkspaceID, TeamID: requirement.TeamID, ProjectSetID: requirement.ProjectSetID,
			ProjectID: requirement.PlanUnitID, RequirementID: requirement.ID,
			Kind: taskgorm.WorkTaskKindTask, Title: taskPlanTitle(requirement.Title + "-" + laneName),
			Description: "AI 任务拆解确认采用后创建的" + laneName + "研发链路。\n\n链路说明：" + strings.TrimSpace(chain.Summary),
			Status:      "draft", Priority: priority, ReporterUserID: &reporterID, AssigneeID: assignee,
		}
		if err := s.createPreparedTask(tx, c, authActor, notificationContext, project, &parent); err != nil {
			return nil, err
		}
		refs = append(refs, taskPlanTaskRef(parent, chain.Lane))
		for _, item := range chain.Tasks {
			parentID := parent.ID
			child := taskgorm.WorkTask{
				WorkspaceID: run.WorkspaceID, TeamID: requirement.TeamID, ProjectSetID: requirement.ProjectSetID,
				ProjectID: requirement.PlanUnitID, RequirementID: requirement.ID,
				Kind: taskgorm.WorkTaskKindSubtask, ParentID: &parentID, Title: taskPlanTitle(item.Title),
				Description: taskPlanDescription(item), Status: "draft", Priority: priority, ReporterUserID: &reporterID,
				AssigneeID: assignee,
			}
			if err := s.createPreparedTask(tx, c, authActor, notificationContext, project, &child); err != nil {
				return nil, err
			}
			refs = append(refs, taskPlanTaskRef(child, chain.Lane))
		}
	}
	return refs, nil
}

func taskPlanTaskRef(task taskgorm.WorkTask, lane string) taskplan.CreatedTaskRef {
	return taskplan.CreatedTaskRef{
		ID: task.ID, TaskKey: task.TaskKey, Title: task.Title,
		Lane: lane, Kind: task.Kind, ParentID: task.ParentID,
	}
}

func taskPlanTitle(value string) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= 220 {
		return value
	}
	return string([]rune(value)[:220])
}

func taskPlanDescription(item taskplan.Item) string {
	criteria := make([]string, 0, len(item.AcceptanceCriteria))
	for _, criterion := range item.AcceptanceCriteria {
		criteria = append(criteria, "- "+strings.TrimSpace(criterion))
	}
	return strings.TrimSpace(item.Description) + "\n\n验收条件：\n" + strings.Join(criteria, "\n")
}

func respondTaskPlanApplyError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, errAITransition):
		fail(c, http.StatusConflict, "AI run changed before the task plan was applied")
	case errors.Is(err, errTaskPlanApplyStale), errors.Is(err, errProjectChildScopeChanged):
		fail(c, http.StatusConflict, "task plan subject changed; rerun before applying")
	case errors.Is(err, errTaskPlanApplyInvalid):
		fail(c, http.StatusConflict, "AI task plan output is invalid")
	case errors.Is(err, errTaskPlanWorkflowBound):
		fail(c, http.StatusConflict, "workflow task plans must be confirmed from their active workflow node")
	default:
		dbFail(c)
	}
	return true
}
