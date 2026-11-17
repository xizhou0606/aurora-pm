package auth

// Request context keys shared by HTTP adapters after authentication and
// workspace selection have completed.
const (
	ActorContextKey           = "pm_actor"
	ActorFactUserIDContextKey = "pm_actor_fact_user_id"
	PrincipalContextKey       = "pm_principal"
	WorkspaceContextKey       = "pm_workspace"
	WorkspaceIDContextKey     = "pm_workspace_id"
)
