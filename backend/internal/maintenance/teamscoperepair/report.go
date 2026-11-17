package teamscoperepair

import "time"

type Options struct {
	WorkspaceID   uint
	TargetTeamID  uint
	ProjectSetIDs []uint
	DryRun        bool
}

type ProjectSetReport struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	CurrentTeamID uint   `json:"current_team_id"`
}

type TableReport struct {
	Scoped      int64 `json:"scoped"`
	WouldUpdate int64 `json:"would_update"`
	Updated     int64 `json:"updated"`
}

type TeamMembershipReport struct {
	ReferencedFactUserIDs []uint `json:"referenced_fact_user_ids,omitempty"`
	MissingFactUserIDs    []uint `json:"missing_fact_user_ids,omitempty"`
	UnresolvedFactUserIDs []uint `json:"unresolved_fact_user_ids,omitempty"`
	Existing              int64  `json:"existing"`
	WouldInsert           int64  `json:"would_insert"`
	Inserted              int64  `json:"inserted"`
}

type Report struct {
	DryRun               bool                   `json:"dry_run"`
	WorkspaceID          uint                   `json:"workspace_id"`
	TargetTeamID         uint                   `json:"target_team_id"`
	ProjectSets          []ProjectSetReport     `json:"project_sets"`
	Tables               map[string]TableReport `json:"tables"`
	TeamMemberships      TeamMembershipReport   `json:"team_memberships"`
	IssuesBefore         map[string]int64       `json:"issues_before"`
	ProjectedIssuesAfter map[string]int64       `json:"projected_issues_after"`
	IssuesAfter          map[string]int64       `json:"issues_after,omitempty"`
	Ready                bool                   `json:"ready"`
	Applied              bool                   `json:"applied"`
	RolledBack           bool                   `json:"rolled_back"`
	StartedAt            time.Time              `json:"started_at"`
	FinishedAt           time.Time              `json:"finished_at"`
}
