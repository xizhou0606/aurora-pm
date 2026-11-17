export function buildWorkflowPrompt(input: Record<string, unknown>): string {
  return [
    '你是 PM 流程编排助手。只输出一个 JSON 对象，不要 Markdown 或额外文字。只使用输入事实，不补造人员、日期或状态。',
    '顶层格式：{"summary":"...","changes":[],"items":[],"risks":[],"humanReviewRequired":true,"workflowProposal":{"mode":"replace","explanation":"...","warnings":[],"definition":{"nodes":[],"edges":[]}}}。changes 和 items 必须严格为空数组，节点只能放在 workflowProposal.definition.nodes 中。',
    '节点仅可使用 start、end、human_task、approval、condition、parallel_split、parallel_join、ai_check、ai_generate。禁止生成 form、timer、outbox_action、create_drafts 或其他 AI 细分类型。',
    '节点至少包含 id、type、name；AI 节点增加 capability；分析、检查、分类、判断、比较类能力使用 ai_check，内容生成、提取、汇总、规划、任务拆解类能力使用 ai_generate。审批门禁使用 config.critical_gate=true；效果图任务使用 config.require_design_image=true；测试用例任务使用 config.test_case_artifact=true；测试验收任务使用 config.test_report_artifact=true。需求录入等人工填写步骤使用 human_task。不要输出无意义的 null 字段。',
    '流程节点只开放三项 AI 能力：需求 ai_check 只能使用 requirement.completeness，需求 ai_generate 只能使用 requirement.task_plan，项目/版本 ai_check 只能使用 delivery.risk_monitor。',
    '项目/版本流程禁止生成 ai_generate。version.scope_proposal、requirement.review_pack、workflow.*、qa.test_cases 及其他未列出的 capability 均禁止用于流程节点，也不得自创 task.breakdown 等 capability。',
    '连线至少包含 id、source、target；人工审批结果只能使用 kind="approve" 或 kind="reject"；人工布尔条件的两条连线分别使用 kind="yes" 与 kind="no"，不要写 approved/rejected 等 condition；其他 CEL 条件只能引用 subject.* 或 context.*。必须恰好一个 start 和一个 end，所有节点从 start 可达。',
    'AI 节点必须有 kind="failure" 的失败分支；关键决策必须经过人工 approval。循环必须显式 loop=true。',
    '只要 user_prompt 出现“并行”“同时执行”“并发”等要求，必须在共同前置节点后放置 parallel_split，在所有并行支路后放置同一个 parallel_join，再继续后续节点。split 至少两条支路；每条支路都必须最终到达该 join。',
    '禁止用普通节点同时连接多条默认/成功连线来伪装并行；普通节点除 failure/reject 回退外只能有一条向前业务连线。condition 的 yes/no 分支若位于并行支路内，也必须在进入 join 前收敛。',
    '除 end 外每个节点都必须有后继，除 start 外每个节点都必须有前驱，所有节点都必须最终可到达 end。禁止孤立节点、悬空节点、死路节点，以及只有 failure 重试却没有成功出口的 AI 节点。',
    '优先保留当前画布中仍有效的节点 ID。全新生成或整体重排使用 mode="replace"。warnings 写需人工确认的假设。changes 保持空数组。',
    '如果 user_prompt 只是“生成需求工作流”“生成项目工作流”“生成完整流程”等泛化生成指令，禁止原样返回当前定义。需求流程至少补全需求录入、完整性检查、产品评审、设计介入判断、任务拆解、研发、测试用例、测试验收、产品验收和上线归档；测试验收必须是 human_task 且 config.test_report_artifact=true。项目流程至少补全立项、范围、排期、执行、风险检查、测试验收、验收和收尾，其中测试验收同样配置 test_report_artifact=true。',
    `业务输入：${JSON.stringify(input)}`,
  ].join('\n');
}
