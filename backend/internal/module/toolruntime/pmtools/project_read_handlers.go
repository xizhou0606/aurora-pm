package pmtools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/audit"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const (
	ProjectBoardGet          = "project.board.get"
	ProjectScopeOptionsList  = "project.scope_options.list"
	ProjectSetAuditList      = "project.set.audit.list"
	ProjectDeliveryLabelList = "project.delivery_label.list"
	ProjectRequirementList   = "project.requirement.list"
	ProjectParticipantList   = "project.participant.list"
)

var projectReadHandlerNames = []string{
	ProjectBoardGet,
	ProjectScopeOptionsList,
	ProjectSetAuditList,
	ProjectDeliveryLabelList,
	ProjectRequirementList,
	ProjectParticipantList,
}

type ProjectReadProjectQueries interface {
	GetProjectSet(context.Context, uint, uint) (project.ProjectSetSnapshot, error)
	GetPlanUnit(context.Context, uint, uint) (project.PlanUnitSnapshot, error)
	ListPlanUnits(context.Context, uint, uint) ([]project.PlanUnitSnapshot, error)
	ListDeliveryLabels(context.Context, uint, uint) ([]project.DeliveryLabelSnapshot, error)
	ListScopeOptions(context.Context, uint, uint) (project.ScopeOptions, error)
}

type ProjectReadRequirementQueries interface {
	ListRequirements(context.Context, uint, requirement.ListFilter) (requirement.ListPage, error)
	ListScopeRequirements(context.Context, uint, []uint) ([]requirement.Snapshot, error)
}

type ProjectReadTaskQueries interface {
	ListTasks(context.Context, uint, task.ListFilter) (task.ListResult, error)
}

type ProjectReadQualityQueries interface {
	ListBugs(context.Context, uint, quality.BugReadFilter) (quality.BugReadPage, error)
}

type ProjectReadAuditQueries interface {
	ListProjectEvents(context.Context, uint, uint, int, int) (audit.ProjectEventPage, error)
}

type ProjectReadIdentityQueries interface {
	ListActiveUsers(context.Context) ([]identity.UserSummary, error)
}

type ProjectReadDependencies struct {
	Projects     ProjectReadProjectQueries
	Requirements ProjectReadRequirementQueries
	Tasks        ProjectReadTaskQueries
	Quality      ProjectReadQualityQueries
	Audits       ProjectReadAuditQueries
	Identities   ProjectReadIdentityQueries
}

func ProjectReadHandlerNames() []string { return append([]string(nil), projectReadHandlerNames...) }

func NewProjectReadHandlers(dependencies ProjectReadDependencies) (map[string]toolruntime.ToolHandler, error) {
	if dependencies.Projects == nil || dependencies.Requirements == nil || dependencies.Tasks == nil ||
		dependencies.Quality == nil || dependencies.Audits == nil || dependencies.Identities == nil {
		return nil, ErrReadHandlersInvalid
	}
	return map[string]toolruntime.ToolHandler{
		ProjectBoardGet: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			var input projectBoardInput
			if err := decodeInput(call.Input, &input); err != nil || input.ProjectSetID == 0 {
				return nil, projectReadInputError("project_new_id is required")
			}
			return loadProjectBoard(ctx, call.Authorization.WorkspaceID, input, dependencies)
		}),
		ProjectScopeOptionsList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			if err := decodeInput(call.Input, &struct{}{}); err != nil {
				return nil, err
			}
			return loadProjectScopeOptions(ctx, call.Authorization.WorkspaceID, call.Authorization.PrincipalID, dependencies)
		}),
		ProjectSetAuditList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			var input projectPageInput
			if err := decodeInput(call.Input, &input); err != nil || input.ID == 0 || input.Offset < 0 {
				return nil, projectReadInputError("id and valid pagination are required")
			}
			if _, err := dependencies.Projects.GetProjectSet(ctx, call.Authorization.WorkspaceID, input.ID); err != nil {
				return nil, err
			}
			return dependencies.Audits.ListProjectEvents(ctx, call.Authorization.WorkspaceID, input.ID, input.Limit, input.Offset)
		}),
		ProjectDeliveryLabelList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := inputID(call.Input)
			if err != nil {
				return nil, err
			}
			items, err := dependencies.Projects.ListDeliveryLabels(ctx, call.Authorization.WorkspaceID, input.ID)
			return makeListOutput(items), err
		}),
		ProjectRequirementList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			var input projectRequirementListInput
			if err := decodeInput(call.Input, &input); err != nil || input.Offset < 0 {
				return nil, projectReadInputError("invalid requirement filter")
			}
			if err := validateRequirementScope(ctx, call.Authorization.WorkspaceID, input, dependencies); err != nil {
				return nil, err
			}
			return dependencies.Requirements.ListRequirements(ctx, call.Authorization.WorkspaceID, requirement.ListFilter{
				ProjectSetID: input.ProjectSetID, PlanUnitID: input.PlanUnitID, Query: input.Query,
				Limit: input.Limit, Offset: input.Offset,
			})
		}),
		ProjectParticipantList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := inputID(call.Input)
			if err != nil {
				return nil, err
			}
			return loadProjectParticipants(ctx, call.Authorization.WorkspaceID, input.ID, dependencies)
		}),
	}, nil
}

type projectBoardInput struct {
	ProjectSetID uint   `json:"project_new_id"`
	Include      string `json:"include"`
}

type projectPageInput struct {
	ID     uint `json:"id"`
	Limit  int  `json:"limit"`
	Offset int  `json:"offset"`
}

type projectRequirementListInput struct {
	ProjectSetID uint   `json:"project_new_id"`
	PlanUnitID   uint   `json:"version_task_new_id"`
	Query        string `json:"q"`
	Limit        int    `json:"limit"`
	Offset       int    `json:"offset"`
}

type projectBoardOutput struct {
	Project      project.ProjectSetSnapshot `json:"project"`
	PlanUnits    []project.PlanUnitSnapshot `json:"version_tasks,omitempty"`
	Requirements []requirement.Snapshot     `json:"requirements,omitempty"`
	Tasks        []task.ListItem            `json:"tasks,omitempty"`
}

type projectScopeOptionsOutput struct {
	ProjectSets  []project.ScopeProjectSet  `json:"project_sets"`
	Projects     []project.PlanUnitSnapshot `json:"projects"`
	Requirements []requirement.Snapshot     `json:"requirements"`
}

func loadProjectBoard(
	ctx context.Context,
	workspaceID uint,
	input projectBoardInput,
	dependencies ProjectReadDependencies,
) (projectBoardOutput, error) {
	included, err := projectBoardIncludes(input.Include)
	if err != nil {
		return projectBoardOutput{}, err
	}
	projectSet, err := dependencies.Projects.GetProjectSet(ctx, workspaceID, input.ProjectSetID)
	if err != nil {
		return projectBoardOutput{}, err
	}
	output := projectBoardOutput{Project: projectSet}
	if included("version_tasks") {
		output.PlanUnits, err = dependencies.Projects.ListPlanUnits(ctx, workspaceID, input.ProjectSetID)
		if err != nil {
			return projectBoardOutput{}, err
		}
	}
	if included("requirements") {
		output.Requirements, err = allProjectRequirements(ctx, workspaceID, input.ProjectSetID, dependencies.Requirements)
		if err != nil {
			return projectBoardOutput{}, err
		}
	}
	if included("tasks") {
		output.Tasks, err = allProjectTasks(ctx, workspaceID, input.ProjectSetID, dependencies.Tasks)
		if err != nil {
			return projectBoardOutput{}, err
		}
	}
	return output, nil
}

func projectBoardIncludes(raw string) (func(string) bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return func(string) bool { return true }, nil
	}
	allowed := map[string]struct{}{"version_tasks": {}, "requirements": {}, "tasks": {}}
	selected := make(map[string]struct{})
	for _, part := range strings.Split(value, ",") {
		name := strings.TrimSpace(part)
		if _, ok := allowed[name]; !ok {
			return nil, projectReadInputError("include contains an unsupported resource")
		}
		selected[name] = struct{}{}
	}
	return func(name string) bool { _, ok := selected[name]; return ok }, nil
}

// loadProjectScopeOptions enumerates the scope options the calling principal
// may select: project data access is enforced by the project domain, so a
// caller without access to a project set never receives it as an option.
func loadProjectScopeOptions(
	ctx context.Context,
	workspaceID, userID uint,
	dependencies ProjectReadDependencies,
) (projectScopeOptionsOutput, error) {
	scope, err := dependencies.Projects.ListScopeOptions(ctx, workspaceID, userID)
	if err != nil {
		return projectScopeOptionsOutput{}, err
	}
	projectIDs := make([]uint, 0, len(scope.ProjectSets))
	for _, item := range scope.ProjectSets {
		projectIDs = append(projectIDs, item.ID)
	}
	requirements, err := dependencies.Requirements.ListScopeRequirements(ctx, workspaceID, projectIDs)
	if err != nil {
		return projectScopeOptionsOutput{}, err
	}
	validUnits := make(map[uint]project.PlanUnitSnapshot, len(scope.PlanUnits))
	for _, unit := range scope.PlanUnits {
		validUnits[unit.ID] = unit
	}
	filtered := make([]requirement.Snapshot, 0, len(requirements))
	for _, item := range requirements {
		if item.PlanUnitID == nil {
			continue
		}
		unit, ok := validUnits[*item.PlanUnitID]
		if ok && unit.ProjectSetID == item.ProjectSetID && unit.TeamID == item.TeamID {
			filtered = append(filtered, item)
		}
	}
	return projectScopeOptionsOutput{ProjectSets: scope.ProjectSets, Projects: scope.PlanUnits, Requirements: filtered}, nil
}

func validateRequirementScope(
	ctx context.Context,
	workspaceID uint,
	input projectRequirementListInput,
	dependencies ProjectReadDependencies,
) error {
	if input.ProjectSetID == 0 && input.PlanUnitID == 0 && strings.TrimSpace(input.Query) == "" {
		return projectReadInputError("project_new_id, version_task_new_id or q is required")
	}
	if input.PlanUnitID > 0 {
		unit, err := dependencies.Projects.GetPlanUnit(ctx, workspaceID, input.PlanUnitID)
		if err != nil {
			return err
		}
		if input.ProjectSetID > 0 && unit.ProjectSetID != input.ProjectSetID {
			return projectReadInputError("plan unit does not belong to project")
		}
		if input.ProjectSetID == 0 {
			input.ProjectSetID = unit.ProjectSetID
		}
	}
	if input.ProjectSetID > 0 {
		_, err := dependencies.Projects.GetProjectSet(ctx, workspaceID, input.ProjectSetID)
		return err
	}
	return nil
}

func allProjectRequirements(
	ctx context.Context,
	workspaceID uint,
	projectSetID uint,
	queries ProjectReadRequirementQueries,
) ([]requirement.Snapshot, error) {
	items := []requirement.Snapshot{}
	for offset := 0; ; offset += requirement.MaximumListLimit {
		page, err := queries.ListRequirements(ctx, workspaceID, requirement.ListFilter{
			ProjectSetID: projectSetID, Limit: requirement.MaximumListLimit, Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
		if int64(len(items)) >= page.Total || len(page.Items) == 0 {
			return items, nil
		}
	}
}

func allProjectTasks(
	ctx context.Context,
	workspaceID uint,
	projectSetID uint,
	queries ProjectReadTaskQueries,
) ([]task.ListItem, error) {
	items := []task.ListItem{}
	for offset := 0; ; offset += 500 {
		page, err := queries.ListTasks(ctx, workspaceID, task.ListFilter{ProjectSetID: projectSetID, Limit: 500, Offset: offset})
		if err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
		if int64(len(items)) >= page.Total || len(page.Items) == 0 {
			return items, nil
		}
	}
}

type projectParticipantSource struct {
	ResourceType  string `json:"resource_type"`
	ResourceID    uint   `json:"resource_id"`
	ResourceKey   string `json:"resource_key"`
	ResourceTitle string `json:"resource_title"`
	Relation      string `json:"relation"`
}

type projectParticipant struct {
	UserID            uint                       `json:"user_id"`
	Username          string                     `json:"username"`
	DisplayName       string                     `json:"display_name"`
	IdentityAvailable bool                       `json:"identity_available"`
	Sources           []projectParticipantSource `json:"sources"`
}

type projectParticipantOutput struct {
	ProjectSet struct {
		ID   uint   `json:"id"`
		Code string `json:"code,omitempty"`
		Name string `json:"name"`
	} `json:"project_set"`
	Participants  []projectParticipant `json:"participants"`
	Total         int                  `json:"total"`
	Authorization struct {
		InformationalOnly bool   `json:"informational_only"`
		GrantsAccess      bool   `json:"grants_access"`
		Message           string `json:"message"`
	} `json:"authorization"`
}

func loadProjectParticipants(
	ctx context.Context,
	workspaceID uint,
	projectSetID uint,
	dependencies ProjectReadDependencies,
) (projectParticipantOutput, error) {
	projectSet, err := dependencies.Projects.GetProjectSet(ctx, workspaceID, projectSetID)
	if err != nil {
		return projectParticipantOutput{}, err
	}
	units, err := dependencies.Projects.ListPlanUnits(ctx, workspaceID, projectSetID)
	if err != nil {
		return projectParticipantOutput{}, err
	}
	requirements, err := allProjectRequirements(ctx, workspaceID, projectSetID, dependencies.Requirements)
	if err != nil {
		return projectParticipantOutput{}, err
	}
	tasks, err := allProjectTasks(ctx, workspaceID, projectSetID, dependencies.Tasks)
	if err != nil {
		return projectParticipantOutput{}, err
	}
	bugs, err := allProjectBugs(ctx, workspaceID, projectSetID, dependencies.Quality)
	if err != nil {
		return projectParticipantOutput{}, err
	}
	users, err := dependencies.Identities.ListActiveUsers(ctx)
	if err != nil {
		return projectParticipantOutput{}, err
	}
	profiles := make(map[uint]identity.UserSummary, len(users))
	for _, user := range users {
		profiles[user.ID] = user
	}
	sourcesByUser := make(map[uint][]projectParticipantSource)
	seen := make(map[uint]map[string]struct{})
	add := func(userID uint, source projectParticipantSource) {
		if userID == 0 {
			return
		}
		if seen[userID] == nil {
			seen[userID] = make(map[string]struct{})
		}
		key := fmt.Sprintf("%s:%d:%s", source.ResourceType, source.ResourceID, source.Relation)
		if _, duplicate := seen[userID][key]; duplicate {
			return
		}
		seen[userID][key] = struct{}{}
		sourcesByUser[userID] = append(sourcesByUser[userID], source)
	}
	if projectSet.OwnerUserID != nil {
		add(*projectSet.OwnerUserID, participantSource("project_set", projectSet.ID, firstProjectValue(projectSet.Code, projectSet.Name), projectSet.Name, "owner"))
	}
	for _, unit := range units {
		if unit.OwnerUserID != nil {
			add(*unit.OwnerUserID, participantSource("project_version", unit.ID, unit.Title, unit.Title, "owner"))
		}
	}
	for _, item := range requirements {
		for _, userID := range item.ProductOwnerUserIDs {
			add(userID, participantSource("requirement", item.ID, item.Title, item.Title, "product_owner"))
		}
		for _, userID := range item.DesignOwnerUserIDs {
			add(userID, participantSource("requirement", item.ID, item.Title, item.Title, "design_owner"))
		}
		for _, userID := range item.DeveloperUserIDs {
			add(userID, participantSource("requirement", item.ID, item.Title, item.Title, "developer"))
		}
	}
	for _, item := range tasks {
		key := firstProjectValue(item.TaskKey, item.Title)
		if item.AssigneeID != nil {
			add(*item.AssigneeID, participantSource("task", item.ID, key, item.Title, "assignee"))
		}
		if item.ReporterUserID != nil {
			add(*item.ReporterUserID, participantSource("task", item.ID, key, item.Title, "reporter"))
		}
	}
	for _, item := range bugs {
		key := firstProjectValue(item.BugKey, item.Title)
		if item.AssigneeUserID != nil {
			add(*item.AssigneeUserID, participantSource("quality_bug", item.ID, key, item.Title, "assignee"))
		}
		if item.ReporterUserID != nil {
			add(*item.ReporterUserID, participantSource("quality_bug", item.ID, key, item.Title, "reporter"))
		}
	}
	userIDs := make([]uint, 0, len(sourcesByUser))
	for userID := range sourcesByUser {
		userIDs = append(userIDs, userID)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	participants := make([]projectParticipant, 0, len(userIDs))
	for _, userID := range userIDs {
		sources := sourcesByUser[userID]
		sort.SliceStable(sources, func(i, j int) bool {
			left := sources[i].ResourceType + "\x00" + sources[i].ResourceKey + "\x00" + sources[i].Relation
			right := sources[j].ResourceType + "\x00" + sources[j].ResourceKey + "\x00" + sources[j].Relation
			return left < right
		})
		profile, exists := profiles[userID]
		participants = append(participants, projectParticipant{
			UserID: userID, Username: profile.Username, DisplayName: profile.DisplayName,
			IdentityAvailable: exists, Sources: sources,
		})
	}
	output := projectParticipantOutput{Participants: participants, Total: len(participants)}
	output.ProjectSet.ID, output.ProjectSet.Code, output.ProjectSet.Name = projectSet.ID, projectSet.Code, projectSet.Name
	output.Authorization.InformationalOnly = true
	output.Authorization.GrantsAccess = false
	output.Authorization.Message = "participation sources do not grant authorization"
	return output, nil
}

func allProjectBugs(
	ctx context.Context,
	workspaceID uint,
	projectSetID uint,
	queries ProjectReadQualityQueries,
) ([]quality.BugSnapshot, error) {
	items := []quality.BugSnapshot{}
	for offset := 0; ; offset += 500 {
		page, err := queries.ListBugs(ctx, workspaceID, quality.BugReadFilter{ProjectSetID: projectSetID, Limit: 500, Offset: offset})
		if err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
		if int64(len(items)) >= page.Total || len(page.Items) == 0 {
			return items, nil
		}
	}
}

func participantSource(resourceType string, resourceID uint, resourceKey, title, relation string) projectParticipantSource {
	return projectParticipantSource{
		ResourceType: resourceType, ResourceID: resourceID, ResourceKey: strings.TrimSpace(resourceKey),
		ResourceTitle: strings.TrimSpace(title), Relation: relation,
	}
}

func firstProjectValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func projectReadInputError(message string) error {
	return fmt.Errorf("%w: %s", ErrReadHandlersInvalid, message)
}
