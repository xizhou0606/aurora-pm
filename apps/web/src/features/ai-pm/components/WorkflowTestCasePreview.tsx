import { copy } from '@/shared/copy'
import { StatusPill } from '@/shared/ui'
import type { TestCase, TestCaseDesign } from '../model/types'

const testCaseCopy = copy.aiPM.workflowSubject

function safeCSVCell(value: unknown) {
  let text = String(value ?? '').replace(/\r?\n/g, '\n')
  if (/^[\t ]*[=+\-@]/.test(text)) text = `'${text}`
  return `"${text.replace(/"/g, '""')}"`
}

export function resolvedTestCaseId(testCase: TestCase, index: number) {
  return testCase.caseId || testCaseCopy.testCaseNumber(index + 1)
}

function expectedResults(testCase: TestCase) {
  return testCase.expectedResults?.length
    ? testCase.expectedResults
    : testCase.expectedResult ? [testCase.expectedResult] : []
}

function numbered(values: string[]) {
  return values.map((value, index) => `${index + 1}. ${value}`).join('\n')
}

export function buildTestCaseCSV(testCases: TestCase[], design?: TestCaseDesign | null) {
  const headers = [
    '用例ID', '用例名称', '优先级', '追踪引用', '类型', '前置条件', '测试数据',
    '测试步骤', '预期结果', '备注', '事实依据', '冒烟', '执行顺序', '自动化候选', '发布阻塞',
  ]
  const rows = testCases.map((testCase, index) => {
    const caseId = resolvedTestCaseId(testCase, index)
    const executionIndex = design?.executionOrder.indexOf(caseId) ?? -1
    return [
      caseId, testCase.title, testCase.priority, testCase.trace?.join('\n') ?? '',
      testCaseCopy.testCaseCategory[testCase.category], testCase.preconditions.join('\n'),
      testCase.testData?.join('\n') ?? '', numbered(testCase.steps), numbered(expectedResults(testCase)),
      testCase.notes?.join('\n') ?? '', testCase.evidence.join('\n'),
      design?.smokeCaseIds.includes(caseId) ? testCaseCopy.testCaseYes : testCaseCopy.testCaseNo,
      executionIndex >= 0 ? executionIndex + 1 : '',
      design?.automationCandidateCaseIds.includes(caseId) ? testCaseCopy.testCaseYes : testCaseCopy.testCaseNo,
      design?.releaseBlockingCaseIds.includes(caseId) ? testCaseCopy.testCaseYes : testCaseCopy.testCaseNo,
    ]
  })
  return `\uFEFF${[headers, ...rows].map((row) => row.map(safeCSVCell).join(',')).join('\r\n')}`
}

function TextList({ values, empty = false }: { values: string[]; empty?: boolean }) {
  if (!values.length) return empty ? <span className="text-muted-foreground">{testCaseCopy.testCaseNone}</span> : null
  return <ul className="my-1 grid gap-1 pl-4">{values.map((value, index) => <li key={`${value}:${index}`}>{value}</li>)}</ul>
}

function DesignSummary({ design }: { design: TestCaseDesign }) {
  return <section className="grid gap-4 border-b border-border pb-4" aria-label={testCaseCopy.testCaseSummary}>
    <div>
      <h3 className="m-0 text-sm font-semibold text-foreground">{testCaseCopy.testCaseObjective}</h3>
      <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{design.objective}</p>
    </div>
    <div className="grid gap-3 sm:grid-cols-2">
      <div className="text-xs leading-5 text-muted-foreground"><strong className="text-foreground">{testCaseCopy.testCaseScope}</strong><TextList values={design.scope} /></div>
      <div className="text-xs leading-5 text-muted-foreground"><strong className="text-foreground">{testCaseCopy.testCaseExclusions}</strong><TextList values={design.exclusions} empty /></div>
    </div>
    <div>
      <h3 className="m-0 text-sm font-semibold text-foreground">{testCaseCopy.testCaseSources}</h3>
      <div className="mt-2 grid gap-2">{design.sourceInventory.map((source) => <div key={source.id} className="flex flex-wrap items-start gap-2 border-l-2 border-border pl-2.5 text-xs leading-5 text-muted-foreground"><StatusPill className="min-w-0" tone={source.status === 'confirmed' ? 'success' : source.status === 'conflict' ? 'danger' : 'warning'}>{testCaseCopy.testCaseSourceStatus[source.status]}</StatusPill><span className="min-w-0 flex-1"><strong className="text-foreground">{source.id} {testCaseCopy.testCaseMetaSeparator} {source.kind}</strong><span className="block">{source.summary}</span></span></div>)}</div>
    </div>
    <div className="grid gap-3 sm:grid-cols-2">
      <div className="text-xs leading-5 text-muted-foreground"><strong className="text-foreground">{testCaseCopy.testCasePriorityLogic}</strong><p className="mt-1 mb-0">{design.coverageStrategy.priorityLogic}</p></div>
      <div className="text-xs leading-5 text-muted-foreground"><strong className="text-foreground">{testCaseCopy.testCaseMethods}</strong><p className="mt-1 mb-0">{design.coverageStrategy.methods.map((method) => testCaseCopy.testCaseMethod[method]).join('、')}</p></div>
    </div>
    {design.coverageStrategy.riskHotspots.length ? <div><h3 className="m-0 text-sm font-semibold text-foreground">{testCaseCopy.testCaseRiskHotspots}</h3><div className="mt-2 grid gap-2">{design.coverageStrategy.riskHotspots.map((risk) => <div key={risk.id} className="flex items-start gap-2 text-xs leading-5 text-muted-foreground"><StatusPill className="min-w-10" tone={risk.priority === 'P0' ? 'danger' : risk.priority === 'P1' ? 'warning' : 'neutral'}>{risk.priority}</StatusPill><span><strong className="text-foreground">{risk.id}</strong> {testCaseCopy.testCaseMetaSeparator} {risk.description}</span></div>)}</div></div> : null}
    <div className="grid gap-3 sm:grid-cols-2">
      <div className="text-xs leading-5 text-muted-foreground"><strong className="text-foreground">{testCaseCopy.testCaseAssumptions}</strong><TextList values={design.assumptions} empty /></div>
      <div className="text-xs leading-5 text-muted-foreground"><strong className="text-foreground">{testCaseCopy.testCaseGaps}</strong><TextList values={design.informationGaps} empty /></div>
    </div>
    <dl className="m-0 grid gap-x-4 gap-y-2 text-xs leading-5 text-muted-foreground sm:grid-cols-[auto_1fr]">
      <dt className="font-semibold text-foreground">{testCaseCopy.testCaseSmoke}</dt><dd className="m-0 break-words">{design.smokeCaseIds.join('、')}</dd>
      <dt className="font-semibold text-foreground">{testCaseCopy.testCaseExecution}</dt><dd className="m-0 break-words">{design.executionOrder.join(' → ')}</dd>
      <dt className="font-semibold text-foreground">{testCaseCopy.testCaseAutomation}</dt><dd className="m-0 break-words">{design.automationCandidateCaseIds.join('、') || testCaseCopy.testCaseNone}</dd>
      <dt className="font-semibold text-foreground">{testCaseCopy.testCaseReleaseBlockers}</dt><dd className="m-0 break-words">{design.releaseBlockingCaseIds.join('、') || testCaseCopy.testCaseNone}</dd>
    </dl>
  </section>
}

export function TestCasePreview({ testCases, design }: { testCases: TestCase[]; design?: TestCaseDesign | null }) {
  return <div className="grid gap-4">
    {design ? <DesignSummary design={design} /> : null}
    <div className="grid gap-3">{testCases.map((testCase, index) => {
      const caseId = resolvedTestCaseId(testCase, index)
      const results = expectedResults(testCase)
      return <article key={caseId} className="rounded-lg border border-border bg-card p-3">
        <div className="flex flex-wrap items-start justify-between gap-2"><div className="min-w-0"><span className="text-xs font-semibold text-violet-600">{caseId}</span><strong className="mt-0.5 block text-sm text-foreground">{testCase.title}</strong>{testCase.trace?.length ? <span className="mt-1 block break-words text-xs leading-4 text-muted-foreground"><strong>{testCaseCopy.testCaseTrace}</strong>{testCase.trace.join('、')}</span> : null}</div><div className="flex flex-wrap gap-1.5"><StatusPill tone="accent">{testCaseCopy.testCaseCategory[testCase.category]}</StatusPill><StatusPill tone={testCase.priority === 'P0' ? 'danger' : testCase.priority === 'P1' ? 'warning' : 'neutral'}>{testCase.priority}</StatusPill>{design?.smokeCaseIds.includes(caseId) ? <StatusPill tone="info">{testCaseCopy.testCaseSmoke}</StatusPill> : null}</div></div>
        {testCase.preconditions.length ? <div className="mt-3 text-xs leading-5 text-muted-foreground"><strong className="block text-foreground">{testCaseCopy.testCasePreconditions}</strong><TextList values={testCase.preconditions} /></div> : null}
        {testCase.testData?.length ? <div className="mt-3 text-xs leading-5 text-muted-foreground"><strong className="block text-foreground">{testCaseCopy.testCaseData}</strong><TextList values={testCase.testData} /></div> : null}
        <div className="mt-3 text-xs leading-5 text-muted-foreground"><strong className="block text-foreground">{testCaseCopy.testCaseSteps}</strong><ol className="my-1 pl-5">{testCase.steps.map((step, stepIndex) => <li key={`${step}:${stepIndex}`}>{step}</li>)}</ol></div>
        <div className="mt-2 rounded-md bg-emerald-50 dark:bg-emerald-950/40 px-2.5 py-2 text-xs leading-5 text-emerald-800 dark:text-emerald-300"><strong className="block">{testCaseCopy.testCaseExpected}</strong><ol className="my-1 pl-5">{results.map((result, resultIndex) => <li key={`${result}:${resultIndex}`}>{result}</li>)}</ol></div>
        {testCase.notes?.length ? <div className="mt-2 text-xs leading-5 text-muted-foreground"><strong className="text-foreground">{testCaseCopy.testCaseNotes}</strong><TextList values={testCase.notes} /></div> : null}
        {testCase.evidence.length ? <p className="mt-2 mb-0 break-words text-xs leading-5 text-muted-foreground"><strong>{testCaseCopy.testCaseEvidence}</strong>{testCase.evidence.join('、')}</p> : null}
      </article>
    })}</div>
  </div>
}
