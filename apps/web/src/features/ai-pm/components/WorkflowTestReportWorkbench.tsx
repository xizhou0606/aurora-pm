import { PhFileText } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import { IconBadge } from '@/shared/ui'
import type { WorkflowInstance } from '../model/types'

const subjectCopy = copy.aiPM.workflowSubject

export function isTestReportWorkflowNode(node?: { type: string; name: string; config?: Record<string, unknown> }) {
  return Boolean(node && (node.type === 'human_task' || node.type === 'form') && (
    node.config?.test_report_artifact === true || node.name.includes('测试验收') || node.name.includes('测试报告')
  ))
}

export function WorkflowTestReportWorkbench({}: {
  workspaceId: number
  instance: Pick<WorkflowInstance, 'id' | 'subject_type' | 'subject_id'>
  node: WorkflowInstance['timeline_nodes'][number]
  readOnly?: boolean
}) {
  return (
    <section className="grid gap-3 rounded-lg border border-border bg-muted/80 p-4">
      <div className="flex min-w-0 items-start gap-2.5">
        <IconBadge icon={PhFileText} tone="blue" />
        <div>
          <strong className="text-sm text-foreground">{subjectCopy.testReportTitle}</strong>
          <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{subjectCopy.testReportDescription}</p>
        </div>
      </div>
      <p className="m-0 text-xs text-muted-foreground">{subjectCopy.testReportRequired}</p>
    </section>
  )
}
