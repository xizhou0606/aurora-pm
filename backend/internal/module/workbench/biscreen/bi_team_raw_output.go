package biscreen

func applyRawSummary(row *biTeamSummaryRow, metric *biTeamRawAccumulator, teamP0, teamP1 int, roleKey string) {
	applyRawValues(metric, teamP0,
		func(requirementCount, bugCount, taskCount, p0 int) {
			row.RequirementCount, row.BugCount, row.TaskCount, row.P0BugCount = requirementCount, bugCount, taskCount, p0
		},
		func(reviewRate *float64, reviewSamples int, firstRate float64, firstSamples int) {
			row.ReviewPassRatePct, row.ReviewPassSampleSize = reviewRate, reviewSamples
			row.FirstPassSampleSize = firstSamples
			if roleKey == "product" && firstSamples > 0 {
				row.FirstPassRatePct = firstRate
			}
		},
		func(closeAvg *float64, closeSamples int, invalidRate *float64, invalidSamples int) {
			row.BugCloseAvgHours, row.BugCloseSampleSize = closeAvg, closeSamples
			row.InvalidBugRatePct, row.InvalidBugSampleSize = invalidRate, invalidSamples
		},
		func(overdueRate *float64, overdueSamples int, bugTask *float64, bugTaskSamples int, p0Share *float64, responseAvg *float64, responseSamples int) {
			row.OverdueRatePct, row.OverdueSampleSize = overdueRate, overdueSamples
			row.BugTaskRatePct, row.BugTaskSampleSize, row.TeamP0SharePct = bugTask, bugTaskSamples, p0Share
			row.BugResponseAvgHours = responseAvg
			if roleKey == "development" {
				row.BugResponseSampleSize = responseSamples
			}
		},
	)
	row.P1BugCount = metric.p1BugCount
	row.TeamP1SharePct = biTeamOptionalPercent(metric.p1BugCount, teamP1)
}

func applyRawMember(row *biTeamMember, metric *biTeamRawAccumulator, teamP0, teamP1 int, roleKey string) {
	applyRawValues(metric, teamP0,
		func(requirementCount, bugCount, taskCount, p0 int) {
			row.RequirementCount, row.BugCount, row.TaskCount, row.P0BugCount = requirementCount, bugCount, taskCount, p0
		},
		func(reviewRate *float64, reviewSamples int, firstRate float64, firstSamples int) {
			row.ReviewPassRatePct, row.ReviewPassSampleSize = reviewRate, reviewSamples
			row.FirstPassSampleSize = firstSamples
			if roleKey == "product" && firstSamples > 0 {
				row.FirstPassRatePct = firstRate
			}
		},
		func(closeAvg *float64, closeSamples int, invalidRate *float64, invalidSamples int) {
			row.BugCloseAvgHours, row.BugCloseSampleSize = closeAvg, closeSamples
			row.InvalidBugRatePct, row.InvalidBugSampleSize = invalidRate, invalidSamples
		},
		func(overdueRate *float64, overdueSamples int, bugTask *float64, bugTaskSamples int, p0Share *float64, responseAvg *float64, responseSamples int) {
			row.OverdueRatePct, row.OverdueSampleSize = overdueRate, overdueSamples
			row.BugTaskRatePct, row.BugTaskSampleSize, row.TeamP0SharePct = bugTask, bugTaskSamples, p0Share
			row.BugResponseAvgHours = responseAvg
			if roleKey == "development" {
				row.BugResponseSampleSize = responseSamples
			}
		},
	)
	row.P1BugCount = metric.p1BugCount
	row.TeamP1SharePct = biTeamOptionalPercent(metric.p1BugCount, teamP1)
}

func applyRawValues(
	metric *biTeamRawAccumulator,
	teamP0 int,
	counts func(int, int, int, int),
	product func(*float64, int, float64, int),
	testing func(*float64, int, *float64, int),
	development func(*float64, int, *float64, int, *float64, *float64, int),
) {
	counts(metric.requirementCount, metric.bugCount, metric.taskCount, metric.p0BugCount)
	product(
		biTeamOptionalPercent(metric.reviewPass, metric.reviewSamples), metric.reviewSamples,
		biTeamPercent(metric.firstPass, metric.firstPassSamples), metric.firstPassSamples,
	)
	var closeAverage *float64
	if metric.bugCloseSamples > 0 {
		value := biTeamRound(metric.bugCloseHours / float64(metric.bugCloseSamples))
		closeAverage = &value
	}
	testing(closeAverage, metric.bugCloseSamples,
		biTeamOptionalPercent(metric.invalidBugs, metric.invalidBugSamples), metric.invalidBugSamples)
	var bugTaskRate *float64
	if metric.taskCount > 0 {
		value := biTeamRound(float64(metric.bugTaskBugs) * 100 / float64(metric.taskCount))
		bugTaskRate = &value
	}
	var p0Share *float64
	if teamP0 > 0 {
		value := biTeamRound(float64(metric.p0BugCount) * 100 / float64(teamP0))
		p0Share = &value
	}
	var responseAverage *float64
	if metric.bugResponseSamples > 0 {
		value := biTeamRound(metric.bugResponseHours / float64(metric.bugResponseSamples))
		responseAverage = &value
	}
	development(
		biTeamOptionalPercent(metric.overdue, metric.overdueSamples), metric.overdueSamples,
		bugTaskRate, metric.taskCount, p0Share, responseAverage, metric.bugResponseSamples,
	)
}

func biTeamOptionalPercent(numerator, denominator int) *float64 {
	if denominator == 0 {
		return nil
	}
	value := biTeamPercent(numerator, denominator)
	return &value
}
