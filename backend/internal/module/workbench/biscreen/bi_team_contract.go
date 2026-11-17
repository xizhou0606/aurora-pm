package biscreen

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	biTeamDimensionVersion = "version"
	biTeamDimensionMonth   = "month"
	biTeamDimensionWeek    = "week"
)

type biTeamData struct {
	Dimension        string                  `json:"dimension"`
	DimensionKey     string                  `json:"dimension_key"`
	DimensionOptions []biTeamDimensionOption `json:"dimension_options"`
	SummaryRows      []biTeamSummaryRow      `json:"summary_rows"`
	MemberGroups     []biTeamMemberGroup     `json:"member_groups"`
	ScoreProfile     biTeamScoreProfileDTO   `json:"score_profile"`
	MetricNotes      []string                `json:"metric_notes"`
}

type biTeamDimensionOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type biTeamSummaryRow struct {
	RoleKey                string                 `json:"role_key"`
	RoleLabel              string                 `json:"role_label"`
	MemberCount            int                    `json:"member_count"`
	Workload               int                    `json:"workload"`
	CompletedWorkload      int                    `json:"completed_workload"`
	AvgResponseHours       float64                `json:"avg_response_hours"`
	ResponseSampleSize     int                    `json:"response_sample_size"`
	OnTimeRatePct          float64                `json:"on_time_rate_pct"`
	OnTimeSampleSize       int                    `json:"on_time_sample_size"`
	QualityRiskRatePct     float64                `json:"quality_risk_rate_pct"`
	QualityRiskSampleSize  int                    `json:"quality_risk_sample_size"`
	ReworkCount            int                    `json:"rework_count"`
	ReworkRatePct          float64                `json:"rework_rate_pct"`
	ReworkSampleSize       int                    `json:"rework_sample_size"`
	FirstPassRatePct       float64                `json:"first_pass_rate_pct"`
	CompletionRatePct      float64                `json:"completion_rate_pct"`
	WorkloadSharePct       float64                `json:"workload_share_pct"`
	BugResponseRatePct     float64                `json:"bug_response_rate_pct"`
	BugResponseSampleSize  int                    `json:"bug_response_sample_size"`
	CompositeScore         *float64               `json:"composite_score"`
	ScoreEligible          bool                   `json:"score_eligible"`
	ScoreCoveragePct       float64                `json:"score_coverage_pct"`
	ScoreBreakdown         []biTeamScoreBreakdown `json:"score_breakdown"`
	ScoreIneligibleReasons []string               `json:"score_ineligible_reasons"`
	ScoreMissingMetricKeys []string               `json:"score_missing_metric_keys"`
	RequirementCount       int                    `json:"requirement_count"`
	ReviewPassRatePct      *float64               `json:"review_pass_rate_pct"`
	ReviewPassSampleSize   int                    `json:"review_pass_sample_size"`
	FirstPassSampleSize    int                    `json:"first_pass_sample_size"`
	BugCount               int                    `json:"bug_count"`
	BugCloseAvgHours       *float64               `json:"bug_close_avg_hours"`
	BugCloseSampleSize     int                    `json:"bug_close_sample_size"`
	InvalidBugRatePct      *float64               `json:"invalid_bug_rate_pct"`
	InvalidBugSampleSize   int                    `json:"invalid_bug_sample_size"`
	TaskCount              int                    `json:"task_count"`
	OverdueRatePct         *float64               `json:"overdue_rate_pct"`
	OverdueSampleSize      int                    `json:"overdue_sample_size"`
	BugTaskRatePct         *float64               `json:"bug_task_rate_pct"`
	BugTaskSampleSize      int                    `json:"bug_task_sample_size"`
	P0BugCount             int                    `json:"p0_bug_count"`
	TeamP0SharePct         *float64               `json:"team_p0_share_pct"`
	P1BugCount             int                    `json:"p1_bug_count"`
	TeamP1SharePct         *float64               `json:"team_p1_share_pct"`
	BugResponseAvgHours    *float64               `json:"bug_response_avg_hours"`
}

type biTeamMemberGroup struct {
	RoleKey   string         `json:"role_key"`
	RoleLabel string         `json:"role_label"`
	Members   []biTeamMember `json:"members"`
}

type biTeamMember struct {
	UserID                 uint                   `json:"user_id"`
	UserName               string                 `json:"user_name"`
	UserAvatarURL          string                 `json:"user_avatar_url"`
	Workload               int                    `json:"workload"`
	CompletedWorkload      int                    `json:"completed_workload"`
	AvgResponseHours       float64                `json:"avg_response_hours"`
	ResponseSampleSize     int                    `json:"response_sample_size"`
	OnTimeRatePct          float64                `json:"on_time_rate_pct"`
	OnTimeSampleSize       int                    `json:"on_time_sample_size"`
	QualityRiskRatePct     float64                `json:"quality_risk_rate_pct"`
	QualityRiskSampleSize  int                    `json:"quality_risk_sample_size"`
	ReworkCount            int                    `json:"rework_count"`
	ReworkRatePct          float64                `json:"rework_rate_pct"`
	ReworkSampleSize       int                    `json:"rework_sample_size"`
	FirstPassRatePct       float64                `json:"first_pass_rate_pct"`
	CompletionRatePct      float64                `json:"completion_rate_pct"`
	WorkloadSharePct       float64                `json:"workload_share_pct"`
	BugResponseRatePct     float64                `json:"bug_response_rate_pct"`
	BugResponseSampleSize  int                    `json:"bug_response_sample_size"`
	CompositeScore         *float64               `json:"composite_score"`
	ScoreEligible          bool                   `json:"score_eligible"`
	ScoreCoveragePct       float64                `json:"score_coverage_pct"`
	ScoreBreakdown         []biTeamScoreBreakdown `json:"score_breakdown"`
	ScoreIneligibleReasons []string               `json:"score_ineligible_reasons"`
	ScoreMissingMetricKeys []string               `json:"score_missing_metric_keys"`
	RequirementCount       int                    `json:"requirement_count"`
	ReviewPassRatePct      *float64               `json:"review_pass_rate_pct"`
	ReviewPassSampleSize   int                    `json:"review_pass_sample_size"`
	FirstPassSampleSize    int                    `json:"first_pass_sample_size"`
	BugCount               int                    `json:"bug_count"`
	BugCloseAvgHours       *float64               `json:"bug_close_avg_hours"`
	BugCloseSampleSize     int                    `json:"bug_close_sample_size"`
	InvalidBugRatePct      *float64               `json:"invalid_bug_rate_pct"`
	InvalidBugSampleSize   int                    `json:"invalid_bug_sample_size"`
	TaskCount              int                    `json:"task_count"`
	OverdueRatePct         *float64               `json:"overdue_rate_pct"`
	OverdueSampleSize      int                    `json:"overdue_sample_size"`
	BugTaskRatePct         *float64               `json:"bug_task_rate_pct"`
	BugTaskSampleSize      int                    `json:"bug_task_sample_size"`
	P0BugCount             int                    `json:"p0_bug_count"`
	TeamP0SharePct         *float64               `json:"team_p0_share_pct"`
	P1BugCount             int                    `json:"p1_bug_count"`
	TeamP1SharePct         *float64               `json:"team_p1_share_pct"`
	BugResponseAvgHours    *float64               `json:"bug_response_avg_hours"`
}

type biTeamScoreProfileDTO struct {
	ID                 uint                        `json:"id"`
	Version            int                         `json:"version"`
	Source             string                      `json:"source"`
	TotalWeightBPS     int                         `json:"total_weight_bps"`
	MinimumCoverageBPS int                         `json:"minimum_coverage_bps"`
	MinimumWorkload    int                         `json:"minimum_workload"`
	Items              []biTeamScoreProfileItemDTO `json:"items"`
}

type biTeamScoreProfileItemDTO struct {
	MetricKey         string   `json:"metric_key"`
	RoleKey           string   `json:"role_key"`
	Label             string   `json:"label"`
	Description       string   `json:"description"`
	Formula           string   `json:"formula"`
	Unit              string   `json:"unit"`
	Direction         string   `json:"direction"`
	Normalization     string   `json:"normalization"`
	ApplicableRoles   []string `json:"applicable_roles"`
	DefinitionVersion int      `json:"definition_version"`
	Status            string   `json:"status"`
	Enabled           bool     `json:"enabled"`
	WeightBPS         int      `json:"weight_bps"`
	WeightPct         float64  `json:"weight_pct"`
	MissingPolicy     string   `json:"missing_policy"`
	MinimumSamples    int      `json:"minimum_samples"`
	SortOrder         int      `json:"sort_order"`
}

type biTeamScoreBreakdown struct {
	MetricKey           string   `json:"metric_key"`
	RawValue            *float64 `json:"raw_value"`
	NormalizedScore     *float64 `json:"normalized_score"`
	ConfiguredWeightPct float64  `json:"configured_weight_pct"`
	EffectiveWeightPct  float64  `json:"effective_weight_pct"`
	SampleSize          int      `json:"sample_size"`
	Eligible            bool     `json:"eligible"`
}

func normalizeBITeamDimension(rawDimension, rawKey string) (string, string, error) {
	dimension := normalizeStatus(rawDimension)
	if dimension == "" {
		dimension = biTeamDimensionVersion
	}
	if dimension != biTeamDimensionVersion && dimension != biTeamDimensionMonth && dimension != biTeamDimensionWeek {
		return "", "", fmt.Errorf("team_dimension must be version, month, or week")
	}
	key := strings.TrimSpace(rawKey)
	if key == "" {
		return dimension, "", nil
	}
	if dimension == biTeamDimensionVersion {
		value, err := strconv.ParseUint(key, 10, 64)
		if err != nil || value == 0 {
			return "", "", fmt.Errorf("team_dimension_key must be a positive version id")
		}
		return dimension, strconv.FormatUint(value, 10), nil
	}
	if dimension == biTeamDimensionWeek {
		if !biValidTeamWeekKey(key) {
			return "", "", fmt.Errorf("team_dimension_key must use a valid ISO week in YYYY-Www format")
		}
		return dimension, key, nil
	}
	parsed, err := time.Parse("2006-01", key)
	if err != nil || parsed.Format("2006-01") != key {
		return "", "", fmt.Errorf("team_dimension_key must use YYYY-MM")
	}
	return dimension, key, nil
}
