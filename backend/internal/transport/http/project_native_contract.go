package httpapi

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func (s *Server) getProjectSet(c *gin.Context) {
	project, allowed := s.projectFromParam(c)
	if !allowed {
		return
	}
	ok(c, s.projectDTO(c, project))
}

func (s *Server) getPlanUnit(c *gin.Context) {
	id, valid := paramID(c, "id")
	if !valid {
		return
	}
	unit, _, allowed := s.planUnitByID(c, id)
	if !allowed {
		return
	}
	ok(c, s.planUnitDTO(c, unit))
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
	AvatarURL         string                     `json:"avatar_url,omitempty"`
	IdentityAvailable bool                       `json:"identity_available"`
	Sources           []projectParticipantSource `json:"sources"`
}

func (s *Server) listProjectParticipants(c *gin.Context) {
	project, allowed := s.projectFromParam(c)
	if !allowed {
		return
	}
	participants, err := s.projectParticipants(c, project)
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{
		"project_set": gin.H{
			"id": project.ID, "code": project.Code, "name": project.Name,
		},
		"participants": participants, "total": len(participants),
		"authorization": gin.H{
			"informational_only": true,
			"grants_access":      false,
			"message":            "participation sources do not grant authorization",
		},
	})
}

func (s *Server) projectParticipants(
	c *gin.Context,
	project projectgorm.ProjectSet,
) ([]projectParticipant, error) {
	sourcesByUser := make(map[uint][]projectParticipantSource)
	seenSources := make(map[uint]map[string]struct{})
	add := func(userID uint, source projectParticipantSource) {
		if userID == 0 {
			return
		}
		key := fmt.Sprintf("%s:%d:%s", source.ResourceType, source.ResourceID, source.Relation)
		if seenSources[userID] == nil {
			seenSources[userID] = make(map[string]struct{})
		}
		if _, exists := seenSources[userID][key]; exists {
			return
		}
		seenSources[userID][key] = struct{}{}
		sourcesByUser[userID] = append(sourcesByUser[userID], source)
	}
	projectKey := firstNonEmpty(project.Code, project.Name)
	if project.OwnerUserID != nil {
		add(*project.OwnerUserID, participantSource("project_set", project.ID, projectKey, project.Name, "owner"))
	}

	// The directory only aggregates people from units/requirements/tasks the
	// caller can see under version-level visibility.
	unitsQuery := s.db.Where(
		"workspace_id = ? AND project_new_id = ?", project.WorkspaceID, project.ID,
	)
	if !s.applyProjectDataFilter(c, unitsQuery, "plan_units", "id") {
		return nil, errProjectAccessFilter
	}
	var units []projectgorm.PlanUnit
	if err := unitsQuery.Find(&units).Error; err != nil {
		return nil, err
	}
	for _, unit := range units {
		if unit.OwnerUserID != nil {
			add(*unit.OwnerUserID, participantSource(
				"project_version", unit.ID, unit.Title, unit.Title, "owner",
			))
		}
	}

	requirementsQuery := s.db.Where(
		"workspace_id = ? AND project_new_id = ?", project.WorkspaceID, project.ID,
	)
	if !s.applyProjectDataFilter(c, requirementsQuery, "requirements", "project_new_id") {
		return nil, errProjectAccessFilter
	}
	var requirements []requirementgorm.Requirement
	if err := requirementsQuery.Find(&requirements).Error; err != nil {
		return nil, err
	}
	for _, requirement := range requirements {
		source := func(relation string) projectParticipantSource {
			return participantSource("requirement", requirement.ID, requirement.Title, requirement.Title, relation)
		}
		if requirement.ProductOwnerUserID != nil {
			add(*requirement.ProductOwnerUserID, source("product_owner"))
		}
		if requirement.DesignOwnerUserID != nil {
			add(*requirement.DesignOwnerUserID, source("design_owner"))
		}
		for _, userID := range requirement.DeveloperUserIDs {
			add(userID, source("developer"))
		}
	}

	var tasks []taskgorm.WorkTask
	taskQuery := pmTaskQuery(s.db).Where(
		"workspace_id = ? AND project_set_id = ?", project.WorkspaceID, project.ID,
	)
	taskQuery = applyWorkspaceHierarchyScope(taskQuery, "work_tasks", project.WorkspaceID)
	if !s.applyProjectDataFilter(c, taskQuery, "work_tasks", "project_set_id") {
		return nil, errProjectAccessFilter
	}
	if err := taskQuery.Find(&tasks).Error; err != nil {
		return nil, err
	}
	for _, task := range tasks {
		source := func(relation string) projectParticipantSource {
			return participantSource("task", task.ID, firstNonEmpty(task.TaskKey, task.Title), task.Title, relation)
		}
		if task.AssigneeID != nil {
			add(*task.AssigneeID, source("assignee"))
		}
		if task.ReporterUserID != nil {
			add(*task.ReporterUserID, source("reporter"))
		}
	}

	var bugs []qualitygorm.QualityBug
	bugQuery := s.db.Where(
		"workspace_id = ? AND project_set_id = ?", project.WorkspaceID, project.ID,
	)
	bugQuery = applyWorkspaceHierarchyScope(bugQuery, "quality_bugs", project.WorkspaceID)
	if !s.applyProjectDataFilter(c, bugQuery, "quality_bugs", "project_set_id") {
		return nil, errProjectAccessFilter
	}
	if err := bugQuery.Find(&bugs).Error; err != nil {
		return nil, err
	}
	for _, bug := range bugs {
		source := func(relation string) projectParticipantSource {
			return participantSource("quality_bug", bug.ID, firstNonEmpty(bug.BugKey, bug.Title), bug.Title, relation)
		}
		if bug.AssigneeUserID != nil {
			add(*bug.AssigneeUserID, source("assignee"))
		}
		if bug.ReporterUserID != nil {
			add(*bug.ReporterUserID, source("reporter"))
		}
	}

	userIDs := make([]uint, 0, len(sourcesByUser))
	for userID := range sourcesByUser {
		userIDs = append(userIDs, userID)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	profiles, err := s.projectParticipantProfiles(c, project.WorkspaceID, userIDs)
	if err != nil {
		return nil, err
	}
	result := make([]projectParticipant, 0, len(userIDs))
	for _, userID := range userIDs {
		sources := sourcesByUser[userID]
		sort.SliceStable(sources, func(i, j int) bool {
			left := sources[i].ResourceType + "\x00" + sources[i].ResourceKey + "\x00" + sources[i].Relation
			right := sources[j].ResourceType + "\x00" + sources[j].ResourceKey + "\x00" + sources[j].Relation
			return left < right
		})
		profile, found := profiles[userID]
		result = append(result, projectParticipant{
			UserID: userID, Username: profile.Username, DisplayName: profile.DisplayName,
			AvatarURL: profile.AvatarURL, IdentityAvailable: found, Sources: sources,
		})
	}
	return result, nil
}

func participantSource(resourceType string, resourceID uint, resourceKey, title, relation string) projectParticipantSource {
	return projectParticipantSource{
		ResourceType: resourceType, ResourceID: resourceID, ResourceKey: strings.TrimSpace(resourceKey),
		ResourceTitle: strings.TrimSpace(title), Relation: relation,
	}
}

func (s *Server) projectParticipantProfiles(
	_ *gin.Context,
	_ uint,
	userIDs []uint,
) (map[uint]identitygorm.User, error) {
	return s.findActivePMUsersByReferenceIDs(userIDs)
}

func (s *Server) getEffectiveProjectAccess(c *gin.Context) {
	projectID := queryUint(c, "project_set_id", "project_new_id")
	if projectID == 0 {
		bad(c, "project_set_id required")
		return
	}
	project, allowed := s.projectByQuery(c, projectID)
	if !allowed {
		return
	}
	actor, _ := currentActor(c)
	principal, principalOK := currentPrincipal(c)
	workspace, workspaceOK := currentWorkspace(c)
	actorID := factUserID(actor.User)
	ownerMatch := project.OwnerUserID != nil && *project.OwnerUserID == actorID
	owner := gin.H{"user_id": project.OwnerUserID, "username": "", "display_name": "", "identity_available": false}
	if project.OwnerUserID != nil {
		profiles, err := s.projectParticipantProfiles(c, workspace.ID, []uint{*project.OwnerUserID})
		if err != nil {
			dbFail(c)
			return
		}
		if profile, found := profiles[*project.OwnerUserID]; found {
			owner["username"] = profile.Username
			owner["display_name"] = profile.DisplayName
			owner["identity_available"] = true
		}
	}
	ok(c, gin.H{
		"effective": principalOK && workspaceOK,
		"actor": gin.H{
			"user_id": actorID, "pm_user_id": principal.UserID,
			"username": actor.User.Username, "source": actor.Source,
		},
		"workspace": gin.H{"id": workspace.ID, "name": workspace.Name, "effective": workspaceOK},
		"project_set": gin.H{
			"id": project.ID, "code": project.Code, "name": project.Name,
			"owner": owner, "actor_is_owner": ownerMatch,
		},
		"access_basis": gin.H{
			"model": "global_role_interface_grants", "workspace_is_data_context": true,
			"project_owner_is_informational": true,
			"legacy_project_permission_used": false,
		},
	})
}

// errProjectAccessFilter signals the transport that the caller's access
// filter could not be built; the endpoint fails closed.
var errProjectAccessFilter = errors.New("project access filter unavailable")

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
