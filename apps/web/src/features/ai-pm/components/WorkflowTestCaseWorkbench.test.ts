import { describe, expect, it } from 'bun:test'

import { buildTestCaseCSV, isTestCaseWorkflowNode, testCaseGenerationStartedAt } from './WorkflowTestCaseWorkbench'

describe('workflow test case helpers', () => {
  it('recognizes configured and legacy test-case human nodes only', () => {
    expect(isTestCaseWorkflowNode({ type: 'human_task', name: '测试用例编写' })).toBe(true)
    expect(isTestCaseWorkflowNode({ type: 'form', name: 'QA 交付', config: { test_case_artifact: true } })).toBe(true)
    expect(isTestCaseWorkflowNode({ type: 'approval', name: '测试用例审批' })).toBe(false)
  })

  it('restores an active generation time from started_at and a requeued run from updated_at', () => {
    expect(testCaseGenerationStartedAt({
      started_at: '2026-08-12T01:35:54.998Z',
      created_at: '2026-08-12T01:35:50.000Z',
    })).toBe(Date.parse('2026-08-12T01:35:54.998Z'))
    expect(testCaseGenerationStartedAt({ created_at: '2026-08-12T01:35:50.000Z' }))
      .toBe(Date.parse('2026-08-12T01:35:50.000Z'))
    expect(testCaseGenerationStartedAt({
      status: 'queued',
      started_at: '2026-08-12T01:35:54.998Z',
      updated_at: '2026-08-12T01:39:00.000Z',
      created_at: '2026-08-12T01:35:50.000Z',
    })).toBe(Date.parse('2026-08-12T01:39:00.000Z'))
    expect(testCaseGenerationStartedAt({ created_at: 'invalid' }, 42)).toBe(42)
  })

  it('exports Excel-friendly UTF-8 CSV and neutralizes spreadsheet formulas', () => {
    const csv = buildTestCaseCSV([{
      caseId: 'TC-SUBMIT-001',
      title: '=HYPERLINK("bad")', category: 'negative', priority: 'P1',
      trace: ['REQ-SUBMIT'], preconditions: ['已登录'], testData: ['空表单'], steps: ['提交空表单'],
      expectedResult: '显示校验错误', expectedResults: ['显示校验错误'], notes: ['派生标签'], evidence: ['subject.acceptance_criteria'],
    }], {
      objective: '验证提交', scope: ['表单'], exclusions: [], sourceInventory: [], assumptions: [], informationGaps: [],
      coverageStrategy: { priorityLogic: '风险优先', methods: ['boundary_value'], riskHotspots: [] }, traceability: [],
      smokeCaseIds: ['TC-SUBMIT-001'], executionOrder: ['TC-SUBMIT-001'], automationCandidateCaseIds: ['TC-SUBMIT-001'], releaseBlockingCaseIds: [],
    })
    expect(csv.startsWith('\uFEFF')).toBe(true)
    expect(csv).toContain('"用例ID"')
    expect(csv).toContain('"TC-SUBMIT-001"')
    expect(csv).toContain('"REQ-SUBMIT"')
    expect(csv).toContain('"空表单"')
    expect(csv).toContain('"\'=HYPERLINK(""bad"")"')
    expect(csv).toContain('"1. 提交空表单"')

    const paddedFormula = buildTestCaseCSV([{
      title: '  =1+1', category: 'boundary', priority: 'P2',
      preconditions: [], steps: ['执行'], expectedResult: '显示文本', evidence: [],
    }])
    expect(paddedFormula).toContain('"\'  =1+1"')
  })

  it('exports a large generated suite without crossing the artifact limit', () => {
    const cases = Array.from({ length: 1_000 }, (_, index) => ({
      caseId: `TC-STRESS-${String(index + 1).padStart(4, '0')}`,
      title: `复杂需求场景 ${index + 1}`,
      category: index % 2 ? 'negative' as const : 'positive' as const,
      priority: index < 20 ? 'P0' as const : 'P2' as const,
      trace: [`REQ-${Math.floor(index / 5) + 1}`],
      preconditions: ['已登录并准备独立测试数据'],
      testData: [`record-${index + 1}`],
      steps: ['提交场景数据', '查询处理结果'],
      expectedResult: '请求被处理\n页面显示可判定的结果',
      expectedResults: ['请求被处理', '页面显示可判定的结果'],
      notes: [],
      evidence: [`test_case_context.tasks[${index}].title`],
    }))

    const csv = buildTestCaseCSV(cases)
    expect(new Blob([csv]).size).toBeLessThan(20 * 1024 * 1024)
    expect(csv.split('\r\n')).toHaveLength(1_001)
    expect(csv).toContain('TC-STRESS-1000')
  })
})
