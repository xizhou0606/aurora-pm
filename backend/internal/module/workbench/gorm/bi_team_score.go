package workbenchgorm

import "time"

// BITeamScoreProfile is an immutable workspace-scoped scoring configuration.
// Updating weights creates a new version and archives the previously active row.
type BITeamScoreProfile struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID      uint       `gorm:"not null;uniqueIndex:ux_bi_team_score_workspace_version;index:ix_bi_team_score_active" json:"workspace_id"`
	Version          int        `gorm:"not null;uniqueIndex:ux_bi_team_score_workspace_version" json:"version"`
	Status           string     `gorm:"size:24;not null;index:ix_bi_team_score_active" json:"status"`
	BasedOnProfileID *uint      `gorm:"index" json:"based_on_profile_id,omitempty"`
	UpdatedByUserID  uint       `gorm:"not null;index" json:"updated_by_user_id"`
	ActivatedAt      *time.Time `json:"activated_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// BITeamScoreProfileItem binds a stable metric key to its weight for one profile.
// WeightBPS uses basis points, where 10_000 is 100 percent.
type BITeamScoreProfileItem struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ProfileID      uint      `gorm:"not null;uniqueIndex:ux_bi_team_score_profile_metric_role;index" json:"profile_id"`
	MetricKey      string    `gorm:"size:80;not null;uniqueIndex:ux_bi_team_score_profile_metric_role" json:"metric_key"`
	RoleKey        string    `gorm:"size:32;not null;default:all;uniqueIndex:ux_bi_team_score_profile_metric_role" json:"role_key"`
	Enabled        bool      `gorm:"not null" json:"enabled"`
	WeightBPS      int       `gorm:"not null" json:"weight_bps"`
	MissingPolicy  string    `gorm:"size:32;not null;default:renormalize" json:"missing_policy"`
	MinimumSamples int       `gorm:"not null;default:1" json:"minimum_samples"`
	ParametersJSON string    `gorm:"type:text;not null" json:"parameters_json"`
	SortOrder      int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt      time.Time `json:"created_at"`
}
