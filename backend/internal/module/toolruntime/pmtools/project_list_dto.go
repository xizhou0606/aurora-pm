package pmtools

import "github.com/xizhou0606/aurora-pm/backend/internal/module/project"

type projectSetListInput struct {
	TeamID uint   `json:"team_id"`
	Query  string `json:"q"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

type projectPlanUnitListInput struct {
	ProjectSetID uint `json:"project_new_id"`
}

type projectAccessEffectiveInput struct {
	ProjectSetID uint `json:"project_set_id"`
}

type projectSetListOutput struct {
	Items []project.ProjectSetSnapshot `json:"items"`
	Total int64                        `json:"total"`
}

type projectAccessEffectiveOutput struct {
	Effective   bool                    `json:"effective"`
	Actor       projectAccessActor      `json:"actor"`
	Workspace   projectAccessWorkspace  `json:"workspace"`
	ProjectSet  projectAccessProjectSet `json:"project_set"`
	AccessBasis projectAccessBasis      `json:"access_basis"`
}

type projectAccessActor struct {
	UserID uint `json:"user_id"`
}

type projectAccessWorkspace struct {
	ID        uint `json:"id"`
	Effective bool `json:"effective"`
}

type projectAccessProjectSet struct {
	ID           uint   `json:"id"`
	Code         string `json:"code,omitempty"`
	Name         string `json:"name"`
	OwnerUserID  *uint  `json:"owner_user_id"`
	ActorIsOwner bool   `json:"actor_is_owner"`
}

type projectAccessBasis struct {
	Model                       string `json:"model"`
	WorkspaceIsDataContext      bool   `json:"workspace_is_data_context"`
	ProjectOwnerIsInformational bool   `json:"project_owner_is_informational"`
	LegacyProjectPermissionUsed bool   `json:"legacy_project_permission_used"`
}
