import { describe, expect, it } from 'bun:test';
import { validateCapabilityOutput } from '../../executor.js';
import {
  applyTestCaseMechanicalRepairs,
  normalizeTestCaseGenerationOutput,
  parseTestCaseGenerationText,
} from '../index.js';
import { completeTestCaseOutput } from './generate-test-fixtures.js';

describe('test-case generation', () => {
  it('leaves vague expectations and unexplained high-risk gaps for the model repair gate', () => {
    const output = completeTestCaseOutput();
    output.testCases[0].expectedResults = ['符合预期'];
    output.testCases[0].expectedResult = '符合预期';
    output.testCaseDesign!.coverageStrategy.riskHotspots = [{
      id: 'REQ-TASK-STATUS', description: '任务状态错误会影响发布', priority: 'P0', evidence: ['subject.title'],
    }];
    const input = { subject: { title: '结算页' }, test_case_context: { tasks: [{ status: 'todo' }] } };

    expect(() => validateCapabilityOutput('qa.test_cases', input, output)).toThrow(/not observable|high-risk coverage omission/);

    const repaired = applyTestCaseMechanicalRepairs(input, output);
    expect(repaired.testCases[0].expectedResults).toEqual(['符合预期']);
    expect(repaired.testCaseDesign?.traceability[0].gapNote).toBe('');
    expect(() => validateCapabilityOutput('qa.test_cases', input, repaired)).toThrow(/not observable|high-risk coverage omission/);
  });

  it('does not replace invalid evidence with unrelated input facts', () => {
    const input = { subject: { title: '结算页' }, test_case_context: { tasks: [{ title: '提交结算' }] } };
    const output = completeTestCaseOutput();
    output.testCases[0].evidence = ['task title'];
    output.testCaseDesign!.sourceInventory[0].evidence = ['requirement title'];
    output.testCaseDesign!.coverageStrategy.riskHotspots = [{
      id: 'RISK-UNTRACED', description: '结算重复提交', priority: 'P0', evidence: ['task title'],
    }];

    const repaired = applyTestCaseMechanicalRepairs(input, output);

    expect(repaired.testCases[0].evidence).toEqual(['task title']);
    expect(repaired.testCaseDesign?.sourceInventory[0].evidence).toEqual(['requirement title']);
    expect(repaired.testCaseDesign?.coverageStrategy.riskHotspots).toEqual(output.testCaseDesign?.coverageStrategy.riskHotspots);
    expect(() => validateCapabilityOutput('qa.test_cases', input, repaired)).toThrow(/evidence is missing or invalid|high-risk hotspot/);
  });

  it('keeps legacy expectedResult lossless without inventing missing design metadata', () => {
    const input = { subject: { title: '结算页' } };
    const output = normalizeTestCaseGenerationOutput({
      summary: '旧格式用例',
      testCases: [{
        caseId: 'TC-PAY-001', title: '提交结算', category: 'positive', priority: 'P1',
        trace: ['REQ-PAY'], steps: ['提交', '查看结果'], expectedResults: ['显示处理中', '显示成功'],
        evidence: ['subject.title'],
      }],
    });

    const repaired = applyTestCaseMechanicalRepairs(input, output);
    expect(repaired.testCases[0].expectedResult).toBe('显示处理中\n显示成功');
    expect(repaired.testCaseDesign).toBeNull();
    expect(() => validateCapabilityOutput('qa.test_cases', input, repaired)).toThrow('test case design metadata is missing');
  });

  it.each([
    [{ category: 'surprise', priority: 'P1' }, 'unknown category'],
    [{ category: 'functional', priority: 'P1' }, 'ambiguous functional category'],
    [{ priority: 'P1' }, 'missing category'],
    [{ category: 'positive', priority: 'urgent-ish' }, 'unknown priority'],
    [{ category: 'positive' }, 'missing priority'],
  ])('rejects %s instead of silently downgrading it (%s)', (fields, _label) => {
    expect(() => normalizeTestCaseGenerationOutput({
      summary: '不完整用例',
      testCases: [{
        caseId: 'TC-PAY-001', title: '提交结算', trace: ['REQ-PAY'], steps: ['提交'],
        expectedResults: ['显示成功'], evidence: ['subject.title'], ...fields,
      }],
    })).toThrow();
  });

  it('recovers common compatible-model aliases into the artifact-only contract', () => {
    const output = normalizeTestCaseGenerationOutput({
      summary: '已覆盖核心场景',
      test_cases: [{
        id: 'TC-SUBMIT-001', name: '重复提交', type: '异常', priority: 'p1', precondition: '用户已登录',
        requirement_ids: ['REQ-SUBMIT'], actions: ['连续提交两次'], expected_result: '只生成一条记录', evidence: ['subject.acceptance_criteria'],
      }],
    });

    expect(output.testCases[0]).toMatchObject({ title: '重复提交', category: 'negative', priority: 'P1' });
    expect(output.changes).toEqual([]);
    expect(output.workflowProposal).toBeNull();
  });

  it('recovers fenced JSON after model reasoning and normalizes object-shaped steps', () => {
    const output = parseTestCaseGenerationText(`<think>正在组织用例</think>
以下是结果：
\`\`\`json
{"data":{"summary":"已覆盖提交场景","cases":[{"id":"TC-SUBMIT-001","case_name":"有效数据提交","test_type":"success","level":"high","requirement_ids":["REQ-SUBMIT"],"conditions":"用户已登录","test_steps":[{"action":"输入有效数据"},{"description":"点击提交"}],"expected_results":"保存成功","basis":"subject.acceptance_criteria"}]}}
\`\`\`
请人工复核。`);

    expect(output.testCases[0]).toMatchObject({
      title: '有效数据提交', category: 'positive', priority: 'P1',
      preconditions: ['用户已登录'], steps: ['输入有效数据', '点击提交'], expectedResult: '保存成功',
    });
  });

  it('extracts the first balanced JSON object when the model appends trailing prose', () => {
    const output = parseTestCaseGenerationText(`{"summary":"已覆盖","design":{"objective":"验证","scope":["主流程"],"exclusions":[],"sourceInventory":[{"id":"SRC-1","kind":"需求","status":"confirmed","summary":"标题","evidence":["subject.title"]}],"assumptions":[],"informationGaps":[],"coverageStrategy":{"priorityLogic":"主路径","methods":["user_journey"],"riskHotspots":[]},"testCases":[{"caseId":"TC-MAIN-001","title":"打开页面","category":"positive","priority":"P2","trace":["REQ-1"],"preconditions":[],"testData":[],"steps":["打开"],"expectedResults":["可见"],"notes":["REQ-1 为派生标签"],"evidence":["subject.title"]}],"traceability":[{"traceId":"REQ-1","caseIds":["TC-MAIN-001"],"coverageTypes":["positive"],"gapNote":""}],"smokeCaseIds":["TC-MAIN-001"],"executionOrder":["TC-MAIN-001"],"automationCandidateCaseIds":[],"releaseBlockingCaseIds":[]}}
以上是完整结果，请人工确认。`);

    expect(output.testCases[0]).toMatchObject({ caseId: 'TC-MAIN-001', title: '打开页面' });
  });

  it('rejects a response truncated after the test-case array', () => {
    expect(() => parseTestCaseGenerationText(`{"summary":"已覆盖","design":{"testCases":[
      {"caseId":"TC-MAIN-001","title":"成功提交","category":"positive","priority":"P0","trace":["REQ-MAIN"],"preconditions":[],"testData":[],"steps":["提交"],"expectedResults":["显示成功"],"notes":[],"evidence":["subject.title"]},
      {"caseId":"TC-MAIN-002","title":"拒绝无效输入","category":"negative","priority":"P1","trace":["REQ-MAIN"],"preconditions":[],"testData":[],"steps":["填写无效值","提交无效值"],"expectedResults":["显示校验提示"],"notes":[],"evidence":["subject.title"]}
    ],"traceability":[`)).toThrow();
  });

  it('normalizes skill-shaped metadata and scalar list fields from compatible models', () => {
    const output = normalizeTestCaseGenerationOutput({
      summary: { title: '结算测试用例设计', version: '1.0' },
      design: {
        objective: '验证结算', scope: '结算提交', exclusions: '',
        sourceInventory: [{ id: 'SRC-1', kind: '需求', status: 'confirmed', summary: '结算要求', evidence: 'subject.title' }],
        assumptions: '', informationGaps: '',
        coverageStrategy: { priorityLogic: '核心路径优先', methods: ['decision_table', 'error_guessing'], riskHotspots: [] },
        testCases: [{
          caseId: 'TC-PAY-001', title: '成功结算', category: 'positive', priority: 'P0',
          trace: 'REQ-PAY', preconditions: '用户已登录', testData: '有效订单', steps: ['提交结算'],
          expectedResults: ['显示结算成功'], notes: 'REQ-PAY 为派生标签', evidence: 'subject.title',
        }],
        traceability: [{ traceId: 'REQ-PAY', caseIds: 'TC-PAY-001', coverageTypes: 'positive', gapNote: '' }],
        smokeCaseIds: 'TC-PAY-001', executionOrder: 'TC-PAY-001',
        automationCandidateCaseIds: 'TC-PAY-001', releaseBlockingCaseIds: 'TC-PAY-001',
      },
    });

    expect(output.summary).toBe('结算测试用例设计');
    expect(output.testCaseDesign?.sourceInventory[0].evidence).toEqual(['subject.title']);
    expect(output.testCaseDesign?.coverageStrategy.methods).toEqual(['decision_table', 'error_guessing']);
    expect(output.testCases[0]).toMatchObject({ trace: ['REQ-PAY'], preconditions: ['用户已登录'] });
  });

  it('rejects case categories masquerading as design methods', () => {
    const output = completeTestCaseOutput();
    const testCases = output.testCases.map(({ expectedResult: _expectedResult, ...testCase }) => testCase);
    expect(() => normalizeTestCaseGenerationOutput({
      summary: output.summary,
      design: {
        ...output.testCaseDesign,
        coverageStrategy: { ...output.testCaseDesign!.coverageStrategy, methods: ['permission'] },
        testCases,
      },
    })).toThrow();
  });

  it('preserves stable multi-segment case IDs and separates trace references', () => {
    const output = normalizeTestCaseGenerationOutput({
      summary: '上传测试',
      testCases: [{
        caseId: 'TC-UPLOAD-FMT-001', title: '上传 PNG', category: 'positive', priority: 'P0',
        trace: 'REQ-UPLOAD; RISK-FORMAT', steps: ['上传 PNG'], expectedResults: ['显示上传成功'],
        evidence: 'subject.title',
      }],
    });

    expect(output.testCases[0]).toMatchObject({
      caseId: 'TC-UPLOAD-FMT-001', trace: ['REQ-UPLOAD', 'RISK-FORMAT'],
    });
  });

  it('preserves step/result and trace mismatches for the deterministic repair gate', () => {
    const output = normalizeTestCaseGenerationOutput({
      summary: '上传测试',
      design: {
        objective: '验证上传', scope: ['上传'], exclusions: [],
        sourceInventory: [{ id: 'SRC-1', kind: '需求', status: 'confirmed', summary: '上传需求', evidence: ['subject.title'] }],
        assumptions: [], informationGaps: [],
        coverageStrategy: { priorityLogic: '主路径优先', methods: ['user_journey'], riskHotspots: [] },
        testCases: [{
          caseId: 'TC-UPLOAD-001', title: '上传文件', category: 'positive', priority: 'P0', trace: ['REQ-UPLOAD'],
          preconditions: [], testData: [], steps: ['选择文件', '提交上传', '查看附件'],
          expectedResults: ['请求被受理', '显示附件'], notes: [], evidence: ['subject.title'],
        }],
        traceability: [{ traceId: 'REQ-UPLOAD', caseIds: ['TC-UPLOAD-001'], coverageTypes: ['negative'], gapNote: '' }],
        smokeCaseIds: ['TC-UPLOAD-001'], executionOrder: ['TC-UPLOAD-001'],
        automationCandidateCaseIds: [], releaseBlockingCaseIds: [],
      },
    });

    expect(output.testCases[0].steps).toEqual(['选择文件', '提交上传', '查看附件']);
    expect(output.testCases[0].expectedResults).toEqual(['请求被受理', '显示附件']);
    expect(output.testCaseDesign?.traceability).toContainEqual(expect.objectContaining({
      traceId: 'REQ-UPLOAD', caseIds: ['TC-UPLOAD-001'], coverageTypes: ['negative'],
    }));
    expect(() => validateCapabilityOutput('qa.test_cases', { subject: { title: '上传' } }, output))
      .toThrow(/steps and expected results must align|traceability coverage types do not match cases/);
  });
});
