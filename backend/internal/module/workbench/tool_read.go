package workbench

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
)

var ErrToolReadInvalid = errors.New("workbench tool read query is invalid")

type TodoFilter struct {
	ProjectSetID    uint
	PlanUnitID      uint
	RequirementID   uint
	Query           string
	Domains         string
	Type            string
	IncludeMentions bool
	Limit           int
	Offset          int
	Now             time.Time
}

type TodoSummary struct {
	Total            int `json:"total"`
	Tasks            int `json:"tasks"`
	Bugs             int `json:"bugs"`
	Workflows        int `json:"workflows"`
	Overdue          int `json:"overdue"`
	DueToday         int `json:"due_today"`
	BlockedTasks     int `json:"blocked_tasks"`
	ReviewBugs       int `json:"review_bugs"`
	WorkflowAIFailed int `json:"workflow_ai_failed"`
	Mentions         int `json:"mentions"`
}

type TodoSummaryOutput struct {
	Summary     TodoSummary `json:"summary"`
	GeneratedAt time.Time   `json:"generated_at"`
}

type TodoFeedItem struct {
	ID                 uint       `json:"id"`
	SourceType         string     `json:"source_type"`
	Title              string     `json:"title"`
	Status             string     `json:"status"`
	Priority           string     `json:"priority,omitempty"`
	DueAt              *time.Time `json:"due_at,omitempty"`
	ProjectSetID       uint       `json:"project_set_id,omitempty"`
	PlanUnitID         uint       `json:"project_new_id,omitempty"`
	RequirementID      uint       `json:"requirement_id,omitempty"`
	AssigneeUserID     *uint      `json:"assignee_user_id,omitempty"`
	MentionAt          *time.Time `json:"mention_at,omitempty"`
	WorkflowInstanceID uint       `json:"workflow_instance_id,omitempty"`
	WorkflowNodeID     string     `json:"workflow_node_id,omitempty"`
	ActionKind         string     `json:"action_kind,omitempty"`
	AllowedActions     []string   `json:"allowed_actions,omitempty"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type TodoFeedOutput struct {
	Items []TodoFeedItem `json:"items"`
	Total int64          `json:"total"`
}

type TodoTaskQueries interface {
	ListPrincipalTodos(context.Context, uint, uint, task.TodoFilter) (task.TodoPage, error)
	ListPrincipalMentions(context.Context, uint, uint, string, task.TodoFilter) (task.TodoPage, error)
}

type TodoBugQueries interface {
	ListPrincipalBugTodos(context.Context, uint, uint, quality.BugTodoFilter) (quality.BugReadPage, error)
}

type TodoWorkflowQueries interface {
	ListPrincipalTodos(context.Context, uint, uint) ([]workflow.TodoSnapshot, error)
}

type TodoAIQueries interface {
	ListRunStatuses(context.Context, uint, []uint) ([]ai.RunStatus, error)
}

type TodoIdentityQueries interface {
	SearchWorkspaceUsers(context.Context, uint, identity.ToolUserFilter) (identity.ToolUserPage, error)
}

type TodoProjectQueries interface {
	GetPlanUnit(context.Context, uint, uint) (project.PlanUnitSnapshot, error)
}

type TodoRequirementQueries interface {
	GetRequirement(context.Context, uint, uint) (requirement.Snapshot, error)
}

type ToolReadDependencies struct {
	Tasks        TodoTaskQueries
	Bugs         TodoBugQueries
	Workflows    TodoWorkflowQueries
	AI           TodoAIQueries
	Identities   TodoIdentityQueries
	Projects     TodoProjectQueries
	Requirements TodoRequirementQueries
}

type ToolReadService struct{ dependencies ToolReadDependencies }

func NewToolReadService(dependencies ToolReadDependencies) (*ToolReadService, error) {
	if dependencies.Tasks == nil || dependencies.Bugs == nil || dependencies.Workflows == nil ||
		dependencies.AI == nil || dependencies.Identities == nil || dependencies.Projects == nil || dependencies.Requirements == nil {
		return nil, ErrToolReadInvalid
	}
	return &ToolReadService{dependencies: dependencies}, nil
}

func (service *ToolReadService) TodoSummary(
	ctx context.Context,
	workspaceID uint,
	principalID uint,
	filter TodoFilter,
) (TodoSummaryOutput, error) {
	filter, err := prepareWorkbenchFilter(workspaceID, principalID, filter)
	if err != nil {
		return TodoSummaryOutput{}, err
	}
	wantTasks, wantBugs, wantWorkflows, err := todoDomains(filter.Domains)
	if err != nil {
		return TodoSummaryOutput{}, err
	}
	summary := TodoSummary{}
	if wantTasks {
		items, loadErr := service.allTaskTodos(ctx, workspaceID, principalID, filter)
		if loadErr != nil {
			return TodoSummaryOutput{}, loadErr
		}
		summary.Tasks = len(items)
		for _, item := range items {
			accumulateTaskSummary(&summary, item, filter.Now)
		}
	}
	if wantBugs {
		items, loadErr := service.allBugTodos(ctx, workspaceID, principalID, filter)
		if loadErr != nil {
			return TodoSummaryOutput{}, loadErr
		}
		summary.Bugs = len(items)
		for _, item := range items {
			accumulateBugSummary(&summary, item, filter.Now)
		}
	}
	if wantWorkflows {
		items, loadErr := service.workflowTodos(ctx, workspaceID, principalID, filter)
		if loadErr != nil {
			return TodoSummaryOutput{}, loadErr
		}
		summary.Workflows = len(items)
		for _, item := range items {
			accumulateWorkflowSummary(&summary, item, filter.Now)
		}
	}
	if filter.IncludeMentions {
		profile, loadErr := service.principalProfile(ctx, workspaceID, principalID)
		if loadErr != nil {
			return TodoSummaryOutput{}, loadErr
		}
		page, loadErr := service.dependencies.Tasks.ListPrincipalMentions(
			ctx, workspaceID, principalID, profile.Username, taskTodoFilter(filter, 1, 0),
		)
		if loadErr != nil {
			return TodoSummaryOutput{}, loadErr
		}
		summary.Mentions = int(page.Total)
	}
	summary.Total = summary.Tasks + summary.Bugs + summary.Workflows
	return TodoSummaryOutput{Summary: summary, GeneratedAt: filter.Now}, nil
}

func (service *ToolReadService) TodoFeed(
	ctx context.Context,
	workspaceID uint,
	principalID uint,
	filter TodoFilter,
) (TodoFeedOutput, error) {
	filter, err := prepareWorkbenchFilter(workspaceID, principalID, filter)
	if err != nil {
		return TodoFeedOutput{}, err
	}
	switch filter.Type {
	case "task":
		page, err := service.dependencies.Tasks.ListPrincipalTodos(ctx, workspaceID, principalID, taskTodoFilter(filter, filter.Limit, filter.Offset))
		return taskFeed(page), err
	case "bug":
		page, err := service.dependencies.Bugs.ListPrincipalBugTodos(ctx, workspaceID, principalID, bugTodoFilter(filter, filter.Limit, filter.Offset))
		return bugFeed(page), err
	case "workflow":
		items, err := service.workflowTodos(ctx, workspaceID, principalID, filter)
		if err != nil {
			return TodoFeedOutput{}, err
		}
		return pagedWorkflowFeed(items, filter.Limit, filter.Offset), nil
	case "mention":
		profile, err := service.principalProfile(ctx, workspaceID, principalID)
		if err != nil {
			return TodoFeedOutput{}, err
		}
		page, err := service.dependencies.Tasks.ListPrincipalMentions(ctx, workspaceID, principalID, profile.Username, taskTodoFilter(filter, filter.Limit, filter.Offset))
		return mentionFeed(page), err
	default:
		return TodoFeedOutput{}, ErrToolReadInvalid
	}
}

func prepareWorkbenchFilter(workspaceID, principalID uint, filter TodoFilter) (TodoFilter, error) {
	if workspaceID == 0 || principalID == 0 || filter.Offset < 0 {
		return TodoFilter{}, ErrToolReadInvalid
	}
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Domains = strings.ToLower(strings.TrimSpace(filter.Domains))
	filter.Type = strings.ToLower(strings.TrimSpace(filter.Type))
	if filter.Type == "" {
		filter.Type = "task"
	}
	if filter.Limit < 1 {
		filter.Limit = 20
	} else if filter.Limit > 500 {
		filter.Limit = 500
	}
	if filter.Now.IsZero() {
		filter.Now = time.Now().UTC()
	} else {
		filter.Now = filter.Now.UTC()
	}
	return filter, nil
}

func todoDomains(raw string) (bool, bool, bool, error) {
	if strings.TrimSpace(raw) == "" {
		return true, true, true, nil
	}
	wantTasks, wantBugs, wantWorkflows := false, false, false
	for _, part := range strings.Split(raw, ",") {
		switch strings.TrimSpace(part) {
		case "task", "tasks":
			wantTasks = true
		case "bug", "bugs", "quality":
			wantBugs = true
		case "workflow", "workflows", "workflow_node", "workflow_nodes":
			wantWorkflows = true
		default:
			return false, false, false, fmt.Errorf("%w: unsupported todo domain", ErrToolReadInvalid)
		}
	}
	return wantTasks, wantBugs, wantWorkflows, nil
}

func (service *ToolReadService) allTaskTodos(ctx context.Context, workspaceID, principalID uint, filter TodoFilter) ([]task.TodoItem, error) {
	items := []task.TodoItem{}
	for offset := 0; ; offset += 500 {
		page, err := service.dependencies.Tasks.ListPrincipalTodos(ctx, workspaceID, principalID, taskTodoFilter(filter, 500, offset))
		if err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
		if int64(len(items)) >= page.Total || len(page.Items) == 0 {
			return items, nil
		}
	}
}

func (service *ToolReadService) allBugTodos(ctx context.Context, workspaceID, principalID uint, filter TodoFilter) ([]quality.BugSnapshot, error) {
	items := []quality.BugSnapshot{}
	for offset := 0; ; offset += 500 {
		page, err := service.dependencies.Bugs.ListPrincipalBugTodos(ctx, workspaceID, principalID, bugTodoFilter(filter, 500, offset))
		if err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
		if int64(len(items)) >= page.Total || len(page.Items) == 0 {
			return items, nil
		}
	}
}

func (service *ToolReadService) workflowTodos(ctx context.Context, workspaceID, principalID uint, filter TodoFilter) ([]workflowTodo, error) {
	items, err := service.dependencies.Workflows.ListPrincipalTodos(ctx, workspaceID, principalID)
	if err != nil {
		return nil, err
	}
	runIDs := make([]uint, 0)
	for _, item := range items {
		if item.AIRunID > 0 {
			runIDs = append(runIDs, item.AIRunID)
		}
	}
	statuses, err := service.dependencies.AI.ListRunStatuses(ctx, workspaceID, runIDs)
	if err != nil {
		return nil, err
	}
	statusByID := make(map[uint]string, len(statuses))
	for _, status := range statuses {
		statusByID[status.ID] = status.Status
	}
	result := make([]workflowTodo, 0, len(items))
	for _, item := range items {
		resolved, ok, err := service.resolveWorkflowTodo(ctx, workspaceID, item, filter, statusByID[item.AIRunID])
		if err != nil {
			return nil, err
		}
		if ok {
			result = append(result, resolved)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		leftOverdue := result[i].DueAt != nil && result[i].DueAt.Before(filter.Now)
		rightOverdue := result[j].DueAt != nil && result[j].DueAt.Before(filter.Now)
		if leftOverdue != rightOverdue {
			return leftOverdue
		}
		priority := map[string]int{"ai_retry": 0, "approval": 1, "ai_confirm": 2, "condition": 3, "complete": 4}
		if priority[result[i].ActionKind] != priority[result[j].ActionKind] {
			return priority[result[i].ActionKind] < priority[result[j].ActionKind]
		}
		if (result[i].DueAt == nil) != (result[j].DueAt == nil) {
			return result[i].DueAt != nil
		}
		if result[i].DueAt != nil && !result[i].DueAt.Equal(*result[j].DueAt) {
			return result[i].DueAt.Before(*result[j].DueAt)
		}
		if !result[i].UpdatedAt.Equal(result[j].UpdatedAt) {
			return result[i].UpdatedAt.After(result[j].UpdatedAt)
		}
		return result[i].ID > result[j].ID
	})
	return result, nil
}

type workflowTodo struct {
	ID             uint
	Title          string
	SubjectType    string
	SubjectID      uint
	ProjectSetID   uint
	PlanUnitID     uint
	RequirementID  uint
	DueAt          *time.Time
	AssigneeUserID uint
	InstanceID     uint
	NodeID         string
	ActionKind     string
	AllowedActions []string
	UpdatedAt      time.Time
}

func (service *ToolReadService) resolveWorkflowTodo(
	ctx context.Context,
	workspaceID uint,
	item workflow.TodoSnapshot,
	filter TodoFilter,
	aiStatus string,
) (workflowTodo, bool, error) {
	result := workflowTodo{
		ID: item.TokenID, Title: item.NodeName, SubjectType: item.SubjectType, SubjectID: item.SubjectID,
		DueAt: item.DueAt, AssigneeUserID: item.AssigneeUserID, InstanceID: item.InstanceID,
		NodeID: item.NodeID, ActionKind: item.ActionKind, AllowedActions: item.AllowedActions, UpdatedAt: item.UpdatedAt,
	}
	subjectTitle := ""
	switch item.SubjectType {
	case "plan_unit":
		unit, err := service.dependencies.Projects.GetPlanUnit(ctx, workspaceID, item.SubjectID)
		if err != nil {
			return workflowTodo{}, false, err
		}
		result.ProjectSetID, result.PlanUnitID, subjectTitle = unit.ProjectSetID, unit.ID, unit.Title
	case "requirement":
		requirementItem, err := service.dependencies.Requirements.GetRequirement(ctx, workspaceID, item.SubjectID)
		if err != nil {
			return workflowTodo{}, false, err
		}
		result.ProjectSetID, result.RequirementID, subjectTitle = requirementItem.ProjectSetID, requirementItem.ID, requirementItem.Title
		if requirementItem.PlanUnitID != nil {
			result.PlanUnitID = *requirementItem.PlanUnitID
		}
	default:
		return workflowTodo{}, false, nil
	}
	if filter.ProjectSetID > 0 && result.ProjectSetID != filter.ProjectSetID ||
		filter.PlanUnitID > 0 && result.PlanUnitID != filter.PlanUnitID ||
		filter.RequirementID > 0 && result.RequirementID != filter.RequirementID {
		return workflowTodo{}, false, nil
	}
	if query := strings.ToLower(filter.Query); query != "" && !strings.Contains(strings.ToLower(strings.Join(
		[]string{item.NodeID, item.NodeName, item.TemplateName, subjectTitle}, " ",
	)), query) {
		return workflowTodo{}, false, nil
	}
	if strings.HasPrefix(item.NodeType, "ai_") {
		switch aiStatus {
		case "failed", "cancelled", "canceled":
			result.ActionKind, result.AllowedActions = "ai_retry", []string{"retry_ai"}
		case "completed", "applied":
			result.ActionKind, result.AllowedActions = "ai_confirm", []string{"complete", "recheck_ai"}
			if item.Capability == "requirement.completeness" {
				result.AllowedActions = append(result.AllowedActions, "override_ai")
			}
		default:
			return workflowTodo{}, false, nil
		}
	}
	return result, true, nil
}

func (service *ToolReadService) principalProfile(ctx context.Context, workspaceID, principalID uint) (identity.ToolUser, error) {
	page, err := service.dependencies.Identities.SearchWorkspaceUsers(ctx, workspaceID, identity.ToolUserFilter{ID: principalID, Limit: 1})
	if err != nil {
		return identity.ToolUser{}, err
	}
	if len(page.Items) != 1 {
		return identity.ToolUser{}, ErrToolReadInvalid
	}
	return page.Items[0], nil
}

func taskTodoFilter(filter TodoFilter, limit, offset int) task.TodoFilter {
	return task.TodoFilter{ProjectSetID: filter.ProjectSetID, PlanUnitID: filter.PlanUnitID, RequirementID: filter.RequirementID, Query: filter.Query, Limit: limit, Offset: offset, Now: filter.Now}
}

func bugTodoFilter(filter TodoFilter, limit, offset int) quality.BugTodoFilter {
	return quality.BugTodoFilter{ProjectSetID: filter.ProjectSetID, PlanUnitID: filter.PlanUnitID, RequirementID: filter.RequirementID, Query: filter.Query, Limit: limit, Offset: offset, Now: filter.Now}
}

func accumulateTaskSummary(summary *TodoSummary, item task.TodoItem, now time.Time) {
	if item.DueAt != nil && item.DueAt.Before(now) {
		summary.Overdue++
	}
	if dueToday(item.DueAt, now) {
		summary.DueToday++
	}
	status := strings.ToLower(strings.TrimSpace(item.Status))
	if status == "blocked" || status == "block" {
		summary.BlockedTasks++
	}
}

func accumulateBugSummary(summary *TodoSummary, item quality.BugSnapshot, now time.Time) {
	if item.DueAt != nil && item.DueAt.Before(now) {
		summary.Overdue++
	}
	if dueToday(item.DueAt, now) {
		summary.DueToday++
	}
	if strings.ToLower(strings.TrimSpace(item.Status)) == "review" {
		summary.ReviewBugs++
	}
}

func accumulateWorkflowSummary(summary *TodoSummary, item workflowTodo, now time.Time) {
	if item.DueAt != nil && item.DueAt.Before(now) {
		summary.Overdue++
	}
	if dueToday(item.DueAt, now) {
		summary.DueToday++
	}
	if item.ActionKind == "ai_retry" {
		summary.WorkflowAIFailed++
	}
}

func dueToday(value *time.Time, now time.Time) bool {
	if value == nil {
		return false
	}
	start := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC)
	return !value.Before(start) && value.Before(start.AddDate(0, 0, 1))
}

func taskFeed(page task.TodoPage) TodoFeedOutput    { return taskLikeFeed(page, "task") }
func mentionFeed(page task.TodoPage) TodoFeedOutput { return taskLikeFeed(page, "mention") }

func taskLikeFeed(page task.TodoPage, sourceType string) TodoFeedOutput {
	items := make([]TodoFeedItem, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, TodoFeedItem{
			ID: item.ID, SourceType: sourceType, Title: item.Title, Status: item.Status, Priority: item.Priority,
			DueAt: item.DueAt, ProjectSetID: item.ProjectSetID, PlanUnitID: item.PlanUnitID,
			RequirementID: item.RequirementID, AssigneeUserID: item.AssigneeID, MentionAt: item.MentionAt, UpdatedAt: item.UpdatedAt,
		})
	}
	return TodoFeedOutput{Items: items, Total: page.Total}
}

func bugFeed(page quality.BugReadPage) TodoFeedOutput {
	items := make([]TodoFeedItem, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, TodoFeedItem{
			ID: item.ID, SourceType: "bug", Title: item.Title, Status: item.Status, Priority: item.Priority,
			DueAt: item.DueAt, ProjectSetID: item.ProjectSetID, PlanUnitID: item.PlanUnitID,
			RequirementID: item.RequirementID, AssigneeUserID: item.AssigneeUserID, UpdatedAt: item.UpdatedAt,
		})
	}
	return TodoFeedOutput{Items: items, Total: page.Total}
}

func pagedWorkflowFeed(items []workflowTodo, limit, offset int) TodoFeedOutput {
	total := len(items)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	rows := make([]TodoFeedItem, 0, end-start)
	for _, item := range items[start:end] {
		assignee := item.AssigneeUserID
		rows = append(rows, TodoFeedItem{
			ID: item.ID, SourceType: "workflow", Title: item.Title, Status: "active", DueAt: item.DueAt,
			ProjectSetID: item.ProjectSetID, PlanUnitID: item.PlanUnitID, RequirementID: item.RequirementID,
			AssigneeUserID: &assignee, WorkflowInstanceID: item.InstanceID, WorkflowNodeID: item.NodeID,
			ActionKind: item.ActionKind, AllowedActions: item.AllowedActions, UpdatedAt: item.UpdatedAt,
		})
	}
	return TodoFeedOutput{Items: rows, Total: int64(total)}
}
