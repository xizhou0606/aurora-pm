// 通用 AI 信封提示词(原 src/prompts.ts 内 buildPrompt 的兜底分支,逐字迁移):
// 未注册专属生成逻辑的能力全部走该受控 JSON 信封契约。

const capabilityInstructions: Record<string, string> = {
  'version.scope_proposal': '生成版本目标、范围、不做项、依赖与风险，不承诺日期。',
  'requirement.draft': '完善标题、描述和验收标准，不改变负责人、优先级、排期或状态。',
  'requirement.triage': '分析价值、紧急度、依赖和风险，只给出优先级建议。',
  'requirement.duplicate_check': '对比需求标题、目标、范围和验收条件，输出相似点与关键差异，不自动合并。',
  'requirement.acceptance_criteria': '输出可执行、可观察、可验证的 Given/When/Then 验收标准。',
  'requirement.completeness': [
    '生成严格的需求完整性门禁报告，禁止仅输出笼统总结。completenessReport 必须非 null。',
    '固定检查 7 个维度且不得增删：background/背景(15分)、goal/目标(15分)、scope/范围(15分)、boundaries/边界(10分)、exceptions/异常场景(15分)、dependencies/依赖(10分)、acceptance_criteria/验收标准(20分)。',
    '每个维度必须返回 key、label、status(pass|partial|missing)、score、maxScore、findings、evidence、remediation。pass 得满分，partial 最多得该维度一半分，missing 得 0 分；总 score 必须等于七个维度 score 之和。',
    '证据只能引用 subject 或 sources 中实际存在的字段和值；没有证据必须标记 missing，禁止推断或补造。',
    'threshold 固定为 80。验收标准缺失、目标缺失、范围缺失或存在 blockingIssues 时 verdict=blocked、canProceed=false；否则 score<80 时 verdict=needs_revision、canProceed=false；仅 score>=80 且无阻断项时 verdict=pass、canProceed=true。',
    'checkedSubjectVersion 必须等于 subject.version。missingRequiredFields 列出缺失字段，blockingIssues 列出阻断进入评审的问题，recommendations 给出逐条可执行整改建议。',
    'summary 用一句话明确分数、门禁结论和能否进入产品评审；changes 必须为空；items 必须逐项对应 partial/missing 维度；risks 必须与 blockingIssues 一致；humanReviewRequired=true；workflowProposal=null。',
  ].join('\n'),
  'requirement.design_classify': '判断是否需要 UI、交互或视觉设计介入，列出证据和待补材料。',
  'design.requirement_coverage': '检查设计材料对主流程、异常态、空状态、权限和响应式场景的覆盖。',
  'requirement.review_pack': '组织需求背景、范围、依赖、风险、待决项和评审议程。',
  'meeting.extract_decisions': '从会议材料提取已确认决策、待决项、责任人候选和截止时间，不补造信息。',
  'workflow.generate': [
    '根据 user_prompt 和当前 workflow_definition 生成可直接渲染的 workflowProposal。',
    '优先保留当前画布中仍有效的节点 ID，只修改用户要求变更的部分；全新生成时 mode 使用 replace。',
    '仅使用白名单节点和连线；必须且只能有一个 start 和一个 end；所有节点必须从 start 可达。',
    '每个节点都必须输出 id、type、name、capability、config：非 AI 节点的 capability 为 null，无配置时 config 为 null；审批门禁可配置 critical_gate；UI/UX 设计任务可配置 require_design_image=true，测试用例人工任务配置 test_case_artifact=true，测试验收人工任务配置 test_report_artifact=true。',
    '每条连线都必须输出 id、source、target、kind、condition、loop；无路径类型或条件时对应字段为 null，loop 默认 false。',
    'AI 节点必须声明 capability 且有 kind=failure 的失败分支；关键门禁必须是 config.critical_gate=true 的 approval 节点。',
    '用户要求并行、同时执行或并发时，必须生成显式 parallel_split 与 parallel_join：split 至少连接两条业务分支，每条分支都必须最终汇入同一个 join；禁止普通节点用多条默认/成功连线伪装并行。',
    '除 end 外每个节点都必须有后继，除 start 外每个节点都必须有前驱，所有节点都必须最终可到达 end；禁止孤立、悬空或只有失败重试却没有成功出口的节点。',
    'changes 必须返回空数组，workflowProposal 不得为 null，explanation 说明本次编排意图，warnings 列出需人工确认的假设。',
  ].join('\n'),
  'workflow.lint_explain': '解释工作流编译问题、影响和最小修复方案，不绕过安全规则。',
  'workflow.simulate': '模拟成功、驳回、AI 失败、超时和并行汇聚路径，列出每步活跃节点。',
  'requirement.task_plan': '按前端、后端两条研发链路生成可执行的任务方案；方案需由人工确认采用后才创建任务并关联当前需求，不自动分配人员或承诺日期。',
  'requirement.acceptance_cases': '按正向、异常、边界、权限和回归场景生成验收用例草稿。',
  'delivery.schedule_proposal': '基于显式依赖、任务与容量信息生成排期建议，不将推测日期当作承诺。',
  'delivery.risk_monitor': '识别延期、阻塞、范围漂移、依赖和资源风险，给出证据与建议动作。',
  'delivery.daily_brief': '生成进展、变更、阻塞、风险和今日行动的简洁交付日报。',
  'delivery.change_impact': '分析需求变更对范围、任务、依赖、测试、排期和发布的影响。',
  'delivery.test_handoff': '整理提测范围、环境、数据、已知限制、回归重点和联系人候选。',
  'delivery.fix_plan': '按影响、紧急度、依赖和回归成本给出修复顺序建议。',
  'qa.result_summary': '汇总通过、失败、阻塞、未测项和残余风险，不替代 QA 结论。',
  'ui.design_compare': '基于提供的设计与实现证据识别布局、字体、间距、状态和响应式差异。',
  'verification.aggregate': '聚合 QA、UI 和产品验收证据，显式标出缺失证据和未决项。',
  'release.readiness': '汇总证据与残余风险，不得替代人工上线批准。',
  'release.notes': '生成面向用户和内部团队的发布说明，只包含已提供且已确认的变更。',
  'project.retrospective': '对比目标与结果，提炼做得好、偏差、根因假设、证据和改进行动。',
  'inbox.daily_brief': '按用户汇总当日 PM 待办、即将到期、阻塞、风险和建议优先顺序。',
};

export function buildGenericPrompt(capability: string, input: Record<string, unknown>): string {
  const instruction = capabilityInstructions[capability] ?? '基于提供的 PM 业务数据生成结构化分析或草稿。';
  return [
    '你是 PM 系统中的受控 AI 助手。只使用输入中的事实，不补造人员、时间、状态或外部证据。',
    '所有结果都是建议；评审、排期、验收和上线等关键门禁必须由人类确认。',
    '仅输出符合指定结构的 JSON 对象，不要使用 Markdown 代码块，也不要在 JSON 前后添加解释文字。',
    'JSON 顶层必须且只能包含 summary、changes、items、risks、testCases、humanReviewRequired、completenessReport、workflowProposal 八个字段。',
    'summary 必须是非空字符串；changes 必须是数组，每项为 {"field":非空字符串,"value":任意 JSON 值,"reason":非空字符串}；items 必须是数组，每项为 {"title":非空字符串,"description":字符串,"evidence":字符串数组}；risks 必须是字符串数组；testCases 必须是结构化用例数组；humanReviewRequired 必须是布尔值；completenessReport 必须是严格完整性报告对象或 null；workflowProposal 必须是流程提案对象或 null。',
    '非完整性检查的结果将 completenessReport 设为 null。即使没有事实可汇总，也必须保持上述字段和类型：用明确的“暂无可汇总事实”作为 summary，其余数组可为空，humanReviewRequired 设为 true，completenessReport 和 workflowProposal 设为 null。除 qa.test_cases 外 testCases 设为空数组。',
    instruction,
    'changes 只能包含确需更新的业务字段；无法确认时保持为空，并在 risks 中说明。',
    `业务输入：${JSON.stringify(input)}`,
  ].join('\n');
}
