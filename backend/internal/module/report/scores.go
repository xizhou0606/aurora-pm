package report

import "fmt"

// The multi-dimension score projection of one report. Scores are derived
// deterministically from the aggregated facts (no extra queries, no AI), so
// the web radar and the exported Feishu score table always describe the same
// numbers the narrative was based on.
//
// 评分是态势参考而不是考核结论：所有维度都来自可追溯的统计口径，依据列会
// 明确写出分子/分母，避免只给一个不可解释的数字。

// ScoreDimension is one scored axis of the report radar.
type ScoreDimension struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Score int    `json:"score"`
	// Basis renders the numerator/denominator behind the score so readers
	// can audit the number without recomputing it.
	Basis string `json:"basis"`
}

// ReportScores is the overall score plus every scored dimension.
type ReportScores struct {
	Overall    int              `json:"overall"`
	Dimensions []ScoreDimension `json:"dimensions"`
}

// Dimension keys are part of the HTTP contract and the frontend copy registry.
const (
	scoreKeyDelivery    = "delivery"
	scoreKeyPunctuality = "punctuality"
	scoreKeyQuality     = "quality"
	scoreKeyOverdue     = "overdue_control"
	scoreKeyVolume      = "volume"
	scoreKeyCode        = "code"
	scoreKeyCodeQuality = "code_quality"
)

// volumeBaseline is the completed-item count treated as full marks per
// cadence. It is a transparent heuristic (documented in the score basis and
// the UI footnote), not a quota.
func volumeBaseline(periodType PeriodType) int {
	switch periodType {
	case PeriodWeekly:
		return 10
	case PeriodMonthly:
		return 25
	default:
		return 3
	}
}

// codeLineBaseline is the additions+deletions line count treated as full
// marks per cadence, and codeCollabBaseline the merged-PR plus completed
// review count. Both are transparent heuristics like volumeBaseline: the
// basis line always renders the raw numbers against them.
func codeLineBaseline(periodType PeriodType) int {
	switch periodType {
	case PeriodWeekly:
		return 1500
	case PeriodMonthly:
		return 5000
	default:
		return 400
	}
}

func codeCollabBaseline(periodType PeriodType) int {
	switch periodType {
	case PeriodWeekly:
		return 8
	case PeriodMonthly:
		return 24
	default:
		return 2
	}
}

// ComputeReportScores projects aggregated task/bug metrics onto 0-100
// dimensions. An empty report (no work at all) collapses to zero so the radar
// reads as "no data" instead of faking a shape. Code is the optional
// code-activity aggregate: nil (workspace 未接入代码贡献) adds no axis and
// never dilutes the overall score.
func ComputeReportScores(tasks TaskMetrics, bugs BugMetrics, period Period, code *CodeActivityReport) ReportScores {
	done := tasks.Completed + bugs.Resolved
	remaining := tasks.Active + bugs.Open
	totalWork := done + remaining
	overdue := tasks.Overdue + bugs.Overdue
	onTime := tasks.CompletedOnTime + bugs.ResolvedOnTime

	dimensions := make([]ScoreDimension, 0, 5)

	// 交付完成：已完成（任务+缺陷）占全部已知工作的比例。
	delivery := 0
	deliveryBasis := "本期无工作记录"
	if totalWork > 0 {
		delivery = percent(done, totalWork)
		deliveryBasis = fmt.Sprintf("完成 %d 项，待办 %d 项", done, remaining)
	}
	dimensions = append(dimensions, ScoreDimension{
		Key: scoreKeyDelivery, Label: "交付完成", Score: delivery, Basis: deliveryBasis,
	})

	// 按期交付：按期完成占已完成的比例；无截止日期视作按期。
	punctuality := 0
	punctualityBasis := "本期无完成记录"
	if done > 0 {
		punctuality = percent(onTime, done)
		punctualityBasis = fmt.Sprintf("按期完成 %d / %d 项", onTime, done)
	}
	dimensions = append(dimensions, ScoreDimension{
		Key: scoreKeyPunctuality, Label: "按期交付", Score: punctuality, Basis: punctualityBasis,
	})

	// 质量修复：缺陷解决率。本期有工作但没有缺陷记满分；完全无工作则
	// 跟随整体归零，避免空报告得到一个形状失真的雷达图。
	bugTotal := bugs.Resolved + bugs.Open
	quality := 0
	qualityBasis := "本期无工作记录"
	if totalWork > 0 {
		if bugTotal > 0 {
			quality = percent(bugs.Resolved, bugTotal)
			qualityBasis = fmt.Sprintf("解决 %d / %d 个缺陷", bugs.Resolved, bugTotal)
		} else {
			quality = 100
			qualityBasis = "本期无缺陷记录"
		}
	}
	dimensions = append(dimensions, ScoreDimension{
		Key: scoreKeyQuality, Label: "质量修复", Score: quality, Basis: qualityBasis,
	})

	// 逾期控制：1 - 逾期占全部已知工作的比例。
	overdueControl := 0
	overdueBasis := "本期无工作记录"
	if totalWork > 0 {
		overdueControl = 100 - percent(overdue, totalWork)
		if overdue > 0 {
			overdueBasis = fmt.Sprintf("逾期 %d 项（任务 %d，缺陷 %d）", overdue, tasks.Overdue, bugs.Overdue)
		} else {
			overdueBasis = "无逾期事项"
		}
	}
	dimensions = append(dimensions, ScoreDimension{
		Key: scoreKeyOverdue, Label: "逾期控制", Score: overdueControl, Basis: overdueBasis,
	})

	// 交付量：完成量相对周期基线的饱和度，封顶 100。
	baseline := volumeBaseline(period.Type)
	volume := 0
	volumeBasis := "本期无完成记录"
	if done > 0 {
		volume = percent(done, baseline)
		if volume > 100 {
			volume = 100
		}
		volumeBasis = fmt.Sprintf("完成 %d 项（基线 %d 项）", done, baseline)
	}
	dimensions = append(dimensions, ScoreDimension{
		Key: scoreKeyVolume, Label: "交付量", Score: volume, Basis: volumeBasis,
	})

	// 代码贡献：行数与评审协作各占一半。行数用增删行合计对比基线，协作
	// 用合并 PR 与完成评审对比基线，两个分量都封顶 100；依据列写出原始
	// 数字，读者可以不重算直接审计。
	if code != nil {
		lines := code.Additions + code.Deletions
		lineScore := percent(lines, codeLineBaseline(period.Type))
		if lineScore > 100 {
			lineScore = 100
		}
		collab := code.PRsMerged + code.ReviewsCompleted
		collabScore := percent(collab, codeCollabBaseline(period.Type))
		if collabScore > 100 {
			collabScore = 100
		}
		// 评审协作缺位的直推工作流不再被腰斩：无 PR/评审时行数即得分，
		// 有协作记录才按 行数/协作 各半计。
		score := lineScore
		codeBasis := fmt.Sprintf("变更 %d 行（基线 %d）", lines, codeLineBaseline(period.Type))
		if collab > 0 {
			score = (lineScore + collabScore) / 2
			codeBasis = fmt.Sprintf("变更 %d 行（基线 %d），合并 PR %d 个 + 评审 %d 次（基线 %d）",
				lines, codeLineBaseline(period.Type), code.PRsMerged, code.ReviewsCompleted, codeCollabBaseline(period.Type))
		}
		if lines == 0 && collab == 0 {
			codeBasis = "本期无归属的代码活动"
		}
		dimensions = append(dimensions, ScoreDimension{
			Key: scoreKeyCode, Label: "代码贡献", Score: score, Basis: codeBasis,
		})

		// 代码质量：独立轴，从调试残留/TODO 命中按千行新增扣分（确定性、
		// 可解释）；AI 逐提交点评在叙述里展开，两者互相印证。
		hygiene := 100
		hygieneBasis := fmt.Sprintf("调试残留 %d 处、TODO %d 处，按千行新增扣分", code.DebugHits, code.TodoHits)
		if lines == 0 && collab == 0 {
			hygiene = 0
			hygieneBasis = "本期无归属的代码活动"
		} else if code.Additions > 0 {
			penalty := (code.DebugHits*6 + code.TodoHits*3) * 1000 / code.Additions
			if penalty > 40 {
				penalty = 40
			}
			hygiene = 100 - penalty
			hygieneBasis = fmt.Sprintf("调试残留 %d 处、TODO %d 处，扣 %d 分（千行新增口径）",
				code.DebugHits, code.TodoHits, 100-hygiene)
		}
		dimensions = append(dimensions, ScoreDimension{
			Key: scoreKeyCodeQuality, Label: "代码质量", Score: hygiene, Basis: hygieneBasis,
		})
	}

	overall := 0
	for _, dimension := range dimensions {
		overall += dimension.Score
	}
	return ReportScores{Overall: overall / len(dimensions), Dimensions: dimensions}
}

func percent(part, whole int) int {
	if whole <= 0 {
		return 0
	}
	return part * 100 / whole
}
