// 报告能力(report.*):PM 侧把按人/按组聚合好的工作事实放在 input.subject,
// 结构化格式要求放在 input.instructions(默认结构见下)。一次性生成,禁止
// 追问;模型只输出 {"summary":...} JSON,由 parseReportOutput 兜底解析。

export const REPORT_REQUEST_TIMEOUT_MS = 120_000;

// 兜底结构仅在 PM 未传 instructions 时使用；PM 侧始终传按周期差异化的
// 结构（今日/本周/本月），因此兜底保持周期中性的“本期/下一期”措辞。
const DEFAULT_REPORT_INSTRUCTIONS = [
  '输出结构（Markdown）：',
  '# {周期类型} · {目标名称} · {日期范围}',
  '## 📌 本期概览（一句话总结核心进展与最大风险）',
  '## ✅ 本期完成（分点列出已完成任务与已解决缺陷，带编号和完成时间）',
  '## 🔄 进行中（分点列出在办任务、当前状态、截止时间与逾期风险）',
  '## 🚧 阻碍与风险（逐条列出逾期任务与未关闭缺陷：影响 → 建议动作；无则写明当前无阻塞项）',
  '## 📅 下一期计划（基于在办任务推断，带优先级与时间节点）',
  '## 📊 数据指标（完成/新增/进行中/逾期任务数，解决/关闭/待处理/逾期缺陷数）',
].join('\n');

/**
 * 日报/周报/月报技能：PM 侧把按人/按组聚合好的工作事实放在 input.subject，
 * 结构化格式要求放在 input.instructions（默认结构见上）。一次性生成，禁止
 * 追问；只输出 {"summary":"<Markdown 汇报全文>"}。
 */
export function buildReportPrompt(input: Record<string, unknown>): string {
  const instructions = typeof input.instructions === 'string' && input.instructions.trim()
    ? input.instructions.trim()
    : DEFAULT_REPORT_INSTRUCTIONS;
  return [
    '你是专业的项目工作汇报写作助理。只使用下方"工作事实"中的数据撰写汇报：禁止编造任何不存在的任务、缺陷、需求、人员或数字，禁止向用户提问。',
    'summary 承载完整汇报正文（Markdown 格式），必须非空。overall_comment 是跳出结构的综合评语：针对此人的工作情况、代码产出与代码质量做 3-5 句综合分析（做了什么、产出如何、代码质量如何、最该改进什么），语气专业、直接面向管理者阅读。',
    'overall_score 是基于工作事实的综合评分（0-100 整数）：严格依据 input.subject 给出，缺失的事实维度不得抬高或压低分数；评分侧重按 input.instructions 中的岗位口径执行（测试岗以提出的缺陷 bug_reports 为主要依据，产品岗以需求数量 requirements 为主要依据，研发岗以任务完成与代码质量为主要依据）。',
    'dimension_comments 是任务、质量、代码贡献三个成员排行各自的 AI 综合短评，每段 2-4 句：结合对应维度的事实点名表现领先与需要干预的成员，给出管理者视角的判断与建议动作，不逐条罗列数字。target.kind 为 group 或 department 时三段都必须非空；target.kind 为 member 时三段都输出空字符串。',
    '汇报使用简体中文；洞察优先——提炼趋势、异常与行动建议，而不是罗列数字；正文控制在 2 分钟内可读完。',
    '输出结构要求：',
    instructions,
    '',
    '工作事实（input.subject）：',
    JSON.stringify(input.subject ?? {}),
  ].join('\n');
}
