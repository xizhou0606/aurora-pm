import {
  chmodSync,
  cpSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { afterEach, describe, expect, it } from 'bun:test';
import {
  buildQATestCaseDraftFinalRepairInstruction,
  buildQATestCaseDraftRepairInstruction,
  buildQATestCaseDraftSkillInvocation,
  buildQATestCaseSkillInvocation,
  buildQATestCaseRepairInstruction,
  invokeQATestCaseCoveragePatchSkill,
  invokeQATestCaseMixedCoverageTargetPatchSkill,
  loadFixedQATestCaseSkill,
  QA_TEST_CASE_MIXED_COVERAGE_TARGET_PATCH_REPLACEMENT_SENTINEL,
  qaTestCaseSkillMetadata,
  requiresAdvancedQATestCaseRules,
} from './qa-test-case-skill.js';
import type { QATestCaseDraft } from '../draft/fixtures/qa-test-case-draft-schema.js';

const sourceRoot = fileURLToPath(new URL('../../../../skills/qa-test-case-design/', import.meta.url));
const sourceLock = fileURLToPath(new URL('../../../../skills/qa-test-case-design.lock.json', import.meta.url));
const temporaryDirectories: string[] = [];

function copiedBundle(): { root: string; lockPath: string } {
  const directory = mkdtempSync(join(tmpdir(), 'pm-qa-skill-'));
  temporaryDirectories.push(directory);
  const root = join(directory, 'qa-test-case-design');
  const lockPath = join(directory, 'qa-test-case-design.lock.json');
  cpSync(sourceRoot, root, { recursive: true, preserveTimestamps: true });
  cpSync(sourceLock, lockPath);
  chmodSync(join(root, 'scripts/validate_testcases.py'), 0o755);
  return { root, lockPath };
}

function coveragePatchDraft(): QATestCaseDraft {
  return {
    summary: '不得泄漏的原 Draft 摘要',
    objective: '不得泄漏的原 Draft 目标',
    scope: ['不得泄漏的原 Draft 范围'],
    exclusions: [],
    assumptions: [],
    informationGaps: [],
    sourceAssessments: [{
      targetRef: 'subject',
      status: 'confirmed',
      summary: '不得泄漏的原来源判断',
      contextRefs: ['subject:title'],
    }],
    priorityLogic: '不得泄漏的原优先级逻辑',
    methods: ['boundary_value'],
    risks: [{
      riskId: 'RISK-PAYMENT-STATE',
      description: '支付状态可能与实际扣款不一致',
      priority: 'P1',
      contextRefs: ['requirement:0:acceptance_criteria'],
      coverageDecision: { status: 'complete' },
    }, {
      riskId: 'RISK-UNRELATED',
      description: '未点名风险正文不得泄漏',
      priority: 'P2',
      contextRefs: ['subject:title'],
      coverageDecision: { status: 'omitted', reason: '原 Draft 的旧原因不得泄漏' },
    }],
    cases: [{
      title: '原 negative 用例正文不得泄漏',
      category: 'negative',
      priority: 'P1',
      contextRefs: [],
      riskLinks: [{
        riskId: 'RISK-PAYMENT-STATE',
        contextRef: 'requirement:0:acceptance_criteria',
      }],
      preconditions: [],
      testData: [],
      actions: [{ step: '提交异常支付', expectedResult: '页面显示明确的拒绝状态' }],
      notes: [],
      smoke: true,
      automationCandidate: true,
      releaseBlocking: true,
    }, {
      title: '原 rollback 用例正文不得泄漏',
      category: 'rollback',
      priority: 'P1',
      contextRefs: [],
      riskLinks: [{
        riskId: 'RISK-PAYMENT-STATE',
        contextRef: 'requirement:0:acceptance_criteria',
      }],
      preconditions: [],
      testData: [],
      actions: [{ step: '触发回滚', expectedResult: '支付记录保留可观察的回滚状态' }],
      notes: [],
      smoke: false,
      automationCandidate: false,
      releaseBlocking: true,
    }],
  };
}

function coveragePatchPromptSkeleton(prompt: string): Record<string, unknown> {
  const marker = '\n{\n  "repairs"';
  const start = prompt.lastIndexOf(marker);
  if (start < 0) throw new Error('coverage patch prompt has no complete skeleton');
  return JSON.parse(prompt.slice(start + 1)) as Record<string, unknown>;
}

function mixedCoverageTargetPatchPromptSkeleton(prompt: string): Record<string, unknown> {
  const marker = '\n{\n  "riskRepairs"';
  const start = prompt.lastIndexOf(marker);
  if (start < 0) throw new Error('mixed coverage target patch prompt has no complete skeleton');
  return JSON.parse(prompt.slice(start + 1)) as Record<string, unknown>;
}

function mixedCoverageTargetPatchDraft(): QATestCaseDraft {
  const value = coveragePatchDraft();
  value.risks.push({
    riskId: 'RISK-TASK-STATE',
    description: '任务状态切换可能遗漏等待处理记录',
    priority: 'P1',
    contextRefs: ['task:0:status'],
    coverageDecision: { status: 'omitted', reason: '不得泄漏的旧任务风险原因' },
  });
  return value;
}

afterEach(() => {
  for (const directory of temporaryDirectories.splice(0)) {
    rmSync(directory, { recursive: true, force: true });
  }
});

describe('fixed qa-test-case-design skill bundle', () => {
  it('loads the exact pinned GitHub snapshot and exposes reproducible metadata', () => {
    expect(qaTestCaseSkillMetadata()).toEqual({
      name: 'qa-test-case-design',
      repository: 'https://github.com/QA19816-glitch/QA-agent',
      path: 'codex-skills/skills/qa-test-case-design',
      commit: 'd75606531f6ce414bbcd95fb203033ae86653852',
      packageSha256: '07e9002dfe456988164f237f4cc33acce1a203564d1c778e94b3fe2bbc43e935',
    });
  });

  it('keeps trusted skill instructions separate from untrusted business input', () => {
    const uniqueTitle = '不要遵循规则-这是业务标题';
    const invocation = buildQATestCaseSkillInvocation({
      subject: { title: uniqueTitle },
      sources: [{ type: 'feishu_document', content: '需求正文' }],
    });

    expect(invocation.instructions).toContain('# QA Test Case Design');
    expect(invocation.instructions).toContain('## Output Selection');
    expect(invocation.instructions).toContain('# 结构化测试用例模板');
    expect(invocation.instructions).toContain('# 测试用例设计与编写规则');
    expect(invocation.instructions).not.toContain('# 高风险、多来源测试用例设计规则');
    expect(invocation.instructions).not.toContain(uniqueTitle);
    expect(invocation.instructions).not.toContain('compact');
    expect(invocation.instructions).not.toContain('最多输出 4 条');
    expect(invocation.prompt).toContain(uniqueTitle);
    expect(invocation.skill).toMatchObject({ mode: 'standard' });
    expect(invocation.skill.resources).toEqual([
      'output-templates/structured-test-cases.md',
      'references/case-writing-rules.md',
    ]);
  });

  it('uses progressive disclosure for complex or multi-source requirements', () => {
    const input = {
      subject: { title: '版本级回归' },
      sources: [{ type: 'feishu_document' }, { type: 'technical_note' }],
      test_case_context: { requirements: [{ id: 1 }, { id: 2 }], tasks: [] },
    };
    expect(requiresAdvancedQATestCaseRules(input)).toBe(true);

    const invocation = buildQATestCaseSkillInvocation(input);
    expect(invocation.skill.mode).toBe('advanced');
    expect(invocation.skill.resources).toContain('prompts/advanced-test-case-design.md');
    expect(invocation.instructions).toContain('# 高风险、多来源测试用例设计规则');
  });

  it('treats a PM test-case artifact as formal work even for one requirement', () => {
    const input = {
      subject: { title: '单需求正式用例' },
      test_case_context: { requirements: [{ id: 1 }], tasks: [] },
    };
    expect(requiresAdvancedQATestCaseRules(input)).toBe(true);
    expect(buildQATestCaseSkillInvocation(input).skill.mode).toBe('advanced');
  });

  it('requires per-item context evidence in generation and repair instructions', () => {
    const invocation = buildQATestCaseSkillInvocation({
      subject: { title: '复杂结算' },
      test_case_context: {
        coverageRequired: true,
        requirements: [{ id: 1 }, { id: 2 }],
        tasks: [{ id: 11 }, { id: 12 }],
      },
    });

    expect(invocation.instructions).toContain('requirements 与 tasks 的每个数组项都必须至少被一条 testCases[].evidence');
    expect(invocation.instructions).toContain('每条用例最多覆盖 8 个不同的 context 数组项');
    expect(invocation.instructions).toContain('禁止一条泛化用例堆满全部 evidence');
    const repair = buildQATestCaseRepairInstruction(['missing coverage']);
    expect(repair).toContain('requirements/tasks 每个数组项都被至少一条用例 evidence');
    expect(repair).toContain('每条用例最多覆盖 8 个不同 context 项');
    expect(repair).toContain('coverageStrategy.methods 只能是 equivalence_partitioning');
    expect(repair).toContain('traceability[].coverageTypes 只能是 positive、negative、boundary');
    expect(repair).toContain('每个 evidence 必须是完整、可解析且真实存在的 JSON 路径');
    expect(repair).toContain('steps.length 必须等于 expectedResults.length');
  });

  it('builds a fixed-Skill Draft IR invocation without mixing in the final artifact host contract', () => {
    const uniqueTitle = '复杂支付需求-仅作为业务事实';
    const input = {
      subject: { title: uniqueTitle },
      sources: [{ type: 'feishu_document', available: true, content: '需求正文' }],
      test_case_context: {
        coverageRequired: true,
        requirements: [{ id: 1 }],
        tasks: [{ id: 11, requirement_id: '1' }],
      },
    };
    const refs = ['subject:title', 'source:0:content', 'requirement:0:title', 'task:0:title'];
    const groupLedger = [
      { targetRef: 'requirement:0', group: 'requirement:0' },
      { targetRef: 'task:0', group: 'requirement:0' },
    ];
    const invocation = buildQATestCaseDraftSkillInvocation(input, refs);

    expect(invocation.skill).toEqual(buildQATestCaseSkillInvocation(input).skill);
    expect(invocation.instructions).toContain('# QA Test Case Design');
    expect(invocation.instructions).toContain('# 结构化测试用例模板');
    expect(invocation.instructions).toContain('# 高风险、多来源测试用例设计规则');
    expect(invocation.instructions).toContain('# 测试用例设计与编写规则');
    expect(invocation.instructions).toContain('FIXED AGENT SKILL: qa-test-case-design@d75606531f6ce414bbcd95fb203033ae86653852');
    expect(invocation.instructions).toContain('package-sha256:07e9002dfe456988164f237f4cc33acce1a203564d1c778e94b3fe2bbc43e935');
    expect(invocation.instructions).toContain('本次模型只输出测试设计 Draft IR');
    expect(invocation.instructions).toContain('最终 caseId、evidence、traceability、执行顺序及其他 artifact 元数据由确定性 compiler');
    expect(invocation.instructions).toContain('structured template 只定义最终 artifact 的测试设计语义');
    expect(invocation.instructions).toContain(JSON.stringify(refs));
    expect(invocation.instructions).toContain(JSON.stringify(['subject', 'source:0', 'requirement:0', 'task:0']));
    expect(invocation.instructions).toContain(JSON.stringify(groupLedger));
    expect(invocation.instructions).not.toContain('顶层必须且只能包含 summary 和 design');
    expect(invocation.instructions).not.toContain(uniqueTitle);
    expect(invocation.prompt).toContain(uniqueTitle);
    expect(invocation.prompt).toContain(JSON.stringify(groupLedger));
    expect(invocation.prompt).toContain('effectiveRefs = case.contextRefs + all valid case.riskLinks[].contextRef');
    expect(invocation.prompt).toContain('关系 leaf 必须直接位于同一条 case.contextRefs');
    expect(invocation.prompt).toContain('riskLinks.contextRef 即使是 subject:description 或 source:N:content 也不能充当关系证明');
    expect(invocation.prompt).toContain('首个可用 requirement/task leaf "requirement:0:title"');
    expect(invocation.prompt).toContain('不得把其他 group 的 leaf 堆入这三条骨架 case');
    expect(invocation.prompt).toContain('"contextRef": "requirement:0:title"');
    expect(invocation.prompt).not.toContain('"contextRef": "subject:title"');
    expect(invocation.prompt).toContain('"sourceAssessments": [');
    expect(invocation.prompt).toContain('不是默认判断');
    expect(invocation.prompt).toContain('不能照抄占位值');
  });

  it('front-loads a first-pass P0/P1 core-category checklist so single-requirement drafts avoid coverage patches', () => {
    const input = {
      subject: { title: '单需求文档' },
      sources: [{ content: '提交后应显示明确结果' }],
      test_case_context: {
        mode: 'single_requirement_document',
        coverageRequired: true,
        requirements: [{ id: 146, title: '单需求文档' }],
        tasks: [],
      },
    };
    const refs = ['subject:title', 'source:0:content', 'requirement:0:title'];
    const invocation = buildQATestCaseDraftSkillInvocation(input, refs);

    expect(invocation.prompt).toContain('首轮交付自检（优先于扩展其他 category）');
    expect(invocation.prompt).toContain('每个 P0/P1 risk 都必须已有三条直接 riskLinks');
    expect(invocation.prompt).toContain('category 字面分别为 positive、negative、boundary');
    expect(invocation.prompt).toContain('单需求文档模式');
    expect(invocation.prompt).toContain('不要把核心三类留到后续 coverage patch');
    expect(invocation.instructions).toContain('首轮输出前必须完成 P0/P1 核心三类自检');
    expect(invocation.prompt).toContain('三条骨架 case 是每个 P0/P1 risk 的最低覆盖形状');
    expect(invocation.prompt).toMatch(/业务输入[\s\S]*输出前最后自检/);
    expect(invocation.prompt).toContain('每个 P0/P1 risk 的 positive、negative、boundary 三类必须齐全');
    expect(invocation.prompt).toContain('优先只保留 1 到 3 个最高优先级的 P0/P1 risk');
  });

  it('uses the same target-level coverage checklist for initial and repaired Drafts without pinning a leaf', () => {
    const input = {
      subject: { title: '多事实覆盖' },
      test_case_context: {
        requirements: [
          { id: 7, acceptance_criteria: '失败时不得产生成功记录' },
          { id: 8, description: '支持撤销已提交的申请' },
        ],
        tasks: [
          { requirement_id: 7, status: '进行中' },
          { requirement_id: 'orphan', description: '补充回归验证' },
        ],
      },
    };
    const refs = [
      'subject:title',
      'requirement:0:acceptance_criteria',
      'requirement:1:description',
      'task:0:status',
      'task:1:description',
    ];
    const requiredCoverageTargets = ['requirement:0', 'requirement:1', 'task:0', 'task:1'];
    const invocation = buildQATestCaseDraftSkillInvocation(input, refs);
    const repair = buildQATestCaseDraftRepairInstruction(['context item is not covered: task:1'], refs, input);
    const finalRepair = buildQATestCaseDraftFinalRepairInstruction(
      ['context item is not covered: task:1'], refs, input,
    );

    for (const content of [invocation.instructions, invocation.prompt, repair, finalRepair]) {
      expect(content).toContain(`HOST requiredCoverageTargets（由 leaf 白名单推导的精确目标清单）：${JSON.stringify(requiredCoverageTargets)}。`);
      expect(content).toContain('effectiveRefs = case.contextRefs + all valid case.riskLinks[].contextRef');
      expect(content).toContain('coveredTargets 必须与 requiredCoverageTargets 完全相等');
      expect(content).toContain('每个 target 可使用它在 leaf 白名单中的任一合法实际 leaf；不强制 title、description、acceptance_criteria 或 status 中的某一个');
    }

    expect(invocation.instructions).toContain(JSON.stringify(refs));
    expect(invocation.prompt).toContain(JSON.stringify(refs));
    expect(invocation.instructions).not.toContain('必须使用 title 作为覆盖锚点');
  });

  it('states the exact Draft IR schema, pair-shaped actions, enums, and compiler relationships', () => {
    const input = {
      subject: { title: '支付回归' },
      test_case_context: {
        requirements: [{ id: 10 }, { id: 20 }],
        tasks: [{ requirement_id: 10 }],
      },
    };
    const invocation = buildQATestCaseDraftSkillInvocation(
      input,
      ['subject:title', 'requirement:0:title', 'requirement:1:acceptance_criteria', 'task:0:status'],
    );

    expect(invocation.instructions).toContain('summary、objective、scope、exclusions、assumptions、informationGaps、sourceAssessments、priorityLogic、methods、risks、cases');
    expect(invocation.instructions).toContain('equivalence_partitioning、boundary_value、decision_table、state_transition、pairwise、user_journey、error_guessing、regression_history');
    expect(invocation.instructions).toContain('positive、negative、boundary、permission、state、duplicate_submit、concurrency、timeout、retry、rollback、stale_data、integration、regression');
    expect(invocation.instructions).toContain('P0、P1、P2、P3');
    expect(invocation.instructions).toContain('^RISK-[A-Z0-9](?:[A-Z0-9-]{0,93}[A-Z0-9])?$');
    expect(invocation.instructions).toContain('同时包含非空 step 和非空 expectedResult');
    expect(invocation.instructions).toContain('这些引用和 riskLinks 只供 compiler 建立 evidence/trace');
    expect(invocation.instructions).toContain('compiler 不会自动补齐');
    expect(invocation.instructions).toContain('HOST requiredCoverageTargets（由 leaf 白名单推导的精确目标清单）');
    expect(invocation.instructions).toContain('coveredTargets 必须与 requiredCoverageTargets 完全相等');
    expect(invocation.instructions).toContain('subject:title|description、source:N:content、requirement:N:title|description|acceptance_criteria、task:N:title|description|status');
    expect(invocation.instructions).toContain('粗粒度对象引用及 URL、ID、key、updated_at 一律禁止');
    expect(invocation.instructions).toContain('从每条 effectiveRefs 提取 requirement:N/task:N targetRef 后跨 case 去重');
    expect(invocation.instructions).toContain('每条 case 最多关联 8 个不同 requirement/task 对象');
    expect(invocation.instructions).toContain('link.contextRef 必须逐字存在于对应 risk.contextRefs');
    expect(invocation.instructions).toContain('它不必在 case.contextRefs 重复');
    expect(invocation.instructions).toContain('不会猜测、替换或补入其他 leaf');
    expect(invocation.instructions).toContain('至少一条 case 的 smoke=true');
    expect(invocation.instructions).toContain('每个 targetRef 恰好输出一项');
    expect(invocation.instructions).toContain('conflict 至少引用两个不同 targetRef 的 leaf 且必须包含自身 targetRef');
    expect(invocation.instructions).toContain('不得因输入字段非空就默认 confirmed');
    expect(invocation.instructions).toContain('普通 case 只能覆盖一个 requirement group');
    expect(invocation.instructions).toContain('effectiveRefs = case.contextRefs + all valid case.riskLinks[].contextRef');
    expect(invocation.instructions).toContain('不得把不同 group 分别藏在两处');
    expect(invocation.instructions).toContain('同一条 case 同时满足三个条件');
    expect(invocation.instructions).toContain('该 case.category=integration；顶层 methods 包含 user_journey');
    expect(invocation.instructions).toContain('该 case.contextRefs 直接包含字面 subject:description 或 source:N:content');
    expect(invocation.instructions).toContain('即使出现在 riskLinks.contextRef');
    expect(invocation.instructions).toContain('不得为了消除意外跨组而把 P0/P1 risk 的 positive、negative、boundary 核心覆盖 case 改成 integration');
    expect(invocation.instructions).toContain('P0/P1 risk 的 positive、negative、boundary 缺类由 compiler');
    expect(invocation.instructions).toContain('模型不得枚举缺类');
    expect(invocation.instructions).toContain('{"status":"complete"}');
    expect(invocation.instructions).toContain('{"status":"omitted","reason":"基于输入事实的非空原因"}');
    expect(invocation.instructions).toContain('P2/P3 risk 必须且只能是 {"status":"not_required"}');
    expect(invocation.instructions).toContain('不会默认补原因或改写决定');
    expect(invocation.instructions).toContain('cases 数组顺序就是 compiler 生成的推荐 executionOrder');
    expect(invocation.prompt).toContain('"actions": [');
    expect(invocation.prompt).toContain('"step": "执行动作"');
    expect(invocation.prompt).toContain('"coverageDecision": {');
    expect(invocation.prompt).toContain('"riskLinks": [');
    expect(invocation.prompt).not.toContain('"coverageOmissions":');
    expect(invocation.prompt).not.toContain('"riskIds":');
    expect(invocation.prompt).toContain('"expectedResult": "页面显示可观察且可判定的业务状态或字段值"');
  });

  it('builds a Draft-specific repair instruction with exact ref and coverage checks', () => {
    const input = {
      test_case_context: {
        requirements: [{ id: 7 }],
        tasks: [{ requirement_id: '7' }, { requirement_id: 'orphan' }],
      },
    };
    const refs = [
      'subject:description',
      'source:0:content',
      'requirement:0:title',
      'requirement:0:acceptance_criteria',
      'task:0:description',
      'task:1:status',
    ];
    const groupLedger = [
      { targetRef: 'requirement:0', group: 'requirement:0' },
      { targetRef: 'task:0', group: 'requirement:0' },
      { targetRef: 'task:1', group: 'orphan-requirement:0' },
    ];
    const repair = buildQATestCaseDraftRepairInstruction(['unknown context ref'], refs, input);

    expect(repair).toContain(JSON.stringify(refs));
    expect(repair).toContain(JSON.stringify(['requirement:0', 'task:0', 'task:1']));
    expect(repair).toContain('粗粒度对象引用及 URL、ID、key、updated_at 均非法');
    expect(repair).toContain('methods 只能是 equivalence_partitioning');
    expect(repair).toContain('category 只能是 positive');
    expect(repair).toContain('所有 priority 只能是 P0、P1、P2、P3');
    expect(repair).toContain('每条 case 的 riskLinks 非空且都指向已声明 riskId');
    expect(repair).toContain('link.contextRef 必须逐字存在于对应 risk.contextRefs');
    expect(repair).toContain('link.contextRef 不必在 case.contextRefs 重复');
    expect(repair).toContain('绝不会猜测或替换 leaf');
    expect(repair).toContain('每条 case 最多关联 8 个不同 requirement/task 对象');
    expect(repair).toContain('methods、riskId、case title、每个 contextRefs 数组，以及同一 case 的 riskLinks.riskId 均不得重复');
    expect(repair).toContain('至少一条 case 的 smoke=true');
    expect(repair).toContain('每个 action 同时且只能包含非空 step 与非空 expectedResult');
    expect(repair).toContain('最终 artifact 仍由确定性 compiler 组装');
    expect(repair).toContain('sourceAssessments');
    expect(repair).toContain('不得按“非空即 confirmed”推断');
    expect(repair).toContain('普通 case 只能覆盖一个 requirement');
    expect(repair).toContain('category=integration');
    expect(repair).toContain(JSON.stringify(groupLedger));
    expect(repair).toContain('effectiveRefs = case.contextRefs + all valid case.riskLinks[].contextRef');
    expect(repair).toContain('不得把不同 group 分别放进两处规避');
    expect(repair).toContain('意外跨组必须拆成按 group 聚焦的 focused cases');
    expect(repair).toContain('不得把 P0/P1 risk 的 positive、negative、boundary 核心 case 改成 integration');
    expect(repair).toContain('同一条 case 必须同时满足 category=integration、顶层 methods 包含 user_journey');
    expect(repair).toContain('case.contextRefs 直接含字面 subject:description 或 source:N:content');
    expect(repair).toContain('riskLinks.contextRef、其他 case/risk/sourceAssessment 的关系引用不计');
    expect(repair).toContain('P0/P1 risk');
    expect(repair).toContain('positive、negative、boundary');
    expect(repair).toContain('三类齐全时 coverageDecision 只能是 {"status":"complete"}');
    expect(repair).toContain('只要缺一类就只能是 {"status":"omitted"');
    expect(repair).toContain('模型不要输出缺类数组');
    expect(repair).toContain('P2/P3 risk 只能是 {"status":"not_required"}');
    expect(repair).toContain('界面文案、业务状态、字段值、记录变化或响应结果');
    expect(repair).toContain('禁止只写“系统正常”“符合预期”“成功”“失败”“正确处理”或“提示错误”');
    expect(repair).toContain('cases 当前数组顺序必须已经是推荐 executionOrder');
  });

  it('builds an atomic final Draft repair with separate target and risk coverage audits', () => {
    const input = {
      test_case_context: {
        requirements: [{ id: 7 }],
        tasks: [{ requirement_id: '7' }, { requirement_id: 'orphan' }],
      },
    };
    const refs = [
      'subject:description',
      'source:0:content',
      'requirement:0:title',
      'requirement:0:acceptance_criteria',
      'task:0:description',
      'task:1:status',
    ];
    const issues = [
      'context item is not covered: task:1',
      'risk RISK-PAYMENT coverage decision must be omitted for missing categories: boundary',
    ];
    const groupLedger = [
      { targetRef: 'requirement:0', group: 'requirement:0' },
      { targetRef: 'task:0', group: 'requirement:0' },
      { targetRef: 'task:1', group: 'orphan-requirement:0' },
    ];
    const repair = buildQATestCaseDraftFinalRepairInstruction(issues, refs, input);

    expect(repair).toContain('当前 assistant 消息中已经成功解析的 Draft IR 是唯一编辑基线');
    expect(repair).toContain('本轮最新门禁 issues 视为一个不可拆分的原子 checklist');
    expect(repair).toContain(JSON.stringify(issues));
    expect(repair).toContain('不得删除或改坏未被 issue 点名的合法');
    expect(repair).toContain('分别维护两个互不替代的机械账本');
    expect(repair).toContain(JSON.stringify(['requirement:0', 'task:0', 'task:1']));
    expect(repair).toContain('从每条 effectiveRefs 提取 requirement:N/task:N targetRef 后跨 case 去重，得到 coveredTargets');
    expect(repair).toContain('coveredTargets 必须与 requiredCoverageTargets 完全相等');
    expect(repair).toContain('只统计直接 riskLinks.riskId 指向该 risk 的 case.category 字面 positive、negative、boundary');
    expect(repair).toContain('rollback、permission、state、integration 或其他 category 均不能补齐核心类别');
    expect(repair).toContain('从最终候选 Draft 重新逐 target 审计 target coverage、重算 effectiveRefs 与 coveredTargets，并逐 risk 重算');
    expect(repair).toContain('再填写每个 coverageDecision');
    expect(repair).toContain('focused case');
    expect(repair).toContain(JSON.stringify(groupLedger));
    expect(repair).toContain('不得只检查 case.contextRefs 而遗漏 riskLinks.contextRef');
    expect(repair).toContain('禁止把 P0/P1 核心 case 改成 integration');
    expect(repair).toContain('case.contextRefs 直接包含字面 subject:description 或 source:N:content');
    expect(repair).toContain('riskLinks.contextRef 不计关系条件');
    expect(repair).toContain('三个条件不得从不同 case、risk 声明、sourceAssessment 或 sibling 拼凑');
    expect(repair).toContain('expectedResult 必须是具体、可观察且可判定');
    expect(repair).toContain('preconditions、testData 必须是输入事实支持、可复现的具体条件或数据');
    expect(repair).toContain('禁止把“系统已有数据”或“准备测试数据”作为完整项');
    expect(repair).toContain('只输出一份完整且严格符合 Draft IR schema 的 JSON 对象');
    expect(repair).toContain('不得向任何层级增加 audit、diagnostics、issues、actualCategories、missingCategories、expectedStatus、coverageAudit、targetCoverage、uncoveredTargets、coverageOmissions');

    // The final pass retains the complete first-pass repair contract.
    expect(repair).toContain(JSON.stringify(refs));
    expect(repair).toContain('sourceAssessments');
    expect(repair).toContain('每条 case 的 riskLinks 非空且都指向已声明 riskId');
    expect(repair).toContain('至少一条 case 的 smoke=true');
  });

  it('builds a fixed-Skill coverage-only patch with exact linked core categories', () => {
    const input = {
      subject: { title: '支付结算' },
      test_case_context: {
        coverageRequired: true,
        requirements: [{ acceptance_criteria: '支付失败不得生成成功记录' }],
        tasks: [],
      },
    };
    const allowedRefs = ['subject:title', 'requirement:0:acceptance_criteria'];
    const invocation = invokeQATestCaseCoveragePatchSkill({
      input,
      allowedRefs,
      draft: coveragePatchDraft(),
      diagnostics: [{
        code: 'risk_coverage_decision_mismatch',
        riskId: 'RISK-PAYMENT-STATE',
        priority: 'P1',
        actualCategories: ['negative'],
        missingCategories: ['positive', 'boundary'],
        expectedStatus: 'omitted',
        message: 'risk RISK-PAYMENT-STATE coverage decision must be omitted for missing categories: positive, boundary',
      }],
    });

    expect(invocation.skill).toEqual({
      name: 'qa-test-case-design',
      repository: 'https://github.com/QA19816-glitch/QA-agent',
      path: 'codex-skills/skills/qa-test-case-design',
      commit: 'd75606531f6ce414bbcd95fb203033ae86653852',
      packageSha256: '07e9002dfe456988164f237f4cc33acce1a203564d1c778e94b3fe2bbc43e935',
      mode: 'advanced',
      resources: [
        'output-templates/structured-test-cases.md',
        'prompts/advanced-test-case-design.md',
        'references/case-writing-rules.md',
      ],
    });
    expect(invocation.instructions).toContain('FIXED AGENT SKILL: qa-test-case-design@d75606531f6ce414bbcd95fb203033ae86653852');
    expect(invocation.instructions).toContain('package-sha256:07e9002dfe456988164f237f4cc33acce1a203564d1c778e94b3fe2bbc43e935');
    expect(invocation.instructions).toContain('QA COVERAGE-ONLY PATCH ADAPTER');
    expect(invocation.instructions).toContain('每个 diagnostic risk 恰好输出一项');
    expect(invocation.instructions).toContain('riskId、resolutions、omissionReason');
    expect(invocation.instructions).toContain('kind="case"');
    expect(invocation.instructions).toContain('kind="omit"');
    expect(invocation.instructions).toContain('title、step、expectedResult 必须由模型基于业务事实原样给出');
    expect(invocation.instructions).toContain('禁止只写“系统正常”或“符合预期”');
    expect(invocation.instructions).toContain('HOST/程序不会生成、默认、改写或拼接这个原因');
    expect(invocation.instructions).toContain('complete/not_required');
    expect(invocation.instructions).not.toContain('支付状态可能与实际扣款不一致');
    expect(invocation.prompt).toContain(JSON.stringify({
      riskId: 'RISK-PAYMENT-STATE',
      description: '支付状态可能与实际扣款不一致',
      priority: 'P1',
      contextRefs: ['requirement:0:acceptance_criteria'],
      missingCategories: ['positive', 'boundary'],
    }));
    expect(invocation.prompt).toContain(JSON.stringify(input));
    expect(invocation.prompt).toContain(JSON.stringify(allowedRefs));
    expect(invocation.prompt).toContain('"resolutions": [');
    expect(invocation.prompt).toContain('__QA_COVERAGE_PATCH_REPLACE_FROM_INPUT__');
    expect(invocation.prompt).toContain('每个 title、step、expectedResult 都必须具体');
    expect(invocation.prompt).toContain('禁止写“系统正常”或“符合预期”');
    expect(invocation.prompt).toContain('只把该项替换为严格形状');
    expect(invocation.prompt).toContain('omissionReason');
    const skeleton = coveragePatchPromptSkeleton(invocation.prompt) as {
      repairs: Array<{
        riskId: string;
        resolutions: Array<Record<string, unknown>>;
        omissionReason: string | null;
      }>;
    };
    expect(skeleton.repairs).toHaveLength(1);
    expect(skeleton.repairs[0].omissionReason).toBeNull();
    expect(skeleton.repairs[0].resolutions).toHaveLength(2);
    expect(skeleton.repairs[0].resolutions.map((resolution) => resolution.category))
      .toEqual(['positive', 'boundary']);
    expect(skeleton.repairs[0].resolutions[0]).toEqual({
      category: 'positive',
      kind: 'case',
      contextRef: 'requirement:0:acceptance_criteria',
      title: '__QA_COVERAGE_PATCH_REPLACE_FROM_INPUT__',
      step: '__QA_COVERAGE_PATCH_REPLACE_FROM_INPUT__',
      expectedResult: '__QA_COVERAGE_PATCH_REPLACE_FROM_INPUT__',
    });
  });

  it('does not leak unrelated Draft model text into a coverage-only prompt', () => {
    const invocation = invokeQATestCaseCoveragePatchSkill({
      input: {
        subject: { title: '支付结算' },
        test_case_context: {
          requirements: [{ acceptance_criteria: '支付失败不得生成成功记录' }],
          tasks: [],
        },
      },
      allowedRefs: ['subject:title', 'requirement:0:acceptance_criteria'],
      draft: coveragePatchDraft(),
      diagnostics: [{
        code: 'risk_coverage_decision_mismatch',
        riskId: 'RISK-PAYMENT-STATE',
        priority: 'P1',
        actualCategories: ['negative'],
        missingCategories: ['positive', 'boundary'],
        expectedStatus: 'omitted',
        message: 'risk RISK-PAYMENT-STATE coverage decision must be omitted for missing categories: positive, boundary',
      }],
    });

    expect(invocation.prompt).not.toContain('不得泄漏的原 Draft 摘要');
    expect(invocation.prompt).not.toContain('不得泄漏的原 Draft 目标');
    expect(invocation.prompt).not.toContain('不得泄漏的原来源判断');
    expect(invocation.prompt).not.toContain('原 Draft 的旧原因不得泄漏');
    expect(invocation.prompt).not.toContain('原 negative 用例正文不得泄漏');
    expect(invocation.prompt).not.toContain('原 rollback 用例正文不得泄漏');
    expect(invocation.prompt).not.toContain('未点名风险正文不得泄漏');
  });

  it('builds a fixed-Skill mixed risk-and-target compact patch with only eligible paths', () => {
    const input = {
      subject: { title: '支付结算' },
      test_case_context: {
        coverageRequired: true,
        requirements: [{ acceptance_criteria: '支付失败不得生成成功记录' }],
        tasks: [{ status: '待处理' }],
      },
    };
    const invocation = invokeQATestCaseMixedCoverageTargetPatchSkill({
      input,
      allowedRefs: [
        'subject:title',
        'requirement:0:acceptance_criteria',
        'task:0:status',
      ],
      draft: mixedCoverageTargetPatchDraft(),
      riskDiagnostics: [{
        code: 'risk_coverage_decision_mismatch',
        riskId: 'RISK-PAYMENT-STATE',
        priority: 'P1',
        actualCategories: ['negative'],
        missingCategories: ['positive', 'boundary'],
        expectedStatus: 'omitted',
        message: 'risk RISK-PAYMENT-STATE coverage decision must be omitted for missing categories: positive, boundary',
      }],
      targetDiagnostics: [{
        code: 'context_item_not_covered',
        targetRef: 'task:0',
        candidateContextRefs: ['task:0:status'],
        group: 'task:0',
        message: 'context item is not covered: task:0',
      }],
      eligibleTargetCoverageSpecs: [{
        targetRef: 'task:0',
        riskId: 'RISK-TASK-STATE',
        contextRef: 'task:0:status',
      }],
    });

    expect(invocation.skill).toEqual({
      name: 'qa-test-case-design',
      repository: 'https://github.com/QA19816-glitch/QA-agent',
      path: 'codex-skills/skills/qa-test-case-design',
      commit: 'd75606531f6ce414bbcd95fb203033ae86653852',
      packageSha256: '07e9002dfe456988164f237f4cc33acce1a203564d1c778e94b3fe2bbc43e935',
      mode: 'advanced',
      resources: [
        'output-templates/structured-test-cases.md',
        'prompts/advanced-test-case-design.md',
        'references/case-writing-rules.md',
      ],
    });
    expect(invocation.instructions).toContain('FIXED AGENT SKILL: qa-test-case-design@d75606531f6ce414bbcd95fb203033ae86653852');
    expect(invocation.instructions).toContain('QA MIXED COVERAGE-TARGET PATCH ADAPTER');
    expect(invocation.instructions).toContain('riskRepairs、targetRepairs');
    expect(invocation.instructions).toContain('完整二元组逐字选择');
    expect(invocation.instructions).toContain('严禁 positive、negative、boundary 和 integration');
    expect(invocation.instructions).toContain('permission、state、duplicate_submit、concurrency、timeout、retry、rollback、stale_data、regression');
    expect(invocation.instructions).toContain('禁止只写“系统正常”“符合预期”“成功”“失败”“正确处理”或“提示错误”');
    expect(invocation.instructions).not.toContain('不得泄漏的原 Draft 摘要');
    expect(invocation.instructions).not.toContain('原 negative 用例正文不得泄漏');
    expect(invocation.prompt).toContain(JSON.stringify({
      riskId: 'RISK-PAYMENT-STATE',
      description: '支付状态可能与实际扣款不一致',
      priority: 'P1',
      contextRefs: ['requirement:0:acceptance_criteria'],
      missingCategories: ['positive', 'boundary'],
    }));
    expect(invocation.prompt).toContain(JSON.stringify({
      targetRef: 'task:0',
      candidates: [{
        riskId: 'RISK-TASK-STATE',
        contextRef: 'task:0:status',
        riskDescription: '任务状态切换可能遗漏等待处理记录',
        riskPriority: 'P1',
      }],
    }));
    expect(invocation.prompt).toContain(JSON.stringify(input));
    expect(invocation.prompt).toContain(QA_TEST_CASE_MIXED_COVERAGE_TARGET_PATCH_REPLACEMENT_SENTINEL);
    expect(invocation.prompt).not.toContain('不得泄漏的旧任务风险原因');
    expect(invocation.prompt).not.toContain('未点名风险正文不得泄漏');
    const skeleton = mixedCoverageTargetPatchPromptSkeleton(invocation.prompt) as {
      riskRepairs: Array<{ riskId: string; resolutions: Array<Record<string, unknown>> }>;
      targetRepairs: Array<Record<string, unknown>>;
    };
    expect(skeleton.riskRepairs).toHaveLength(1);
    expect(skeleton.riskRepairs[0].riskId).toBe('RISK-PAYMENT-STATE');
    expect(skeleton.riskRepairs[0].resolutions.map((resolution) => resolution.category))
      .toEqual(['positive', 'boundary']);
    expect(skeleton.targetRepairs).toEqual([{
      targetRef: 'task:0',
      riskId: QA_TEST_CASE_MIXED_COVERAGE_TARGET_PATCH_REPLACEMENT_SENTINEL,
      contextRef: QA_TEST_CASE_MIXED_COVERAGE_TARGET_PATCH_REPLACEMENT_SENTINEL,
      category: QA_TEST_CASE_MIXED_COVERAGE_TARGET_PATCH_REPLACEMENT_SENTINEL,
      title: QA_TEST_CASE_MIXED_COVERAGE_TARGET_PATCH_REPLACEMENT_SENTINEL,
      step: QA_TEST_CASE_MIXED_COVERAGE_TARGET_PATCH_REPLACEMENT_SENTINEL,
      expectedResult: QA_TEST_CASE_MIXED_COVERAGE_TARGET_PATCH_REPLACEMENT_SENTINEL,
    }]);
  });

  it('fails closed before mixed patch invocation for stale targets or invalid candidate paths', () => {
    const parameters = {
      input: { subject: { title: '支付结算' } },
      allowedRefs: [
        'subject:title',
        'requirement:0:acceptance_criteria',
        'task:0:status',
      ],
      draft: mixedCoverageTargetPatchDraft(),
      riskDiagnostics: [{
        code: 'risk_coverage_decision_mismatch' as const,
        riskId: 'RISK-PAYMENT-STATE',
        priority: 'P1' as const,
        actualCategories: ['negative'] as ('positive' | 'negative' | 'boundary')[],
        missingCategories: ['positive', 'boundary'] as ('positive' | 'negative' | 'boundary')[],
        expectedStatus: 'omitted' as const,
        message: 'coverage',
      }],
      targetDiagnostics: [{
        code: 'context_item_not_covered' as const,
        targetRef: 'task:0',
        candidateContextRefs: ['task:0:status'],
        group: 'task:0',
        message: 'context item is not covered: task:0',
      }],
      eligibleTargetCoverageSpecs: [{
        targetRef: 'task:0',
        riskId: 'RISK-TASK-STATE',
        contextRef: 'task:0:status',
      }],
    };

    expect(() => invokeQATestCaseMixedCoverageTargetPatchSkill({
      ...parameters,
      eligibleTargetCoverageSpecs: [{
        targetRef: 'requirement:0',
        riskId: 'RISK-PAYMENT-STATE',
        contextRef: 'requirement:0:acceptance_criteria',
      }],
    })).toThrow(/spec points at an unmentioned target/);
    expect(() => invokeQATestCaseMixedCoverageTargetPatchSkill({
      ...parameters,
      eligibleTargetCoverageSpecs: [{
        targetRef: 'task:0',
        riskId: 'RISK-TASK-STATE',
        contextRef: 'requirement:0:acceptance_criteria',
      }],
    })).toThrow(/spec has an invalid target leaf/);
    expect(() => invokeQATestCaseMixedCoverageTargetPatchSkill({
      ...parameters,
      eligibleTargetCoverageSpecs: [{
        targetRef: 'task:0',
        riskId: 'RISK-PAYMENT-STATE',
        contextRef: 'task:0:status',
      }],
    })).toThrow(/spec leaf is not declared by risk/);
    expect(() => invokeQATestCaseMixedCoverageTargetPatchSkill({
      ...parameters,
      eligibleTargetCoverageSpecs: [
        parameters.eligibleTargetCoverageSpecs[0],
        parameters.eligibleTargetCoverageSpecs[0],
      ],
    })).toThrow(/eligible target coverage specs must be unique/);
  });

  it('rejects mechanically resolvable P2 decisions before model invocation', () => {
    expect(() => invokeQATestCaseCoveragePatchSkill({
      input: { subject: { title: '支付结算' } },
      allowedRefs: ['subject:title', 'requirement:0:acceptance_criteria'],
      draft: coveragePatchDraft(),
      diagnostics: [{
        code: 'risk_coverage_decision_mismatch',
        riskId: 'RISK-UNRELATED',
        priority: 'P2',
        actualCategories: [],
        missingCategories: ['positive', 'negative', 'boundary'],
        expectedStatus: 'not_required',
        message: 'risk RISK-UNRELATED coverage decision must be not_required for P2',
      }],
    })).toThrow(/differs from exact riskLinks coverage/);
  });

  it('fails closed when coverage diagnostics do not match literal riskLinks categories', () => {
    const parameters = {
      input: { subject: { title: '支付结算' } },
      allowedRefs: ['subject:title', 'requirement:0:acceptance_criteria'],
      draft: coveragePatchDraft(),
    };

    expect(() => invokeQATestCaseCoveragePatchSkill({
      ...parameters,
      diagnostics: [{
        code: 'risk_coverage_decision_mismatch',
        riskId: 'RISK-PAYMENT-STATE',
        priority: 'P1',
        actualCategories: ['negative', 'boundary'],
        missingCategories: ['positive'],
        expectedStatus: 'omitted',
        message: 'mismatch',
      }],
    })).toThrow(/differs from exact riskLinks coverage/);
    expect(() => invokeQATestCaseCoveragePatchSkill({
      ...parameters,
      diagnostics: [{
        code: 'risk_coverage_decision_mismatch',
        riskId: 'RISK-PAYMENT-STATE',
        priority: 'P1',
        actualCategories: ['negative'],
        missingCategories: ['positive', 'boundary'],
        expectedStatus: 'omitted',
        message: 'first',
      }, {
        code: 'risk_coverage_decision_mismatch',
        riskId: 'RISK-PAYMENT-STATE',
        priority: 'P1',
        actualCategories: ['negative'],
        missingCategories: ['positive', 'boundary'],
        expectedStatus: 'omitted',
        message: 'duplicate',
      }],
    })).toThrow(/diagnostic risk IDs must be unique/);
  });

  it('rejects a malformed or duplicate Draft context-ref whitelist before model invocation', () => {
    expect(() => buildQATestCaseDraftSkillInvocation({}, [])).toThrow(/at least one allowed context ref/);
    expect(() => buildQATestCaseDraftSkillInvocation({}, ['requirement:01:title'])).toThrow(/non-canonical/);
    expect(() => buildQATestCaseDraftSkillInvocation({}, ['requirement:0'])).toThrow(/non-canonical/);
    expect(() => buildQATestCaseDraftSkillInvocation({}, ['source:0:title'])).toThrow(/non-canonical/);
    expect(() => buildQATestCaseDraftSkillInvocation({}, ['task:0:id'])).toThrow(/non-canonical/);
    expect(() => buildQATestCaseDraftSkillInvocation({}, ['subject:title', 'subject:title'])).toThrow(/must be unique/);
  });

  it('propagates the stable duplicate requirement ID error before Draft model invocation', () => {
    const input = {
      subject: { title: '重复需求编号' },
      test_case_context: {
        requirements: [{ id: 1, title: '需求一' }, { id: '1', title: '需求二' }],
        tasks: [],
      },
    };

    expect(() => buildQATestCaseDraftSkillInvocation(input, [
      'subject:title', 'requirement:0:title', 'requirement:1:title',
    ])).toThrow('qa test case context has duplicate canonical requirement IDs');
  });

  it('loads advanced rules for high-risk, formal-review, and supplied multi-source work', () => {
    expect(requiresAdvancedQATestCaseRules({ subject: { risk_level: 'high' } })).toBe(true);
    expect(requiresAdvancedQATestCaseRules({ subject: { formal_review: true } })).toBe(true);
    expect(requiresAdvancedQATestCaseRules({
      sources: [{ available: true }, { available: false }],
    })).toBe(true);
  });

  it('fails closed when a vendored file is changed', () => {
    const copy = copiedBundle();
    const target = join(copy.root, 'output-templates/structured-test-cases.md');
    writeFileSync(target, `${readFileSync(target, 'utf8')}tampered\n`);
    expect(() => loadFixedQATestCaseSkill(copy.root, copy.lockPath)).toThrow(/content hash mismatch/);
  });

  it('fails closed when the package contains an untracked file', () => {
    const copy = copiedBundle();
    writeFileSync(join(copy.root, 'EXTRA.md'), 'unexpected');
    expect(() => loadFixedQATestCaseSkill(copy.root, copy.lockPath)).toThrow(/file list differs/);
  });

  it('fails closed when the executable mode differs from upstream', () => {
    const copy = copiedBundle();
    chmodSync(join(copy.root, 'scripts/validate_testcases.py'), 0o644);
    expect(() => loadFixedQATestCaseSkill(copy.root, copy.lockPath)).toThrow(/file mode mismatch/);
  });

  it('fails closed when both the executable mode and editable lock are changed', () => {
    const copy = copiedBundle();
    chmodSync(join(copy.root, 'scripts/validate_testcases.py'), 0o644);
    const lock = JSON.parse(readFileSync(copy.lockPath, 'utf8')) as {
      files: Record<string, { mode: string }>;
    };
    lock.files['scripts/validate_testcases.py'].mode = '100644';
    writeFileSync(copy.lockPath, JSON.stringify(lock));
    expect(() => loadFixedQATestCaseSkill(copy.root, copy.lockPath)).toThrow(/locked file mode differs/);
  });
});
