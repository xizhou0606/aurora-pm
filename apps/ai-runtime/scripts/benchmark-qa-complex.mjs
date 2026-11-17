import { createHmac } from 'node:crypto';
import { pathToFileURL } from 'node:url';

const domains = [
  ['发布审批', '高风险发布必须由项目负责人和质量负责人双人审批；任一拒绝都不得进入发布队列。'],
  ['角色权限', '仅发布经理可提交发布，开发和访客只能查看；角色变更后旧会话权限必须失效。'],
  ['版本冻结', '冻结后的版本禁止新增范围，解冻必须保留审计记录且不能绕过审批。'],
  ['依赖校验', '存在未完成前置任务或阻塞依赖时禁止发布，依赖解除后才可继续。'],
  ['灰度策略', '灰度比例仅允许 1 到 100，扩量必须按阶段执行，失败时停止后续扩量。'],
  ['定时发布', '定时发布时间必须晚于当前时间，跨时区按工作区时区换算且只触发一次。'],
  ['并发提交', '同一版本重复或并发提交只能生成一个发布单，其余请求返回已有结果。'],
  ['回滚控制', '发布失败可回滚至上一稳定版本；回滚失败必须阻断继续发布并告警。'],
  ['制品一致性', '发布前校验制品摘要、版本号和目标环境一致，任一不匹配都应拒绝。'],
  ['环境门禁', '生产环境必须通过测试、变更窗口和容量门禁，测试环境可跳过变更窗口。'],
  ['通知审计', '审批、发布、失败和回滚事件都必须记录操作者、时间、旧值和新值。'],
  ['状态机', '发布单仅允许草稿到审批中、审批中到排队、排队到执行中以及终态迁移。'],
  ['恢复重试', '外部部署服务超时可按幂等键重试三次，未知结果必须查询后再决定是否重试。'],
];

const expectedSkill = Object.freeze({
  commit: 'd75606531f6ce414bbcd95fb203033ae86653852',
  packageSha256: '07e9002dfe456988164f237f4cc33acce1a203564d1c778e94b3fe2bbc43e935',
  mode: 'advanced',
});

export function benchmarkInput() {
  const requirements = domains.map(([title, rule], index) => ({
    id: 1001 + index,
    title,
    description: `${rule} 本需求与审批、状态机、审计及失败恢复规则存在联动，需要覆盖正常、异常、边界和跨模块场景。`,
    acceptance_criteria: [
      `满足规则时 ${title} 流程产生唯一且可审计的成功结果`,
      `违反规则时 ${title} 被明确拒绝且状态与数据不发生错误推进`,
      `并发、超时或边界输入下 ${title} 的结果可观察且可判定`,
    ],
  }));
  const tasks = domains.map(([title], index) => ({
    id: 2001 + index,
    requirement_id: 1001 + index,
    title: `实现并验证${title}`,
    description: `完成${title}的接口、状态校验、幂等处理、审计记录和自动化验证，并验证与其他发布规则的交互。`,
    status: index % 3 === 0 ? 'in_progress' : 'todo',
  }));
  return {
    subject: {
      id: 9001,
      type: 'plan_unit',
      title: '企业级多环境发布编排与失败恢复',
      description: '该版本把审批、权限、冻结、依赖、灰度、定时、幂等、回滚、制品、环境门禁、审计、状态机和重试串成同一发布旅程；跨需求冲突和交互必须显式分析。',
    },
    sources: [{
      url: 'https://example.feishu.cn/docx/release-policy',
      kind: 'feishu_link',
      available: true,
      revision: 'benchmark-v1',
      content_hash: 'benchmark-content-v1',
      content: '统一发布规则：生产发布先校验权限、冻结、依赖、制品和环境门禁，再审批、排队、执行、审计；失败后停止扩量并按幂等键回滚或查询恢复。',
    }],
    test_case_context: {
      coverageRequired: true,
      requirements,
      tasks,
      counts: { requirements: requirements.length, tasks: tasks.length },
    },
  };
}

function coveredIndexes(cases, collection, count) {
  const evidence = cases.flatMap((item) => Array.isArray(item?.evidence) ? item.evidence : []);
  return Array.from({ length: count }, (_, index) => evidence.some((ref) => (
    typeof ref === 'string' && (
      ref.startsWith(`test_case_context.${collection}.${index}.`)
        || ref.startsWith(`test_case_context.${collection}[${index}].`)
    )
  )));
}

function safeUsage(value) {
  if (!value || typeof value !== 'object') return undefined;
  const metrics = {};
  for (const key of ['inputTokens', 'outputTokens', 'totalTokens']) {
    if (typeof value[key] === 'number' && Number.isFinite(value[key])) metrics[key] = value[key];
  }
  return Object.keys(metrics).length ? metrics : undefined;
}

export function evaluateBenchmarkResponse({ status, decoded, elapsedMs, responseBytes }) {
  const output = decoded?.data?.output;
  const cases = Array.isArray(output?.testCases) ? output.testCases : [];
  const requirementCoverage = coveredIndexes(cases, 'requirements', domains.length);
  const taskCoverage = coveredIndexes(cases, 'tasks', domains.length);
  const provenance = output?.skillProvenance ?? {};
  const failureCodes = [];

  if (!Number.isInteger(status) || status < 200 || status >= 300) failureCodes.push('http_status');
  if (cases.length === 0) failureCodes.push('empty_cases');
  if (!output?.testCaseDesign
    || typeof output.testCaseDesign !== 'object'
    || Array.isArray(output.testCaseDesign)) {
    failureCodes.push('missing_test_case_design');
  }
  if (requirementCoverage.some((covered) => !covered)) failureCodes.push('requirement_coverage');
  if (taskCoverage.some((covered) => !covered)) failureCodes.push('task_coverage');
  if (provenance.commit !== expectedSkill.commit) failureCodes.push('skill_commit');
  if (provenance.packageSha256 !== expectedSkill.packageSha256) failureCodes.push('skill_package_sha256');
  if (provenance.mode !== expectedSkill.mode) failureCodes.push('skill_mode');

  return {
    benchmarkPassed: failureCodes.length === 0,
    failureCodes,
    status,
    elapsedMs,
    responseBytes,
    caseCount: cases.length,
    requirementCoverage: requirementCoverage.filter(Boolean).length,
    taskCoverage: taskCoverage.filter(Boolean).length,
    missingRequirementIndexes: requirementCoverage.flatMap((covered, index) => covered ? [] : [index]),
    missingTaskIndexes: taskCoverage.flatMap((covered, index) => covered ? [] : [index]),
    skill: {
      commitMatched: provenance.commit === expectedSkill.commit,
      packageSha256Matched: provenance.packageSha256 === expectedSkill.packageSha256,
      modeMatched: provenance.mode === expectedSkill.mode,
    },
    usage: safeUsage(decoded?.data?.usage),
  };
}

async function main() {
  if (!process.env.AI_INTERNAL_SECRET) throw new Error('AI_INTERNAL_SECRET is required');
  const input = benchmarkInput();
  const runId = Number(process.env.QA_BENCH_RUN_ID ?? 990020);
  const endpoint = process.env.QA_BENCH_ENDPOINT
    ?? 'http://127.0.0.1:18091/internal/v1/generate';
  const body = JSON.stringify({ runId, workspaceId: 2, capability: 'qa.test_cases', input });
  const timestamp = String(Date.now());
  const signature = createHmac('sha256', process.env.AI_INTERNAL_SECRET)
    .update(`${timestamp}.${body}`)
    .digest('hex');
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 370_000);
  const startedAt = Date.now();
  try {
    const response = await fetch(endpoint, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'x-pm-timestamp': timestamp,
        'x-pm-signature': signature,
      },
      body,
      signal: controller.signal,
    });
    const responseText = await response.text();
    let decoded = null;
    try { decoded = JSON.parse(responseText); } catch {}
    const report = evaluateBenchmarkResponse({
      status: response.status,
      decoded,
      elapsedMs: Date.now() - startedAt,
      responseBytes: Buffer.byteLength(responseText),
    });
    console.log(JSON.stringify(report, null, 2));
    if (!report.benchmarkPassed) process.exitCode = 1;
  } finally {
    clearTimeout(timer);
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch(() => {
    console.log(JSON.stringify({
      benchmarkPassed: false,
      status: 'transport_error',
      failureCodes: ['transport_error'],
    }, null, 2));
    process.exitCode = 1;
  });
}
