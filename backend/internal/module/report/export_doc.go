package report

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// The complete Feishu document body: facts first (meta callout, metrics,
// score table, itemised details), then the AI narrative verbatim. The
// integration writer parses this markdown into native Feishu docx blocks —
// headings, callouts, tables, and lists — so the published document reads
// like a report instead of a plain-text dump.
//
// 事实段落与网页预览同源（同一个 Report 投影），AI 评述用 run 自己的输出，
// 因此文档内容与生成时的快照一致。

const exportMarkdownEmpty = "本期无记录。"

// composeReportRunContent rebuilds the document body from the run's stored
// facts snapshot. It degrades to the bare summary when the stored input
// cannot be parsed (legacy runs), so export never hard-fails on old data.
func composeReportRunContent(runInputJSON, summary string) string {
	var payload struct {
		Subject Report `json:"subject"`
	}
	if err := json.Unmarshal([]byte(runInputJSON), &payload); err != nil || payload.Subject.Target.ID == 0 {
		return summary
	}
	return composeReportDocumentMarkdown(payload.Subject, summary)
}

// composeReportDocumentMarkdown renders one report into the full document
// markdown.
func composeReportDocumentMarkdown(report Report, summary string) string {
	var builder strings.Builder
	word := cadenceWordingFor(report.Period)

	builder.WriteString("> 📌 对象：" + report.Target.Name + typeSuffix(report.Target.Kind) +
		" ｜ 统计区间：" + report.Period.Start.Format(anchorDateLayout) + " 至 " + report.Period.AnchorDate +
		" ｜ 生成时间：" + report.GeneratedAt.Format(anchorDateLayout) + "\n\n")

	writeMetricsTable(&builder, report)
	writeCodeSection(&builder, report)
	writeBugReportsSection(&builder, report)
	writeRequirementSection(&builder, report)
	writeUIBugsSection(&builder, report)
	writeScoresTable(&builder, report)
	writeCompletedSection(&builder, report, word.label)
	writeOpenSection(&builder, report)
	if report.Target.Kind == TargetGroup || report.Target.Kind == TargetDepartment {
		writeMemberTable(&builder, report)
	}
	// 事实版导出（未生成 AI 评述）不带空章节，直接以事实明细收尾。
	if strings.TrimSpace(summary) != "" {
		writeNarrative(&builder, summary)
	}
	return builder.String()
}

func typeSuffix(kind ReportTargetKind) string {
	switch kind {
	case TargetGroup:
		return "（用户组）"
	case TargetDepartment:
		return "（部门）"
	default:
		return ""
	}
}

// writeCodeSection renders the 代码活动 facts. Code volume is display-only
// (不进评分)，采集异常时显式说明，避免被读成"零贡献"。
func writeCodeSection(builder *strings.Builder, report Report) {
	if report.Code == nil {
		return
	}
	builder.WriteString("## 💻 代码活动\n\n")
	builder.WriteString("| 指标 | 数量 |\n| --- | --- |\n")
	rows := [][2]string{
		{"覆盖仓库", itoa(report.Code.Repos)},
		{"合入请求", itoa(report.Code.PRsMerged)},
		{"直接提交", itoa(report.Code.Commits)},
		{"新增行数", itoa(report.Code.Additions)},
		{"删除行数", itoa(report.Code.Deletions)},
		{"完成评审", itoa(report.Code.ReviewsCompleted)},
	}
	for _, row := range rows {
		builder.WriteString("| " + row[0] + " | " + row[1] + " |\n")
	}
	// 行数构成只在有拆分数据时输出（历史聚合行四类为 0，仅总数可用）。
	if classSum := report.Code.SourceAdditions + report.Code.TestAdditions +
		report.Code.DocAdditions + report.Code.ConfigAdditions; classSum > 0 {
		builder.WriteString("\n新增行数构成：源码 " + itoa(report.Code.SourceAdditions) +
			"，测试 " + itoa(report.Code.TestAdditions) +
			"，文档 " + itoa(report.Code.DocAdditions) +
			"，配置 " + itoa(report.Code.ConfigAdditions) + "\n")
	}
	if report.Code.PRsMerged > 0 {
		builder.WriteString("\n评审覆盖：" + itoa(report.Code.PRsReviewed) + "/" +
			itoa(report.Code.PRsMerged) + " 个合入请求带评审\n")
	} else if report.Code.Commits > 0 {
		builder.WriteString("\n本期变更全部为直接提交，未经评审合入\n")
	}
	if len(report.Code.CommitReviews) > 0 {
		builder.WriteString("\n### 逐提交明细\n\n| 日期 | 提交 | 增/删 | 调试残留 | TODO | AI 结论 |\n| --- | --- | --- | --- | --- | --- |\n")
		pipe := strings.NewReplacer("|", "\\|", "\n", " ")
		for _, item := range report.Code.CommitReviews {
			ai := "—"
			if item.AiVerdict != "" {
				ai = item.AiVerdict
				if item.AiSummary != "" {
					ai += "：" + pipe.Replace(item.AiSummary)
				}
			}
			builder.WriteString("| " + item.Day + " | " + pipe.Replace(item.Subject) +
				" | +" + itoa(item.Additions) + "/-" + itoa(item.Deletions) +
				" | " + itoa(item.DebugHits) + " | " + itoa(item.TodoHits) + " | " + ai + " |\n")
		}
	}
	if report.Code.Freshness.Degraded {
		builder.WriteString("\n> ⚠️ 采集状态异常（" + report.Code.Freshness.Error +
			"）：以上为已采集数据，近期变更可能缺失，不代表当期无活动。\n\n")
	}
}

// writeBugReportsSection renders the 测试岗/测试团队 core facts: raised bugs
// by severity. BugReports 与 Code 互斥，二者至多渲染一节。
func writeBugReportsSection(builder *strings.Builder, report Report) {
	if report.BugReports == nil {
		return
	}
	builder.WriteString("## 🐞 提出 Bug\n\n")
	builder.WriteString("| 指标 | 数量 |\n| --- | --- |\n")
	rows := [][2]string{
		{"提出总数", itoa(report.BugReports.Total)},
		{"P0 致命", itoa(report.BugReports.P0)},
		{"P1 严重", itoa(report.BugReports.P1)},
		{"P2 一般", itoa(report.BugReports.P2)},
	}
	for _, row := range rows {
		builder.WriteString("| " + row[0] + " | " + row[1] + " |\n")
	}
	builder.WriteString("\n")
}

// writeRequirementSection renders the 产品岗/产品团队 core facts: owned
// requirements with their status distribution (当前快照口径).
func writeRequirementSection(builder *strings.Builder, report Report) {
	if report.Requirements == nil {
		return
	}
	builder.WriteString("## 📋 需求概览\n\n")
	builder.WriteString("| 指标 | 数量 |\n| --- | --- |\n")
	rows := [][2]string{
		{"负责需求总数", itoa(report.Requirements.Total)},
		{"本期新增", itoa(report.Requirements.CreatedInPeriod)},
		{"已完成", itoa(report.Requirements.Done)},
		{"验收中", itoa(report.Requirements.Acceptance)},
		{"开发中", itoa(report.Requirements.Development)},
		{"待评审/需求池", itoa(report.Requirements.Pending)},
	}
	for _, row := range rows {
		builder.WriteString("| " + row[0] + " | " + row[1] + " |\n")
	}
	builder.WriteString("\n")
}

// writeUIBugsSection renders the 设计岗/设计团队 core facts: UI-domain
// defect handling. UIBugs 与 Code 互斥，二者至多渲染一节。
func writeUIBugsSection(builder *strings.Builder, report Report) {
	if report.UIBugs == nil {
		return
	}
	builder.WriteString("## 🎨 UI 缺陷\n\n")
	builder.WriteString("| 指标 | 数量 |\n| --- | --- |\n")
	rows := [][2]string{
		{"本期解决", itoa(report.UIBugs.Resolved)},
		{"待处理", itoa(report.UIBugs.Open)},
		{"逾期", itoa(report.UIBugs.Overdue)},
	}
	for _, row := range rows {
		builder.WriteString("| " + row[0] + " | " + row[1] + " |\n")
	}
	builder.WriteString("\n")
}

func writeMetricsTable(builder *strings.Builder, report Report) {
	builder.WriteString("## 📊 数据指标\n\n")
	builder.WriteString("| 指标 | 数量 |\n| --- | --- |\n")
	rows := [][2]string{
		{"完成任务", itoa(report.Tasks.Completed)},
		{"新增任务", itoa(report.Tasks.Created)},
		{"进行中任务", itoa(report.Tasks.Active)},
		{"逾期任务", itoa(report.Tasks.Overdue)},
		{"解决缺陷", itoa(report.Bugs.Resolved)},
		{"关闭缺陷", itoa(report.Bugs.Closed)},
		{"待处理缺陷", itoa(report.Bugs.Open)},
		{"逾期缺陷", itoa(report.Bugs.Overdue)},
	}
	for _, row := range rows {
		builder.WriteString("| " + row[0] + " | " + row[1] + " |\n")
	}
	builder.WriteString("\n")
}

func writeScoresTable(builder *strings.Builder, report Report) {
	builder.WriteString(fmt.Sprintf("## 🎯 多维评分（综合 %d / 100）\n\n", report.Scores.Overall))
	builder.WriteString("| 维度 | 得分 | 依据 |\n| --- | --- | --- |\n")
	for _, dimension := range report.Scores.Dimensions {
		builder.WriteString("| " + cellText(dimension.Label) + " | " + itoa(dimension.Score) +
			" | " + cellText(dimension.Basis) + " |\n")
	}
	builder.WriteString("\n评分为统计口径自动计算，仅作态势参考。\n\n")
}

func writeCompletedSection(builder *strings.Builder, report Report, label string) {
	builder.WriteString("## ✅ " + label + "完成\n\n")
	if report.Tasks.Completed == 0 && report.Bugs.Resolved == 0 {
		builder.WriteString(exportMarkdownEmpty + "\n\n")
		return
	}
	if len(report.Tasks.CompletedItems) > 0 {
		builder.WriteString("| 任务 | 完成时间 |\n| --- | --- |\n")
		for _, item := range report.Tasks.CompletedItems {
			builder.WriteString("| " + cellText(itemLabel(item.Key, item.Title)) + " | " +
				momentDate(item.CompletedAt) + " |\n")
		}
		builder.WriteString(truncationNote(report.Tasks.Completed, len(report.Tasks.CompletedItems)))
	}
	if len(report.Bugs.ResolvedItems) > 0 {
		builder.WriteString("| 缺陷 | 严重级 | 解决时间 |\n| --- | --- | --- |\n")
		for _, item := range report.Bugs.ResolvedItems {
			builder.WriteString("| " + cellText(itemLabel(item.Key, item.Title)) + " | " +
				strings.ToUpper(cellText(item.Severity)) + " | " + momentDate(item.ResolvedAt) + " |\n")
		}
		builder.WriteString(truncationNote(report.Bugs.Resolved, len(report.Bugs.ResolvedItems)))
	}
	builder.WriteString("\n")
}

func writeOpenSection(builder *strings.Builder, report Report) {
	builder.WriteString("## 🔄 进行中与风险\n\n")
	sections := 0
	if len(report.Tasks.OverdueItems) > 0 {
		sections++
		builder.WriteString("### ⏰ 逾期任务\n")
		for _, item := range report.Tasks.OverdueItems {
			builder.WriteString("- " + itemLabel(item.Key, item.Title) + "（截止 " + momentDate(item.DueAt) + "）\n")
		}
		builder.WriteString(truncationNote(report.Tasks.Overdue, len(report.Tasks.OverdueItems)))
	}
	if len(report.Tasks.ActiveItems) > 0 {
		sections++
		builder.WriteString("### 🚧 在办任务\n")
		for _, item := range report.Tasks.ActiveItems {
			line := "- " + itemLabel(item.Key, item.Title)
			if item.DueAt != nil {
				line += "（截止 " + momentDate(item.DueAt) + "）"
			}
			builder.WriteString(line + "\n")
		}
		builder.WriteString(truncationNote(report.Tasks.Active, len(report.Tasks.ActiveItems)))
	}
	if len(report.Bugs.OpenItems) > 0 {
		sections++
		builder.WriteString("### 🐞 待处理缺陷\n")
		for _, item := range report.Bugs.OpenItems {
			line := "- 【" + strings.ToUpper(item.Severity) + "】" + itemLabel(item.Key, item.Title)
			if item.DueAt != nil {
				line += "（截止 " + momentDate(item.DueAt) + "）"
			}
			builder.WriteString(line + "\n")
		}
		builder.WriteString(truncationNote(report.Bugs.Open, len(report.Bugs.OpenItems)))
	}
	if sections == 0 {
		builder.WriteString("当前无在办事项与未关闭缺陷。\n")
	}
	builder.WriteString("\n")
}

func writeMemberTable(builder *strings.Builder, report Report) {
	if len(report.Members) == 0 {
		return
	}
	// 超大组的嵌套表格请求体有上限；明细截断并明示，避免静默丢失。
	const memberTableLimit = 50
	members := report.Members
	if len(members) > memberTableLimit {
		members = members[:memberTableLimit]
	}
	focus := teamRoleFocus(report.Target)
	switch focus {
	case focusTesting:
		// 测试团队：成员概览把缺陷解决列换成提出缺陷分档。
		builder.WriteString("## 👥 成员概览\n\n")
		builder.WriteString("| 成员 | 完成 | 进行中 | 逾期 | 提出 Bug | P0 | P1 | P2 |\n| --- | --- | --- | --- | --- | --- | --- | --- |\n")
		for _, member := range members {
			builder.WriteString("| " + cellText(member.Member.DisplayName) +
				" | " + itoa(member.Tasks.Completed) +
				" | " + itoa(member.Tasks.Active) +
				" | " + itoa(member.Tasks.Overdue) +
				" | " + memberBugReportsCell(member.BugReports, func(metrics BugReportMetrics) int { return metrics.Total }) +
				" | " + memberBugReportsCell(member.BugReports, func(metrics BugReportMetrics) int { return metrics.P0 }) +
				" | " + memberBugReportsCell(member.BugReports, func(metrics BugReportMetrics) int { return metrics.P1 }) +
				" | " + memberBugReportsCell(member.BugReports, func(metrics BugReportMetrics) int { return metrics.P2 }) + " |\n")
		}
	case focusProduct:
		// 产品团队：成员概览把缺陷解决列换成需求数量事实。
		builder.WriteString("## 👥 成员概览\n\n")
		builder.WriteString("| 成员 | 完成 | 进行中 | 逾期 | 负责需求 | 本期新增 | 已完成 |\n| --- | --- | --- | --- | --- | --- | --- |\n")
		for _, member := range members {
			builder.WriteString("| " + cellText(member.Member.DisplayName) +
				" | " + itoa(member.Tasks.Completed) +
				" | " + itoa(member.Tasks.Active) +
				" | " + itoa(member.Tasks.Overdue) +
				" | " + memberRequirementsCell(member.Requirements, func(metrics RequirementMetrics) int { return metrics.Total }) +
				" | " + memberRequirementsCell(member.Requirements, func(metrics RequirementMetrics) int { return metrics.CreatedInPeriod }) +
				" | " + memberRequirementsCell(member.Requirements, func(metrics RequirementMetrics) int { return metrics.Done }) + " |\n")
		}
	case focusDesign:
		// 设计团队：成员概览把缺陷解决列换成 UI 缺陷处理事实。
		builder.WriteString("## 👥 成员概览\n\n")
		builder.WriteString("| 成员 | 完成 | 进行中 | 逾期 | 解决 UI 缺陷 | 待处理 | 逾期 UI 缺陷 |\n| --- | --- | --- | --- | --- | --- | --- |\n")
		for _, member := range members {
			builder.WriteString("| " + cellText(member.Member.DisplayName) +
				" | " + itoa(member.Tasks.Completed) +
				" | " + itoa(member.Tasks.Active) +
				" | " + itoa(member.Tasks.Overdue) +
				" | " + memberUIBugsCell(member.UIBugs, func(metrics UIBugMetrics) int { return metrics.Resolved }) +
				" | " + memberUIBugsCell(member.UIBugs, func(metrics UIBugMetrics) int { return metrics.Open }) +
				" | " + memberUIBugsCell(member.UIBugs, func(metrics UIBugMetrics) int { return metrics.Overdue }) + " |\n")
		}
	default:
		builder.WriteString("## 👥 成员概览\n\n")
		builder.WriteString("| 成员 | 完成 | 进行中 | 逾期 | 解决缺陷 | 待处理缺陷 |\n| --- | --- | --- | --- | --- | --- |\n")
		for _, member := range members {
			builder.WriteString("| " + cellText(member.Member.DisplayName) +
				" | " + itoa(member.Tasks.Completed) +
				" | " + itoa(member.Tasks.Active) +
				" | " + itoa(member.Tasks.Overdue) +
				" | " + itoa(member.Bugs.Resolved) +
				" | " + itoa(member.Bugs.Open) + " |\n")
		}
	}
	builder.WriteString(truncationNote(len(report.Members), len(members)))
	builder.WriteString("\n")
}

// memberBugReportsCell renders one bug-report metric; a nil metrics block
// (未接入的旧数据) renders as zero instead of breaking the table shape.
func memberBugReportsCell(metrics *BugReportMetrics, pick func(BugReportMetrics) int) string {
	if metrics == nil {
		return "0"
	}
	return itoa(pick(*metrics))
}

// memberRequirementsCell is the requirement-side counterpart of
// memberBugReportsCell.
func memberRequirementsCell(metrics *RequirementMetrics, pick func(RequirementMetrics) int) string {
	if metrics == nil {
		return "0"
	}
	return itoa(pick(*metrics))
}

// memberUIBugsCell is the UI-defect-side counterpart: a nil metrics block
// renders as zero instead of breaking the table shape.
func memberUIBugsCell(metrics *UIBugMetrics, pick func(UIBugMetrics) int) string {
	if metrics == nil {
		return "0"
	}
	return itoa(pick(*metrics))
}

func writeNarrative(builder *strings.Builder, summary string) {
	trimmed := strings.TrimSpace(summary)
	builder.WriteString("## 🤖 AI 评述\n\n")
	if trimmed == "" {
		builder.WriteString(exportMarkdownEmpty + "\n")
		return
	}
	builder.WriteString(trimmed + "\n")
}

// itemLabel prefixes the business key when present so rows stay traceable.
func itemLabel(key, title string) string {
	key = strings.TrimSpace(key)
	title = strings.TrimSpace(title)
	if key == "" {
		return title
	}
	return key + " " + title
}

// cellText protects the markdown table syntax: pipes and line breaks inside
// a cell would break the writer's table parser.
func cellText(value string) string {
	value = strings.ReplaceAll(value, "|", "／")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

// truncationNote explains partial display so readers never mistake the shown
// rows for the whole set.
func truncationNote(total, shown int) string {
	if total <= shown || total == 0 {
		return ""
	}
	return fmt.Sprintf("\n（共 %d 条，仅展示 %d 条）\n", total, shown)
}

// momentDate renders an optional timestamp as a plain calendar date; a nil
// moment renders as an em dash so table cells never collapse to empty.
func momentDate(moment *time.Time) string {
	if moment == nil {
		return "—"
	}
	return moment.Format(anchorDateLayout)
}

func itoa(value int) string {
	return fmt.Sprintf("%d", value)
}
