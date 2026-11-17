import { describe, expect, it } from 'bun:test';
import {
  applyQATestCaseGlobalPlanRefPatch,
  parseQATestCaseGlobalPlanRefPatchText,
  QATestCaseGlobalPlanRefPatchError,
  resolveQATestCaseGlobalPlanRefPatchTargets,
} from './qa-test-case-global-plan-ref-patch.js';
import {
  compileQATestCaseGlobalPlan,
  QATestCaseGlobalPlanCompilationError,
} from './qa-test-case-global-plan.js';
import type {
  QATestCaseGlobalPlan,
  QATestCaseGlobalPlanRefDiagnostic,
} from './qa-test-case-global-plan-schema.js';

const input = {
  subject: { title: '发布流程', description: '审批后执行发布并记录审计' },
  sources: [{ available: true, content: '审批、发布和审计属于同一发布链路' }],
  test_case_context: {
    requirements: [
      { id: 11, title: '发布审批' },
      { id: 22, title: '执行发布' },
    ],
    tasks: [
      { id: 101, requirement_id: 11, title: '实现审批' },
      { id: 202, requirement_id: 22, title: '实现发布' },
    ],
  },
};

const allowedRefs = [
  'subject:title', 'subject:description', 'source:0:content',
  'requirement:0:title', 'requirement:1:title',
  'task:0:title', 'task:1:title',
];

function invalidPlan(): QATestCaseGlobalPlan {
  return {
    summary: '跨需求发布计划',
    objective: '验证审批后执行发布',
    priorityLogic: '先审批再发布',
    methods: ['user_journey'],
    sourceFindings: [{
      status: 'gap', summary: '外部规则引用待修复', contextRefs: ['source:9:content'],
    }],
    interactions: [{
      description: '审批结果未正确传递会错误发布',
      priority: 'P1',
      relationshipRef: 'source:8:content',
      coverageDecision: { status: 'omitted', reason: '异常与边界仍需分片覆盖' },
      cases: [{
        title: '审批后执行发布', category: 'integration', priority: 'P1',
        contextRefs: ['requirement:9:title', 'requirement:1:title'],
        preconditions: [], testData: [],
        actions: [{ step: '审批并发布', expectedResult: '发布记录显示审批结果' }],
        notes: [], smoke: true, automationCandidate: true, releaseBlocking: true,
        executionPosition: 'before_batches',
      }],
    }],
    noCrossInteractionReason: '',
  };
}

function diagnostics(): QATestCaseGlobalPlanRefDiagnostic[] {
  return refDiagnostics(invalidPlan());
}

function refDiagnostics(plan: QATestCaseGlobalPlan): QATestCaseGlobalPlanRefDiagnostic[] {
  let failure: unknown;
  try {
    compileQATestCaseGlobalPlan(input, plan);
  } catch (error) {
    failure = error;
  }
  expect(failure).toBeInstanceOf(QATestCaseGlobalPlanCompilationError);
  return (failure as QATestCaseGlobalPlanCompilationError).diagnostics;
}

function validPatch() {
  return {
    repairs: [
      { path: 'sourceFindings[0].contextRefs[0]', replacementRef: 'source:0:content' },
      { path: 'interactions[0].relationshipRef', replacementRef: 'subject:description' },
      {
        path: 'interactions[0].cases[0].contextRefs[0]',
        replacementRef: 'requirement:0:title',
      },
    ],
  };
}

describe('Global Plan unavailable ref diagnostics', () => {
  it('records fixed source, relationship, and nested case paths without business text', () => {
    expect(diagnostics()).toEqual([
      expect.objectContaining({
        code: 'unavailable_context_ref',
        path: 'sourceFindings[0].contextRefs[0]', kind: 'any',
      }),
      expect.objectContaining({
        code: 'unavailable_relationship_ref',
        path: 'interactions[0].relationshipRef', kind: 'relationship',
      }),
      expect.objectContaining({
        code: 'unavailable_context_ref',
        path: 'interactions[0].cases[0].contextRefs[0]', kind: 'any',
      }),
    ]);
    expect(JSON.stringify(diagnostics())).not.toContain('审批后执行发布并记录审计');
  });

  it('leaves diagnostics empty for non-ref compilation failures', () => {
    const failure = new QATestCaseGlobalPlanCompilationError(['global case titles must be unique']);
    expect(failure.diagnostics).toEqual([]);
  });
});

describe('resolveQATestCaseGlobalPlanRefPatchTargets', () => {
  it('excludes occupied nested-case and source-finding refs from exact candidates', () => {
    const plan = invalidPlan();
    plan.sourceFindings[0].contextRefs.push('subject:title');
    const targets = resolveQATestCaseGlobalPlanRefPatchTargets(
      plan, refDiagnostics(plan), allowedRefs,
    );

    expect(targets.find((target) => target.path === 'sourceFindings[0].contextRefs[0]')
      ?.candidateRefs).not.toContain('subject:title');
    expect(targets.find((target) => (
      target.path === 'interactions[0].cases[0].contextRefs[0]'
    ))?.candidateRefs).not.toContain('requirement:1:title');
  });

  it('restricts relationship candidates to explicit relationship leaves', () => {
    const target = resolveQATestCaseGlobalPlanRefPatchTargets(
      invalidPlan(), diagnostics(), allowedRefs,
    ).find((candidate) => candidate.kind === 'relationship');

    expect(target?.candidateRefs).toEqual(['subject:description', 'source:0:content']);
  });

  it('fails closed when an unavailable ref has no unoccupied candidate', () => {
    const plan = invalidPlan();
    plan.sourceFindings = [];
    plan.interactions[0].relationshipRef = 'subject:description';
    plan.interactions[0].cases[0].contextRefs = [
      'requirement:9:title', 'subject:description', 'requirement:1:title',
    ];
    const refs = ['subject:description', 'requirement:1:title'];

    expect(() => resolveQATestCaseGlobalPlanRefPatchTargets(
      plan, refDiagnostics(plan), refs,
    )).toThrow('has no replacement candidates');
  });
});

describe('applyQATestCaseGlobalPlanRefPatch', () => {
  it('replaces only diagnosed paths on a clone and passes the final compiler', () => {
    const original = invalidPlan();
    const before = structuredClone(original);
    const repaired = applyQATestCaseGlobalPlanRefPatch(
      original, diagnostics(), allowedRefs, validPatch(),
    );

    expect(original).toEqual(before);
    expect(repaired.sourceFindings[0].contextRefs).toEqual(['source:0:content']);
    expect(repaired.interactions[0].relationshipRef).toBe('subject:description');
    expect(repaired.interactions[0].cases[0].contextRefs).toEqual([
      'requirement:0:title', 'requirement:1:title',
    ]);
    expect(repaired.interactions[0].description).toBe(before.interactions[0].description);
    expect(() => compileQATestCaseGlobalPlan(input, repaired)).not.toThrow();
  });

  it('rejects extra, missing, and duplicate repair paths', () => {
    const extra = validPatch();
    extra.repairs.push({
      path: 'interactions[0].cases[0].contextRefs[1]', replacementRef: 'task:1:title',
    });
    expect(() => applyQATestCaseGlobalPlanRefPatch(
      invalidPlan(), diagnostics(), allowedRefs, extra,
    )).toThrow('repairs must exactly match diagnostics');

    const missing = validPatch();
    missing.repairs.pop();
    expect(() => applyQATestCaseGlobalPlanRefPatch(
      invalidPlan(), diagnostics(), allowedRefs, missing,
    )).toThrow('repairs must exactly match diagnostics');

    const duplicate = validPatch();
    duplicate.repairs[2] = { ...duplicate.repairs[1] };
    expect(() => applyQATestCaseGlobalPlanRefPatch(
      invalidPlan(), diagnostics(), allowedRefs, duplicate,
    )).toThrow('paths must be unique');
  });

  it('rejects refs outside the exact allowed list and invalid ref syntax', () => {
    const unavailable = validPatch();
    unavailable.repairs[2].replacementRef = 'requirement:7:title';
    expect(() => applyQATestCaseGlobalPlanRefPatch(
      invalidPlan(), diagnostics(), allowedRefs, unavailable,
    )).toThrow('replacement ref is not in the allowed list');

    const malformed = validPatch() as unknown as { repairs: Array<Record<string, unknown>> };
    malformed.repairs[2].replacementRef = 'nearby requirement';
    expect(() => applyQATestCaseGlobalPlanRefPatch(
      invalidPlan(), diagnostics(), allowedRefs, malformed,
    )).toThrow('strict schema');
  });

  it('restricts relationship replacements to explicit relationship evidence', () => {
    const patch = validPatch();
    patch.repairs[1].replacementRef = 'requirement:0:title';
    expect(() => applyQATestCaseGlobalPlanRefPatch(
      invalidPlan(), diagnostics(), allowedRefs, patch,
    )).toThrow('relationship replacement ref is not allowed');
  });

  it('rejects stale diagnostics and replacements that create duplicate refs', () => {
    const stale = diagnostics();
    stale[0] = { ...stale[0], kind: 'relationship' };
    expect(() => applyQATestCaseGlobalPlanRefPatch(
      invalidPlan(), stale, allowedRefs, validPatch(),
    )).toThrow('diagnostic does not match path kind');

    const duplicate = validPatch();
    duplicate.repairs[2].replacementRef = 'requirement:1:title';
    expect(() => applyQATestCaseGlobalPlanRefPatch(
      invalidPlan(), diagnostics(), allowedRefs, duplicate,
    )).toThrow('replacement ref is not a candidate');
  });

  it('keeps the final duplicate guard for multiple diagnostics in the same owner', () => {
    const plan = invalidPlan();
    plan.sourceFindings[0].contextRefs = ['source:9:content', 'source:8:content'];
    plan.interactions[0].relationshipRef = 'subject:description';
    plan.interactions[0].cases[0].contextRefs[0] = 'requirement:0:title';
    const sourceDiagnostics = refDiagnostics(plan);
    expect(resolveQATestCaseGlobalPlanRefPatchTargets(
      plan, sourceDiagnostics, allowedRefs,
    ).every((target) => target.candidateRefs.includes('source:0:content'))).toBe(true);

    expect(() => applyQATestCaseGlobalPlanRefPatch(
      plan,
      sourceDiagnostics,
      allowedRefs,
      { repairs: sourceDiagnostics.map((diagnostic) => ({
        path: diagnostic.path, replacementRef: 'source:0:content',
      })) },
    )).toThrow('created duplicate refs');
  });
});

describe('parseQATestCaseGlobalPlanRefPatchText', () => {
  it('accepts complete JSON and fenced JSON after thinking text', () => {
    const patch = validPatch();
    expect(parseQATestCaseGlobalPlanRefPatchText(JSON.stringify(patch))).toEqual(patch);
    expect(parseQATestCaseGlobalPlanRefPatchText(
      `<think>repair refs</think>\n\`\`\`json\n${JSON.stringify(patch)}\n\`\`\``,
    )).toEqual(patch);
  });

  it.each([
    ['not-json PRIVATE_RECEIVED_VALUE', 'global_plan_ref_patch_invalid_json'],
    [JSON.stringify({ repairs: [], received: 'PRIVATE_RECEIVED_VALUE' }), 'global_plan_ref_patch_invalid_schema'],
  ])('classifies invalid text safely without echoing raw values', (text, issue) => {
    let failure: unknown;
    try {
      parseQATestCaseGlobalPlanRefPatchText(text);
    } catch (error) {
      failure = error;
    }
    expect(failure).toBeInstanceOf(QATestCaseGlobalPlanRefPatchError);
    expect((failure as QATestCaseGlobalPlanRefPatchError).issues).toEqual([issue]);
    expect((failure as Error).message).not.toContain('PRIVATE_RECEIVED_VALUE');
  });
});
