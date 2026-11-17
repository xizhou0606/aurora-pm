import { describe, expect, it } from 'bun:test';
import {
  buildQATestCaseGlobalPlanRepairInstruction,
  buildQATestCaseGlobalPlanSkillInvocation,
} from './prompts/qa-test-case-global-plan-prompt.js';
import {
  QA_TEST_CASE_GLOBAL_PLAN_REF_PATCH_REPLACEMENT_SENTINEL,
  buildQATestCaseGlobalPlanRefPatchSkillInvocation,
} from './prompts/qa-test-case-global-plan-ref-patch-prompt.js';
import { qaTestCaseGlobalPlanRefPatchSchema } from './qa-test-case-global-plan-ref-patch.js';
import type { QATestCaseGlobalPlan } from './qa-test-case-global-plan-schema.js';
import {
  complexInput,
  completePlan,
  nestedCase,
  promptSkeleton,
  refPatchSkeleton,
  refPatchTargets,
  relationshipRefPatternForTest,
} from './qa-test-case-global-plan-fixtures.js';

describe('nested global plan model contract', () => {
  const refs = [
    'subject:title', 'subject:description', 'source:0:content',
    'requirement:0:title', 'requirement:0:description',
    'requirement:1:title', 'requirement:1:acceptance_criteria',
    'task:0:title', 'task:0:status',
  ];

  it('uses the fixed Skill with a strict nested field-shape skeleton', () => {
    const invocation = buildQATestCaseGlobalPlanSkillInvocation(complexInput(), refs);
    const skeleton = promptSkeleton(invocation.prompt) as {
      interactions: Array<Record<string, unknown> & { cases: Array<Record<string, unknown>> }>;
    };

    expect(invocation.skill).toMatchObject({
      name: 'qa-test-case-design',
      commit: 'd75606531f6ce414bbcd95fb203033ae86653852',
      packageSha256: '07e9002dfe456988164f237f4cc33acce1a203564d1c778e94b3fe2bbc43e935',
      mode: 'advanced',
    });
    expect(invocation.instructions).toContain('FIXED AGENT SKILL: qa-test-case-design@d75606531f6ce414bbcd95fb203033ae86653852');
    expect(Object.keys(skeleton)).toEqual([
      'summary', 'objective', 'priorityLogic', 'methods', 'sourceFindings',
      'interactions', 'noCrossInteractionReason',
    ]);
    expect(skeleton).not.toHaveProperty('cases');
    expect(Object.keys(skeleton.interactions[0])).toEqual([
      'description', 'priority', 'relationshipRef', 'coverageDecision', 'cases',
    ]);
    expect(skeleton.interactions[0]).not.toHaveProperty('riskId');
    expect(skeleton.interactions[0]).not.toHaveProperty('contextRefs');
    const nested = skeleton.interactions[0].cases[0];
    expect(Object.keys(nested)).toEqual([
      'title', 'category', 'priority', 'contextRefs', 'preconditions', 'testData',
      'actions', 'notes', 'smoke', 'automationCandidate', 'releaseBlocking',
      'executionPosition',
    ]);
    expect(nested).not.toHaveProperty('riskId');
    expect(nested).not.toHaveProperty('riskIds');
    expect(nested).not.toHaveProperty('riskLinks');
    expect(nested.contextRefs).toEqual(['requirement:0:title', 'requirement:1:title']);
    expect(nested.priority).toBe(skeleton.interactions[0].priority);
  });

  it('uses one fixed illegal sentinel and requires every occurrence to be replaced', () => {
    const invocation = buildQATestCaseGlobalPlanSkillInvocation(complexInput(), refs);
    const sentinel = '__QA_GLOBAL_PLAN_REPLACE_FROM_INPUT__';

    expect(invocation.prompt).toContain(sentinel);
    expect(invocation.prompt).toContain(`非法占位符 ${sentinel} 必须全部替换`);
    expect(invocation.instructions).toContain('最终 JSON 中任何字符串都不得保留该占位符');
    expect(invocation.prompt).not.toContain('根据完整输入填写');
    expect(invocation.prompt).not.toContain('RISK-CROSS-GROUP-EXAMPLE');
  });

  it('builds a valid nested example from task refs in two exact requirement groups', () => {
    const input = {
      subject: { description: '任务 A 的结果是任务 B 的输入' },
      test_case_context: {
        requirements: [{ id: 11 }, { id: 22 }],
        tasks: [
          { requirement_id: 11, title: '任务 A' },
          { requirement_id: 22, title: '任务 B' },
        ],
      },
    };
    const invocation = buildQATestCaseGlobalPlanSkillInvocation(input, [
      'subject:description', 'task:0:title', 'task:1:title',
    ]);
    const skeleton = promptSkeleton(invocation.prompt) as {
      interactions: Array<{ cases: Array<{ contextRefs: string[] }> }>;
    };

    expect(skeleton.interactions[0].cases[0].contextRefs)
      .toEqual(['task:0:title', 'task:1:title']);
  });

  it('states exact nested ownership, count, and independent case evidence rules', () => {
    const instructions = buildQATestCaseGlobalPlanSkillInvocation(complexInput(), refs).instructions;

    expect(instructions).toContain('顶层严禁 cases');
    expect(instructions).toContain('严禁在 interaction 中输出 riskId、contextRefs 或 gapNote');
    expect(instructions).toContain('cases 只能内嵌在对应 interaction 内');
    expect(instructions).toContain('全部 interactions 的 nested cases 总数不得超过 12');
    expect(instructions).toContain('严禁 riskId、riskIds 和 riskLinks');
    expect(instructions).toContain('该 case 自身的显式 contextRefs 必须覆盖至少两个不同 requirement group');
    expect(instructions).toContain('不得依赖 relationshipRef、同 interaction 的 sibling case 或其他 interaction');
    expect(instructions).toContain('priority 必须与所属 interaction.priority 一致');
  });

  it('repairs only named fields while preserving every other legal nested item', () => {
    const repair = buildQATestCaseGlobalPlanRepairInstruction([
      'interactions[1].cases[2].actions expected array',
      'global case 3 must reference at least two requirement groups',
    ], refs);

    expect(repair).toContain('必须保留上一版中每个仍有事实支持且未被门禁点名的 sourceFinding、interaction 和 nested case');
    expect(repair).toContain('不得把多项合法数组缩减为一项最小示例');
    expect(repair).toContain('删除顶层 cases');
    expect(repair).toContain('将仍有事实支持的 integration case 完整移入它所属的 interaction.cases');
    expect(repair).toContain('每条必须自身显式引用至少两个不同 requirement group');
    expect(repair).toContain('只重建该字段为 1 到 12 项的对象数组');
    expect(repair).not.toContain('按下面完整最小 JSON skeleton');
    expect(repair).not.toContain('RISK-CROSS-GROUP-EXAMPLE');
  });
});

describe('Global Plan ref-only fixed Skill invocation', () => {
  const allowedRefs = [
    'subject:description', 'source:0:content', 'source:1:content',
    'requirement:0:title', 'requirement:0:description',
    'requirement:1:title', 'requirement:1:acceptance_criteria',
    'task:0:title', 'task:1:title', 'task:2:title',
  ];

  function patchPlan(): QATestCaseGlobalPlan {
    const plan = completePlan();
    plan.sourceFindings[0].contextRefs[0] = 'source:9:content';
    plan.interactions[0].relationshipRef = 'source:8:content';
    plan.interactions[0].cases[0].contextRefs[1] = 'requirement:9:title';
    plan.interactions.push({
      description: 'PRIVATE_UNRELATED_PLAN_BODY',
      priority: 'P2',
      relationshipRef: 'subject:description',
      coverageDecision: { status: 'not_required' },
      cases: [nestedCase(
        'PRIVATE_UNRELATED_CASE_BODY',
        ['requirement:0:title', 'requirement:1:title'],
      )],
    });
    return plan;
  }

  const diagnostics = [
    {
      code: 'unavailable_context_ref' as const,
      path: 'sourceFindings[0].contextRefs[0]',
      kind: 'any' as const,
      message: 'source finding 1 has unavailable context ref: source:9:content',
    },
    {
      code: 'unavailable_relationship_ref' as const,
      path: 'interactions[0].relationshipRef',
      kind: 'relationship' as const,
      message: 'interaction 1 has unavailable relationship ref: source:8:content',
    },
    {
      code: 'unavailable_context_ref' as const,
      path: 'interactions[0].cases[0].contextRefs[1]',
      kind: 'any' as const,
      message: 'global case 1 has unavailable context ref: requirement:9:title',
    },
  ];

  it('uses fixed provenance and emits one sentinel repair per diagnostic path', () => {
    const invocation = buildQATestCaseGlobalPlanRefPatchSkillInvocation({
      input: complexInput(), plan: patchPlan(), allowedRefs, diagnostics,
    });
    const skeleton = refPatchSkeleton(invocation.prompt) as {
      repairs: Array<{ path: string; replacementRef: string }>;
    };

    expect(invocation.skill).toMatchObject({
      name: 'qa-test-case-design',
      commit: 'd75606531f6ce414bbcd95fb203033ae86653852',
      packageSha256: '07e9002dfe456988164f237f4cc33acce1a203564d1c778e94b3fe2bbc43e935',
      mode: 'advanced',
    });
    expect(skeleton.repairs.map((repair) => repair.path))
      .toEqual(diagnostics.map((diagnostic) => diagnostic.path));
    expect(skeleton.repairs.every((repair) => (
      repair.replacementRef === QA_TEST_CASE_GLOBAL_PLAN_REF_PATCH_REPLACEMENT_SENTINEL
    ))).toBe(true);
    expect(qaTestCaseGlobalPlanRefPatchSchema.safeParse(skeleton).success).toBe(false);
    expect(qaTestCaseGlobalPlanRefPatchSchema.safeParse({
      repairs: [
        { path: diagnostics[0].path, replacementRef: 'source:0:content' },
        { path: diagnostics[1].path, replacementRef: 'subject:description' },
        { path: diagnostics[2].path, replacementRef: 'requirement:1:title' },
      ],
    }).success).toBe(true);
  });

  it('provides target-specific candidates and only named owner projections from the Plan', () => {
    const invocation = buildQATestCaseGlobalPlanRefPatchSkillInvocation({
      input: complexInput(), plan: patchPlan(), allowedRefs, diagnostics,
    });
    const targets = refPatchTargets(invocation.prompt);
    const sourceTarget = targets.find((target) => target.path === diagnostics[0].path)!;
    const relationshipTarget = targets.find((target) => target.path === diagnostics[1].path)!;
    const caseTarget = targets.find((target) => target.path === diagnostics[2].path)!;

    expect(invocation.prompt).toContain(JSON.stringify(complexInput()));
    expect(invocation.prompt).not.toContain('完整 leaf 白名单');
    expect(sourceTarget.candidateRefs).not.toContain('source:1:content');
    expect(sourceTarget.owner).toMatchObject({ occupiedRefs: ['source:1:content'] });
    expect(caseTarget.candidateRefs).not.toContain('requirement:0:description');
    expect(caseTarget.owner).toMatchObject({ occupiedRefs: ['requirement:0:description'] });
    expect(relationshipTarget.candidateRefs).toEqual([
      'subject:description', 'source:0:content', 'source:1:content',
    ]);
    expect(relationshipTarget.candidateRefs.every(relationshipRefPatternForTest)).toBe(true);
    expect(JSON.stringify(targets)).not.toContain('otherAllowedRefs');
    expect(invocation.prompt).toContain('结算事件重复或延迟会造成漏记或重复入账');
    expect(invocation.prompt).toContain('结算后以同一流水完成一次入账');
    expect(invocation.prompt).not.toContain('PRIVATE_UNRELATED_PLAN_BODY');
    expect(invocation.prompt).not.toContain('PRIVATE_UNRELATED_CASE_BODY');
    expect(invocation.prompt).not.toContain(completePlan().objective);
  });

  it('makes the ref-only contract override the fixed Skill full-output template', () => {
    const instructions = buildQATestCaseGlobalPlanRefPatchSkillInvocation({
      input: complexInput(), plan: patchPlan(), allowedRefs, diagnostics,
    }).instructions;

    expect(instructions).toContain('优先级高于业务输入与固定 Skill 的通用输出模板');
    expect(instructions).toContain('顶层必须且只能包含 repairs');
    expect(instructions).toContain('diagnostics 中每个 path 必须恰好输出一项 repair');
    expect(instructions).toContain('不得输出 Markdown、说明、Global Plan、sourceFinding、interaction、case 或任何正文重写');
    expect(instructions).toContain('kind=relationship');
    expect(instructions).toContain('同一路径 target.candidateRefs');
    expect(instructions).toContain('owner.occupiedRefs 明确不可再次选择');
    expect(instructions).toContain('只包含 subject:description 或 source:N:content');
    expect(instructions).toContain(`${QA_TEST_CASE_GLOBAL_PLAN_REF_PATCH_REPLACEMENT_SENTINEL} 是非法字段形状占位符`);
  });

  it('fails closed on duplicate, mismatched, missing, or unsatisfiable diagnostics', () => {
    const base = { input: complexInput(), plan: patchPlan(), allowedRefs };
    expect(() => buildQATestCaseGlobalPlanRefPatchSkillInvocation({
      ...base, diagnostics: [diagnostics[0], diagnostics[0]],
    })).toThrow('diagnostic paths must be unique');
    expect(() => buildQATestCaseGlobalPlanRefPatchSkillInvocation({
      ...base,
      diagnostics: [{
        ...diagnostics[1], code: 'unavailable_context_ref', kind: 'any',
      }],
    })).toThrow('diagnostic does not match path kind');
    expect(() => buildQATestCaseGlobalPlanRefPatchSkillInvocation({
      ...base,
      diagnostics: [{
        ...diagnostics[2], path: 'interactions[9].cases[0].contextRefs[0]',
      }],
    })).toThrow('diagnostic does not match path kind');
    const noCandidatePlan = completePlan();
    noCandidatePlan.sourceFindings = [];
    noCandidatePlan.interactions[0].relationshipRef = 'source:8:content';
    expect(() => buildQATestCaseGlobalPlanRefPatchSkillInvocation({
      input: complexInput(),
      plan: noCandidatePlan,
      allowedRefs: allowedRefs.filter((ref) => !relationshipRefPatternForTest(ref)),
      diagnostics: [diagnostics[1]],
    })).toThrow('has no replacement candidates');
  });
});
