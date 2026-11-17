package report

// The narrative-format instructions carried inside every reporting run input.
// The AI runtime owns the prompt assembly; this field tells its reporting
// skill which structure and style to produce.
//
// Structures are adapted from MIT/Apache-licensed open-source weekly-report
// skills:
//   - Personal: guaguaguaxia/weekly_report (★3236, the highest-starred weekly
//     report generator on GitHub — markdown 分点叙述 + 结构化填充) combined with
//     the standard weekly-review sections from Wscats/report-generator (MIT).
//   - Group: the team/project template family of the same skills (团队产能、
//     成员贡献、阻塞项、风险集中点), aggregated across the whole role group.
//
// Both are reduced to a one-shot format: the runtime must never ask
// follow-up questions — it generates from the facts in input.subject only.

// 报告可能覆盖上一完整周期（周/月回退）或任意历史区间，因此结构措辞按周期
// 类型区分（今日/本周/本月），并由具体统计区间锚定。

// cadenceWording carries the period-specific section naming so 日报、周报、
// 月报 read natively instead of reusing weekly phrasing everywhere.
type cadenceWording struct {
	// overview/complete/plan 是各节名前缀（如 今日/本周/本月）。
	label   string
	plan    string
	focus   string
	style   string
	summary string
}

func cadenceWordingFor(period Period) cadenceWording {
	switch period.Type {
	case PeriodDaily:
		return cadenceWording{
			label:   "今日",
			plan:    "明日计划",
			focus:   "明日重点",
			summary: "今日概览",
			style:   "日报采用站会口径：概览 1-2 句话讲清当天主线与最大风险，明细只保留关键项，全文 1 分钟内可读完。",
		}
	case PeriodMonthly:
		return cadenceWording{
			label:   "本月",
			plan:    "下月计划",
			focus:   "下月重点",
			summary: "本月概览",
			style:   "月报采用月度总结口径：在明细之上提炼跨周趋势、阶段性结论与下月方向，突出里程碑与量化变化。",
		}
	default:
		return cadenceWording{
			label:   "本周",
			plan:    "下周计划",
			focus:   "下周重点",
			summary: "本周概览",
			style:   "周报采用周度复盘口径：提炼本周节奏与异常，计划落到具体时间节点，全文 2 分钟内可读完。",
		}
	}
}

// memberReportInstructions builds the personal narrative instructions. The
// role focus swaps the overall-comment emphasis and the fact sections:
// testing members are judged on 提出缺陷, product members on 需求数量, and
// both lose every code-related clause because their reports carry no code.
func memberReportInstructions(period Period, focus roleFocus) string {
	word := cadenceWordingFor(period)
	text := "你是专业的个人工作汇报写作助理。请严格基于给定的工作事实（input.subject 中的成员、统计周期、任务与缺陷数据、多维评分），用 Markdown 分点叙述的形式填充生成一篇完整的" +
		"个人工作汇报，不要追问、不要编造任何给定事实之外的信息。" +
		"输出结构（Markdown）：\n" +
		"# {周期类型} · {姓名} · " + period.AnchorDate + "\n" +
		"## 📌 " + word.summary + "（统计区间 " + period.Start.Format(anchorDateLayout) + " 至 " + period.AnchorDate + "）\n（一句话总结本期核心进展与最大风险，突出 1-2 个重点）\n" +
		"## ✅ " + word.label + "完成\n（分点列出本期完成任务与已解决缺陷，带编号和完成时间）\n" +
		"## 🔄 进行中\n（分点列出在办任务、当前状态、截止时间与逾期风险）\n" +
		"## 🚧 阻碍与风险\n（逐条列出逾期任务与未关闭缺陷：影响 → 建议动作；无则写明当前无阻塞项）\n" +
		"## 📅 " + word.plan + "\n（基于在办任务推断，带优先级与时间节点）\n" +
		"## 📊 数据指标\n" + memberMetricsHint(focus) + "\n"
	switch focus {
	case focusTesting:
		text += "overall_comment 是跳出小节结构的综合评语：该成员是测试岗位，input.subject.bug_reports（本期提出缺陷的总数与 P0/P1/P2 分档）是其核心质量贡献证据，" +
			"综合评语应围绕提出缺陷的数量、严重级别分布，以及任务与缺陷协作情况展开 3-5 句综合分析，直接面向管理者阅读，不重复罗列数字。" +
			"input.subject.code 不存在：本汇报不含代码活动，禁止提及、臆测或评价任何代码产出。"
	case focusProduct:
		text += "overall_comment 是跳出小节结构的综合评语：该成员是产品岗位，input.subject.requirements（负责需求总数、本期新增与状态分布）是其核心产出证据，" +
			"综合评语应围绕需求产出规模、推进状态（已完成/验收中/开发中/待评审）以及任务与缺陷协作情况展开 3-5 句综合分析，直接面向管理者阅读，不重复罗列数字。" +
			"input.subject.code 不存在：本汇报不含代码活动，禁止提及、臆测或评价任何代码产出。"
	case focusDesign:
		text += "overall_comment 是跳出小节结构的综合评语：该成员是设计岗位，input.subject.ui_bugs（本期解决的 UI 域缺陷与当前待处理/逾期）是其核心质量证据，" +
			"综合评语应围绕 UI 缺陷处理情况与任务协作展开 3-5 句综合分析，直接面向管理者阅读，不重复罗列数字；数据缺失时如实说明本周期无 UI 域缺陷记录，不得编造。" +
			"input.subject.code 不存在：本汇报不含代码活动，禁止提及、臆测或评价任何代码产出。"
	default:
		text += "overall_comment 是跳出小节结构的综合评语：针对此人的工作情况（任务与缺陷）、代码产出（行数/提交/PR/评审）与代码质量（命中与逐条点评的综合印象）做 3-5 句综合分析，直接面向管理者阅读，不重复罗列数字。" +
			"综合评语先按 input.subject.target.role_names（生效角色）与 job_title（飞书职务）判断岗位职能：仅研发类岗位（前端/后端/全栈/架构等）成员才对代码质量做重点考核；" +
			"非研发类岗位（设计/运营/管理等）成员的代码活动属辅助性质，overall_comment 不得因其代码产出少或缺失而负面评价或压低综合印象；无法判断岗位时按研发类处理。"
	}
	text += "overall_score 是 0-100 的整数综合评分：严格基于 input.subject 的工作事实给出，缺失的事实维度不得抬高或压低分数。" +
		"测试岗以 bug_reports 的数量与严重级别分布为主要依据；产品岗以 requirements 的数量与状态分布为主要依据；设计岗以 ui_bugs 的处理情况为主要依据；其余岗位以任务完成与代码质量为主要依据。"
	text += "个人汇报没有成员排行：dimension_comments 的 tasks/quality/code 三段都输出空字符串。"
	if focus == focusNone {
		text += "若 input.subject.code 存在，请把代码活动（合入请求、直接提交、新增/删除行及其构成、完成评审、评审覆盖）纳入叙述与总结，但不得据此计算综合分、排名或工时；" +
			"代码相关叙述措辞同样按岗位职能区分：非研发类岗位成员的代码叙述采用「少量/协助性代码变更」等辅助性质措辞，不因代码产出少或缺失而负面评价；研发类成员才依据 commit_reviews 与质量命中对代码质量做重点考核；" +
			"若 input.subject.code.commit_reviews 存在，必须新增一节「## 💻 代码质量分析」。commit_reviews 内 ai_verdict/ai_summary 是代码质量审查技能的逐条结论、code.quality_assessment 是其综合评估：有 ai_verdict 的提交必须逐条引用其结论点评（可补充依据），无 ai_verdict 的按 subject+行数有限点评；" +
			"综合评估以 quality_assessment 为骨架展开（分数、共性问题、风险、建议），技能结论与 diff 证据冲突时以证据为准并指出分歧；命中调试残留/TODO/巨型变更（debug_hits/todo_hits/超大 additions）的提交要点名；不得编造样本之外的内容。" +
			"若 input.subject.code.freshness.degraded 为 true，必须在风险一节说明采集状态异常。"
	}
	text += "风格要求：" + word.style + "洞察优先——提炼趋势、异常和行动建议而不是罗列数字；语气专业、面向向上汇报。"
	return text
}

// memberMetricsHint adapts the 数据指标 section hint to the role focus so the
// narrative quotes the role's core facts instead of only task/bug counters.
func memberMetricsHint(focus roleFocus) string {
	switch focus {
	case focusTesting:
		return "（完成/新增/进行中/逾期任务数，解决/关闭/待处理/逾期缺陷数，提出缺陷总数与 P0/P1/P2 分布）"
	case focusProduct:
		return "（完成/新增/进行中/逾期任务数，解决/关闭/待处理/逾期缺陷数，负责需求总数、本期新增与状态分布）"
	case focusDesign:
		return "（完成/新增/进行中/逾期任务数，解决/关闭/待处理/逾期缺陷数，UI 域缺陷解决/待处理/逾期）"
	default:
		return "（完成/新增/进行中/逾期任务数，解决/关闭/待处理/逾期缺陷数）"
	}
}

func groupReportInstructions(period Period, focus roleFocus) string {
	return teamReportInstructions(period, "角色组（用户组）", focus)
}

func departmentReportInstructions(period Period, focus roleFocus) string {
	return teamReportInstructions(period, "部门（飞书组织架构，含子部门全体成员）", focus)
}

func teamReportInstructions(period Period, subjectNoun string, focus roleFocus) string {
	word := cadenceWordingFor(period)
	text := "你是专业的团队汇报写作助理。input.subject 是一个" + subjectNoun + "整组的聚合工作事实，含全体成员列表与任务/缺陷聚合数据。请严格基于给定事实，用 Markdown 分点叙述生成一篇" +
		"团队汇报，不要追问、不要编造任何给定事实之外的信息。团队汇报与个人汇报不同：站在团队负责人视角，关注产能分布、成员贡献与风险集中点。" +
		"输出结构（Markdown）：\n" +
		"# {周期类型} · {团队名} · " + period.AnchorDate + "\n" +
		"## 📋 " + word.summary + "（统计区间 " + period.Start.Format(anchorDateLayout) + " 至 " + period.AnchorDate + "）\n（团队规模、本期整体进展一段话总结：完成/进行/逾期总量）\n" +
		"## 👥 成员贡献\n" + teamMemberContributionHint(focus) + "\n" +
		"## 🚧 风险与阻塞\n（跨成员聚合：逾期任务与未关闭缺陷按优先级归并，指出影响面与建议的协调动作）\n" +
		"## 🎯 " + word.focus + "\n（基于全组在办任务给出 3-5 条团队级重点，带时间节点）\n" +
		"## 📊 " + word.label + "数据指标\n" + teamMetricsHint(focus) + "\n"
	switch focus {
	case focusTesting:
		text += "input.subject.code 不存在：本汇报不含代码活动，禁止提及、臆测或评价任何代码产出。"
	case focusProduct:
		text += "input.subject.code 不存在：本汇报不含代码活动，禁止提及、臆测或评价任何代码产出。"
	case focusDesign:
		text += "这是设计团队：成员的核心质量证据是 UI 域缺陷处理。成员列表里每人 ui_bugs 是其本期解决/待处理/逾期的 UI 缺陷；成员贡献与质量小节应结合 UI 缺陷处理情况点名领先与需干预的成员，数据缺失时如实说明。" +
			"input.subject.code 不存在：本汇报不含代码活动，禁止提及、臆测或评价任何代码产出。"
	default:
		text += "若 input.subject.code 存在，增加「## 💻 代码质量」小节：按成员汇总提交数与代码命中（调试残留/TODO，来自 commit_reviews 的 debug_hits/todo_hits），点名质量风险集中的成员与代表性提交；无 commit_reviews 时只按 code 聚合指标点评。"
	}
	text += teamDimensionCommentsInstruction(focus)
	text += "overall_score 是 0-100 的整数综合评分：基于团队聚合事实给出，缺失的事实维度不得抬高或压低分数；测试团队以全组 bug_reports、产品团队以全组 requirements、设计团队以全组 ui_bugs 为主要依据之一。"
	text += "风格要求：" + word.style + "洞察优先——识别负载不均、风险集中的成员与任务；语气专业、面向团队负责人向上汇报。"
	return text
}

func teamMemberContributionHint(focus roleFocus) string {
	switch focus {
	case focusTesting:
		return "（每人一行：姓名 — 完成 N 项、进行中 N 项、逾期 N 项、提出缺陷 N 个；对提出缺陷数量与严重级别分布突出的成员点名列出代表性发现）"
	case focusProduct:
		return "（每人一行：姓名 — 完成 N 项、进行中 N 项、逾期 N 项、负责需求 N 个；对需求产出规模与推进状态突出的成员点名列出代表性需求）"
	case focusDesign:
		return "（每人一行：姓名 — 完成 N 项、进行中 N 项、逾期 N 项、解决 UI 缺陷 N 个；对 UI 缺陷处理及时、存量控制突出的成员点名）"
	default:
		return "（每人一行：姓名 — 完成 N 项、进行中 N 项、逾期 N 项；对突出贡献者点名列出代表性产出）"
	}
}

func teamMetricsHint(focus roleFocus) string {
	switch focus {
	case focusTesting:
		return "（全组完成/新增/进行中/逾期任务数，解决/关闭/待处理/逾期缺陷数，提出缺陷总数与 P0/P1/P2 分布）"
	case focusProduct:
		return "（全组完成/新增/进行中/逾期任务数，解决/关闭/待处理/逾期缺陷数，负责需求总数、本期新增与状态分布）"
	case focusDesign:
		return "（全组完成/新增/进行中/逾期任务数，解决/关闭/待处理/逾期缺陷数，UI 域缺陷解决/待处理/逾期）"
	default:
		return "（全组完成/新增/进行中/逾期任务数，解决/关闭/待处理/逾期缺陷数）"
	}
}

func teamDimensionCommentsInstruction(focus roleFocus) string {
	switch focus {
	case focusTesting:
		return "输出 JSON 的 dimension_comments 必须给出 tasks/quality/code 三段维度短评（每段 2-4 句）：tasks 结合成员任务完成与逾期情况点名领先与需干预的成员；quality 结合缺陷解决与逾期风险；code 段在测试团队输出「测试贡献」短评——结合各成员 bug_reports 的提出缺陷数量与严重级别分布。短评面向管理者给判断与建议动作，不逐条罗列数字。"
	case focusProduct:
		return "输出 JSON 的 dimension_comments 必须给出 tasks/quality/code 三段维度短评（每段 2-4 句）：tasks 结合成员任务完成与逾期情况点名领先与需干预的成员；quality 结合缺陷解决与逾期风险；code 段在产品团队输出「需求贡献」短评——结合各成员 requirements 的数量与状态分布。短评面向管理者给判断与建议动作，不逐条罗列数字。"
	case focusDesign:
		return "输出 JSON 的 dimension_comments 必须给出 tasks/quality/code 三段维度短评（每段 2-4 句）：tasks 结合成员任务完成与逾期情况点名领先与需干预的成员；quality 结合缺陷解决与逾期风险；code 段在设计团队输出「设计质量」短评——结合各成员 ui_bugs 的 UI 缺陷解决与存量情况。短评面向管理者给判断与建议动作，不逐条罗列数字。"
	default:
		return "输出 JSON 的 dimension_comments 必须给出 tasks/quality/code 三段维度短评（每段 2-4 句）：tasks 结合成员任务完成与逾期情况点名领先与需干预的成员；quality 结合缺陷解决与逾期风险；code 结合提交与代码命中情况（无代码事实时写明本周期无代码数据）。短评面向管理者给判断与建议动作，不逐条罗列数字。"
	}
}

func reportInstructionsFor(report Report) string {
	focus := reportFocusFor(report.Target)
	switch report.Target.Kind {
	case TargetGroup:
		return groupReportInstructions(report.Period, focus)
	case TargetDepartment:
		return departmentReportInstructions(report.Period, focus)
	default:
		return memberReportInstructions(report.Period, focus)
	}
}

func reportRunPayload(capability string, workspaceID uint, memberReport Report) map[string]any {
	return map[string]any{
		"capability":   capability,
		"workspace_id": workspaceID,
		"subject":      memberReport,
		"instructions": reportInstructionsFor(memberReport),
	}
}
