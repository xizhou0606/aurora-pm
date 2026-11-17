package httpapi

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

type workflowTodoSubject struct {
	ProjectSetID    uint
	ProjectID       uint
	RequirementID   uint
	ProjectSetName  string
	ProjectName     string
	RequirementName string
	Title           string
}

type workflowTodoItem struct {
	Token          workflowgorm.WorkflowToken
	Instance       workflowgorm.WorkflowInstance
	Node           workflowNode
	Subject        workflowTodoSubject
	TemplateName   string
	ActionKind     string
	AllowedActions []string
	AIStatus       string
	AIRunID        uint
	AssigneeUserID uint
	DueAt          *time.Time
}

func (s *Server) workbenchWorkflowTodos(
	c *gin.Context,
	scope workspaceScope,
) ([]workflowTodoItem, error) {
	workspace, _ := currentWorkspace(c)
	actor, _ := currentActor(c)
	userID := factUserID(actor.User)
	var tokens []workflowgorm.WorkflowToken
	if err := s.db.Where(
		"workspace_id = ? AND status = ? AND (assignee_user_id = ? OR assignee_user_id IS NULL)",
		workspace.ID, "active", userID,
	).Order("updated_at DESC, id DESC").Find(&tokens).Error; err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return []workflowTodoItem{}, nil
	}

	instanceIDs := make([]uint, 0, len(tokens))
	seenInstances := map[uint]bool{}
	for _, token := range tokens {
		if !seenInstances[token.InstanceID] {
			seenInstances[token.InstanceID] = true
			instanceIDs = append(instanceIDs, token.InstanceID)
		}
	}
	var instances []workflowgorm.WorkflowInstance
	if err := s.db.Where(
		"workspace_id = ? AND id IN ? AND LOWER(status) = ?",
		workspace.ID, instanceIDs, "running",
	).Find(&instances).Error; err != nil {
		return nil, err
	}
	byInstance := make(map[uint]workflowgorm.WorkflowInstance, len(instances))
	versionIDs, templateIDs := make([]uint, 0, len(instances)), make([]uint, 0, len(instances))
	for _, instance := range instances {
		byInstance[instance.ID] = instance
		versionIDs = append(versionIDs, instance.TemplateVersionID)
		templateIDs = append(templateIDs, instance.TemplateID)
	}
	if len(byInstance) == 0 {
		return []workflowTodoItem{}, nil
	}

	var versions []workflowgorm.WorkflowTemplateVersion
	if err := s.db.Where("workspace_id = ? AND id IN ?", workspace.ID, versionIDs).
		Find(&versions).Error; err != nil {
		return nil, err
	}
	definitions := make(map[uint]workflowDefinition, len(versions))
	for _, version := range versions {
		definition, issues := compileWorkflowDefinition(version.Definition)
		if len(issues) == 0 {
			definitions[version.ID] = definition
		}
	}
	var templates []workflowgorm.WorkflowTemplate
	if err := s.db.Select("id", "name").Where(
		"workspace_id = ? AND id IN ?", workspace.ID, templateIDs,
	).Find(&templates).Error; err != nil {
		return nil, err
	}
	templateNames := make(map[uint]string, len(templates))
	for _, template := range templates {
		templateNames[template.ID] = template.Name
	}
	aiStates, err := s.loadWorkflowTodoAIStates(workspace.ID, instanceIDs)
	if err != nil {
		return nil, err
	}

	refs := workspaceContextReferences{}
	for _, instance := range instances {
		switch instance.SubjectType {
		case "plan_unit":
			refs.ProjectIDs = appendPositiveID(refs.ProjectIDs, instance.SubjectID)
		case "requirement":
			refs.RequirementIDs = appendPositiveID(refs.RequirementIDs, instance.SubjectID)
		}
	}
	context := s.loadWorkspaceContext(workspace.ID, refs)
	search := strings.ToLower(strings.TrimSpace(c.Query("q")))
	items := make([]workflowTodoItem, 0, len(tokens))
	for _, token := range tokens {
		instance, exists := byInstance[token.InstanceID]
		if !exists {
			continue
		}
		definition, exists := definitions[instance.TemplateVersionID]
		if !exists {
			continue
		}
		node, exists := workflowNodeByID(definition, token.NodeID)
		if !exists || !workflowNodeTakesAssignee(node) {
			continue
		}
		assignee, assigneeErr := workflowTokenEffectiveAssigneeTx(s.db, instance, definition, token)
		if assigneeErr != nil || assignee == nil || *assignee != userID {
			continue
		}
		subject, exists := workflowTodoSubjectContext(context, instance)
		if !exists || !workflowTodoScopeAllows(scope, instance.SubjectType, subject) {
			continue
		}
		templateName := templateNames[instance.TemplateID]
		if !workflowTodoMatchesSearch(search, node, templateName, subject) {
			continue
		}
		aiState := aiStates[workflowTodoAIKey(instance.ID, node.ID)]
		actionKind, allowed, visible := s.workflowTodoActions(instance, definition, node, aiState)
		if !visible {
			continue
		}
		item := workflowTodoItem{
			Token: token, Instance: instance, Node: node, Subject: subject,
			TemplateName: templateName, ActionKind: actionKind, AllowedActions: allowed,
			AssigneeUserID: *assignee,
		}
		if aiState.ID > 0 {
			item.AIStatus, item.AIRunID = strings.ToLower(aiState.Status), aiState.ID
		}
		if plan, exists := workflowNodePlans(instance)[node.ID]; exists {
			item.DueAt = plan.PlannedEndAt
		}
		items = append(items, item)
	}
	sortWorkflowTodoItems(items, time.Now().UTC())
	return items, nil
}

func workflowTodoSubjectContext(
	context workspaceContext,
	instance workflowgorm.WorkflowInstance,
) (workflowTodoSubject, bool) {
	if instance.SubjectType == "plan_unit" {
		project, exists := context.Projects[instance.SubjectID]
		if !exists {
			return workflowTodoSubject{}, false
		}
		set, exists := context.ProjectSets[project.ProjectSetID]
		if !exists || project.TeamID != set.TeamID {
			return workflowTodoSubject{}, false
		}
		return workflowTodoSubject{
			ProjectSetID: set.ID, ProjectID: project.ID,
			ProjectSetName: set.Name, ProjectName: project.Title, Title: project.Title,
		}, true
	}
	if instance.SubjectType != "requirement" {
		return workflowTodoSubject{}, false
	}
	requirement, exists := context.Requirements[instance.SubjectID]
	if !exists {
		return workflowTodoSubject{}, false
	}
	set, exists := context.ProjectSets[requirement.ProjectSetID]
	if !exists || requirement.TeamID != set.TeamID {
		return workflowTodoSubject{}, false
	}
	subject := workflowTodoSubject{
		ProjectSetID: set.ID, RequirementID: requirement.ID,
		ProjectSetName: set.Name, RequirementName: requirement.Title, Title: requirement.Title,
	}
	if requirement.PlanUnitID > 0 {
		project, exists := context.Projects[requirement.PlanUnitID]
		if !exists || project.ProjectSetID != set.ID || project.TeamID != set.TeamID {
			return workflowTodoSubject{}, false
		}
		subject.ProjectID, subject.ProjectName = project.ID, project.Title
	}
	return subject, true
}

func workflowTodoScopeAllows(
	scope workspaceScope,
	subjectType string,
	subject workflowTodoSubject,
) bool {
	if len(scope.ProjectSetIDs) > 0 && !workflowTodoHasID(scope.ProjectSetIDs, subject.ProjectSetID) ||
		len(scope.ProjectIDs) > 0 && !workflowTodoHasID(scope.ProjectIDs, subject.ProjectID) {
		return false
	}
	if len(scope.RequirementIDs) == 0 {
		return true
	}
	return subjectType == "requirement" && workflowTodoHasID(scope.RequirementIDs, subject.RequirementID)
}

func workflowTodoHasID(values []uint, target uint) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func workflowTodoMatchesSearch(
	search string,
	node workflowNode,
	templateName string,
	subject workflowTodoSubject,
) bool {
	if search == "" {
		return true
	}
	return strings.Contains(strings.ToLower(strings.Join([]string{
		node.ID, node.Name, templateName, subject.ProjectSetName,
		subject.ProjectName, subject.RequirementName, subject.Title,
	}, " ")), search)
}
