package httpapi

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

type workspaceContext struct {
	ProjectSets  map[uint]projectgorm.ProjectSet
	Projects     map[uint]projectgorm.PlanUnit
	Requirements map[uint]requirementgorm.Requirement
	Users        map[uint]identitygorm.User
}

type workspaceContextReferences struct {
	ProjectSetIDs  []uint
	ProjectIDs     []uint
	RequirementIDs []uint
	UserIDs        []uint
}

func (refs workspaceContextReferences) withTask(task taskgorm.WorkTask) workspaceContextReferences {
	refs.ProjectSetIDs = appendPositiveID(refs.ProjectSetIDs, task.ProjectSetID)
	refs.ProjectIDs = appendPositiveID(refs.ProjectIDs, task.ProjectID)
	refs.RequirementIDs = appendPositiveID(refs.RequirementIDs, task.RequirementID)
	refs.UserIDs = appendOptionalID(refs.UserIDs, task.AssigneeID)
	refs.UserIDs = appendOptionalID(refs.UserIDs, task.ReporterUserID)
	return refs
}

func (refs workspaceContextReferences) withTasks(tasks []taskgorm.WorkTask) workspaceContextReferences {
	for _, task := range tasks {
		refs = refs.withTask(task)
	}
	return refs
}

func (refs workspaceContextReferences) withQualityBug(bug qualitygorm.QualityBug) workspaceContextReferences {
	refs.ProjectSetIDs = appendPositiveID(refs.ProjectSetIDs, bug.ProjectSetID)
	refs.ProjectIDs = appendPositiveID(refs.ProjectIDs, bug.ProjectID)
	refs.RequirementIDs = appendPositiveID(refs.RequirementIDs, bug.RequirementID)
	refs.UserIDs = appendOptionalID(refs.UserIDs, bug.AssigneeUserID)
	refs.UserIDs = appendOptionalID(refs.UserIDs, bug.ReporterUserID)
	refs.UserIDs = appendOptionalID(refs.UserIDs, bug.ReviewerUserID)
	return refs
}

func (refs workspaceContextReferences) withQualityBugs(bugs []qualitygorm.QualityBug) workspaceContextReferences {
	for _, bug := range bugs {
		refs = refs.withQualityBug(bug)
	}
	return refs
}

func (refs workspaceContextReferences) withUserIDs(ids ...uint) workspaceContextReferences {
	for _, id := range ids {
		refs = refs.withUserID(id)
	}
	return refs
}

func (refs workspaceContextReferences) withUserID(id uint) workspaceContextReferences {
	refs.UserIDs = appendPositiveID(refs.UserIDs, id)
	return refs
}

func appendPositiveID(ids []uint, id uint) []uint {
	if id > 0 {
		return append(ids, id)
	}
	return ids
}

func appendOptionalID(ids []uint, id *uint) []uint {
	if id == nil {
		return ids
	}
	return appendPositiveID(ids, *id)
}

func (s *Server) loadWorkspaceContext(
	workspaceID uint,
	refs workspaceContextReferences,
) workspaceContext {
	ctx := workspaceContext{
		ProjectSets: map[uint]projectgorm.ProjectSet{}, Projects: map[uint]projectgorm.PlanUnit{},
		Requirements: map[uint]requirementgorm.Requirement{}, Users: map[uint]identitygorm.User{},
	}
	projectSetIDs := mergeSortedUserIDs(refs.ProjectSetIDs)
	projectIDs := mergeSortedUserIDs(refs.ProjectIDs)
	requirementIDs := mergeSortedUserIDs(refs.RequirementIDs)
	userIDs := mergeSortedUserIDs(refs.UserIDs)

	var requirements []requirementgorm.Requirement
	if len(requirementIDs) > 0 {
		_ = s.db.Where("workspace_id = ? AND id IN ?", workspaceID, requirementIDs).Find(&requirements).Error
	}
	for _, row := range requirements {
		ctx.Requirements[row.ID] = row
		projectSetIDs = appendPositiveID(projectSetIDs, row.ProjectSetID)
		projectIDs = appendPositiveID(projectIDs, row.PlanUnitID)
		userIDs = appendOptionalID(userIDs, row.ProductOwnerUserID)
		userIDs = appendOptionalID(userIDs, row.DesignOwnerUserID)
		userIDs = append(userIDs, row.FrontendOwnerUserIDs...)
		userIDs = append(userIDs, row.BackendOwnerUserIDs...)
		userIDs = append(userIDs, row.TesterUserIDs...)
		userIDs = append(userIDs, row.DeveloperUserIDs...)
	}
	projectSetIDs = mergeSortedUserIDs(projectSetIDs)
	projectIDs = mergeSortedUserIDs(projectIDs)
	var projects []projectgorm.PlanUnit
	if len(projectIDs) > 0 {
		_ = s.db.Where("workspace_id = ? AND id IN ?", workspaceID, projectIDs).Find(&projects).Error
	}
	for _, row := range projects {
		ctx.Projects[row.ID] = row
		projectSetIDs = appendPositiveID(projectSetIDs, row.ProjectSetID)
		userIDs = appendOptionalID(userIDs, row.OwnerUserID)
	}
	projectSetIDs = mergeSortedUserIDs(projectSetIDs)
	var sets []projectgorm.ProjectSet
	if len(projectSetIDs) > 0 {
		_ = s.db.Where("workspace_id = ? AND id IN ?", workspaceID, projectSetIDs).Find(&sets).Error
	}
	for _, row := range sets {
		ctx.ProjectSets[row.ID] = row
		userIDs = appendOptionalID(userIDs, row.OwnerUserID)
	}
	userIDs = mergeSortedUserIDs(userIDs)
	users, err := s.findActivePMUsersByFactIDs(userIDs)
	if err == nil {
		for id, user := range users {
			ctx.Users[id] = user
		}
	}
	return ctx
}

func (s *Server) loadStrictWorkspaceContext(
	_ context.Context,
	workspaceID uint,
	userIDs []uint,
) (workspaceContext, error) {
	return s.loadWorkspaceContext(workspaceID, workspaceContextReferences{UserIDs: userIDs}), nil
}

func (ctx workspaceContext) enrichTask(row gin.H, task taskgorm.WorkTask) gin.H {
	projectSet := ctx.ProjectSets[task.ProjectSetID]
	project := ctx.Projects[task.ProjectID]
	requirement := ctx.Requirements[task.RequirementID]
	row["project_set_name"] = projectSet.Name
	row["project_name"] = project.Title
	row["requirement_title"] = requirement.Title
	row["scope"] = gin.H{
		"project_set": gin.H{
			"id": projectSet.ID, "code": projectSet.Code, "name": projectSet.Name,
		},
		"project_version": gin.H{
			"id": project.ID, "title": project.Title, "type": project.Type,
		},
		"requirement": gin.H{
			"id": requirement.ID, "title": requirement.Title,
		},
	}
	if task.AssigneeID != nil {
		fallback := retiredUserDisplayName(*task.AssigneeID)
		row["assignee_name"] = fallback
		row["assignee_display_name"] = fallback
		row["assignee_username"] = ""
		if user, ok := ctx.Users[*task.AssigneeID]; ok {
			row["assignee_name"] = user.DisplayName
			row["assignee_display_name"] = user.DisplayName
			row["assignee_username"] = user.Username
		}
	}
	if task.ReporterUserID != nil {
		fallback := retiredUserDisplayName(*task.ReporterUserID)
		row["reporter_name"] = fallback
		row["reporter_display_name"] = fallback
		row["reporter_username"] = ""
		if user, ok := ctx.Users[*task.ReporterUserID]; ok {
			row["reporter_name"] = user.DisplayName
			row["reporter_display_name"] = user.DisplayName
			row["reporter_username"] = user.Username
		}
	}
	return row
}

func (ctx workspaceContext) enrichTaskCreator(row gin.H, creatorID uint) gin.H {
	if creatorID == 0 {
		return row
	}
	row["created_by_user_id"] = creatorID
	fallback := retiredUserDisplayName(creatorID)
	row["created_by_name"] = fallback
	row["created_by_display_name"] = fallback
	row["created_by_username"] = ""
	if user, ok := ctx.Users[creatorID]; ok {
		row["created_by_name"] = user.DisplayName
		row["created_by_display_name"] = user.DisplayName
		row["created_by_username"] = user.Username
	}
	return row
}

func userDisplayName(ctx workspaceContext, id *uint) string {
	if id == nil || *id == 0 {
		return ""
	}
	if user, ok := ctx.Users[*id]; ok {
		if user.DisplayName != "" {
			return user.DisplayName
		}
		return user.Username
	}
	return retiredUserDisplayName(*id)
}

func retiredUserDisplayName(id uint) string {
	if id == 0 {
		return ""
	}
	return fmt.Sprintf("用户 #%d", id)
}

func userAvatarURL(ctx workspaceContext, id *uint) string {
	if id == nil || *id == 0 {
		return ""
	}
	return ctx.Users[*id].AvatarURL
}
