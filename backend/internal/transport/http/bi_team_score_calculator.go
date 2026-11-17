package httpapi

import "math"

type biTeamScoreInput struct {
	workload              int
	completedWorkload     int
	avgResponseHours      float64
	responseSampleSize    int
	onTimeRatePct         float64
	onTimeSampleSize      int
	qualityRiskRatePct    float64
	qualityRiskSampleSize int
	firstPassRatePct      float64
	reworkSampleSize      int
	workloadSharePct      float64
	workloadShareScorePct float64
	workloadShareSamples  int
	bugResponseRatePct    float64
	bugResponseSamples    int
}

type biTeamScoreResult struct {
	completionRatePct float64
	compositeScore    *float64
	eligible          bool
	coveragePct       float64
	breakdown         []biTeamScoreBreakdown
	ineligibleReasons []string
	missingMetricKeys []string
}

func biTeamScoreFor(
	roleKey string,
	input biTeamScoreInput,
	profile biTeamScoreProfileDTO,
) biTeamScoreResult {
	completion := biTeamPercent(input.completedWorkload, input.workload)
	result := biTeamScoreResult{completionRatePct: completion}
	type metricValue struct {
		raw        float64
		normalized float64
		samples    int
	}
	values := map[string]metricValue{
		"completion_rate": {
			raw: completion, normalized: completion, samples: input.workload,
		},
		"on_time_rate": {
			raw: input.onTimeRatePct, normalized: input.onTimeRatePct, samples: input.onTimeSampleSize,
		},
		"first_pass_rate": {
			raw: input.firstPassRatePct, normalized: input.firstPassRatePct, samples: input.reworkSampleSize,
		},
		"quality_health": {
			raw: input.qualityRiskRatePct, normalized: clampBITeamScore(100 - input.qualityRiskRatePct),
			samples: input.qualityRiskSampleSize,
		},
		"response_efficiency": {
			raw:        input.avgResponseHours,
			normalized: biTeamResponseEfficiency(roleKey, input.avgResponseHours),
			samples:    input.responseSampleSize,
		},
		"workload_share": {
			raw: input.workloadSharePct, normalized: input.workloadShareScorePct,
			samples: input.workloadShareSamples,
		},
		"bug_response_rate": {
			raw: input.bugResponseRatePct, normalized: input.bugResponseRatePct,
			samples: input.bugResponseSamples,
		},
	}
	availableWeight, configuredWeight := 0, 0
	forceIneligible := false
	for _, item := range profile.Items {
		if !item.Enabled || item.WeightBPS <= 0 || !biTeamScoreAppliesToRole(item, roleKey) {
			continue
		}
		configuredWeight += item.WeightBPS
		value := values[item.MetricKey]
		available := value.samples >= item.MinimumSamples
		if available {
			availableWeight += item.WeightBPS
		} else if item.MissingPolicy == biTeamScoreMissingIneligible {
			forceIneligible = true
		}
		if !available {
			result.missingMetricKeys = append(result.missingMetricKeys, item.MetricKey)
		}
	}
	if configuredWeight > 0 {
		result.coveragePct = biTeamRound(float64(availableWeight) * 100 / float64(configuredWeight))
	}
	eligible := input.workload >= profile.MinimumWorkload &&
		availableWeight*biTeamScoreTotalWeightBPS >= configuredWeight*profile.MinimumCoverageBPS &&
		availableWeight > 0 && !forceIneligible
	if input.workload < profile.MinimumWorkload {
		result.ineligibleReasons = append(result.ineligibleReasons, "workload_below_minimum")
	}
	if configuredWeight == 0 || availableWeight*biTeamScoreTotalWeightBPS < configuredWeight*profile.MinimumCoverageBPS {
		result.ineligibleReasons = append(result.ineligibleReasons, "coverage_below_minimum")
	}
	if forceIneligible {
		result.ineligibleReasons = append(result.ineligibleReasons, "required_metric_missing")
	}
	weightedScore := 0.0
	for _, item := range profile.Items {
		if !item.Enabled || item.WeightBPS <= 0 || !biTeamScoreAppliesToRole(item, roleKey) {
			continue
		}
		value := values[item.MetricKey]
		available := value.samples >= item.MinimumSamples
		breakdown := biTeamScoreBreakdown{
			MetricKey: item.MetricKey, ConfiguredWeightPct: item.WeightPct,
			SampleSize: value.samples, Eligible: available,
		}
		if available {
			raw, normalized := biTeamRound(value.raw), biTeamRound(clampBITeamScore(value.normalized))
			breakdown.RawValue, breakdown.NormalizedScore = &raw, &normalized
			breakdown.EffectiveWeightPct = biTeamRound(float64(item.WeightBPS) * 100 / float64(availableWeight))
			weightedScore += normalized * float64(item.WeightBPS) / float64(availableWeight)
		}
		result.breakdown = append(result.breakdown, breakdown)
	}
	if eligible {
		score := biTeamRound(clampBITeamScore(weightedScore))
		result.compositeScore, result.eligible = &score, true
	}
	return result
}

func biTeamScoreAppliesToRole(item biTeamScoreProfileItemDTO, roleKey string) bool {
	if item.RoleKey != "" && item.RoleKey != biTeamScoreRoleAll {
		return item.RoleKey == roleKey
	}
	for _, applicableRole := range item.ApplicableRoles {
		if applicableRole == roleKey {
			return true
		}
	}
	return false
}

func biTeamResponseEfficiency(roleKey string, hours float64) float64 {
	target := 24.0
	if roleKey == "testing" {
		target = 8
	}
	if hours <= target {
		return 100
	}
	if hours >= target*4 {
		return 0
	}
	return 100 * (target*4 - hours) / (target * 3)
}

func clampBITeamScore(value float64) float64 {
	return math.Max(0, math.Min(100, value))
}

func scoreInputFromAccumulator(
	metrics *biTeamAccumulator,
	roleWorkload int,
	roleMemberCount int,
) biTeamScoreInput {
	workloadShare := biTeamPercent(metrics.workload, roleWorkload)
	workloadShareScore := 0.0
	if roleWorkload > 0 && roleMemberCount > 0 {
		workloadShareScore = clampBITeamScore(
			float64(metrics.workload*roleMemberCount) * 100 / float64(roleWorkload),
		)
	}
	return biTeamScoreInput{
		workload: metrics.workload, completedWorkload: metrics.completedWorkload,
		avgResponseHours: metrics.averageResponse(), responseSampleSize: metrics.responseSamples,
		onTimeRatePct: metrics.onTimeRate(), onTimeSampleSize: metrics.onTimeSamples,
		qualityRiskRatePct: metrics.riskRate(), qualityRiskSampleSize: metrics.riskSamples,
		firstPassRatePct: metrics.firstPassRate(), reworkSampleSize: len(metrics.submittedItems),
		workloadSharePct: workloadShare, workloadShareScorePct: biTeamRound(workloadShareScore),
		workloadShareSamples: metrics.workload,
		bugResponseRatePct:   metrics.bugResponseRate(), bugResponseSamples: metrics.bugResponseSamples,
	}
}

func applyBITeamScoreToSummary(
	row *biTeamSummaryRow,
	roleKey string,
	metrics *biTeamAccumulator,
	profile biTeamScoreProfileDTO,
	roleWorkload int,
	roleMemberCount int,
) {
	result := biTeamScoreFor(roleKey, scoreInputFromAccumulator(metrics, roleWorkload, roleMemberCount), profile)
	row.CompletionRatePct, row.CompositeScore = result.completionRatePct, result.compositeScore
	row.WorkloadSharePct = biTeamPercent(metrics.workload, roleWorkload)
	row.BugResponseRatePct, row.BugResponseSampleSize = metrics.bugResponseRate(), metrics.bugResponseSamples
	row.ScoreEligible, row.ScoreCoveragePct, row.ScoreBreakdown =
		result.eligible, result.coveragePct, result.breakdown
	row.ScoreIneligibleReasons, row.ScoreMissingMetricKeys = result.ineligibleReasons, result.missingMetricKeys
}

func applyBITeamScoreToMember(
	row *biTeamMember,
	roleKey string,
	metrics *biTeamAccumulator,
	profile biTeamScoreProfileDTO,
	roleWorkload int,
	roleMemberCount int,
) {
	result := biTeamScoreFor(roleKey, scoreInputFromAccumulator(metrics, roleWorkload, roleMemberCount), profile)
	row.CompletionRatePct, row.CompositeScore = result.completionRatePct, result.compositeScore
	row.WorkloadSharePct = biTeamPercent(metrics.workload, roleWorkload)
	row.BugResponseRatePct, row.BugResponseSampleSize = metrics.bugResponseRate(), metrics.bugResponseSamples
	row.ScoreEligible, row.ScoreCoveragePct, row.ScoreBreakdown =
		result.eligible, result.coveragePct, result.breakdown
	row.ScoreIneligibleReasons, row.ScoreMissingMetricKeys = result.ineligibleReasons, result.missingMetricKeys
}
