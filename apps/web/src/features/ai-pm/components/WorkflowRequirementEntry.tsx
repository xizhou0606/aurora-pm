import { PhFileText } from '@/shared/ui/icons'

import { REQUIREMENT_DOCUMENTS_COPY as subjectCopy } from '@/shared/copy/requirement-documents'
import { IconBadge } from '@/shared/ui'
import type { Requirement } from '../model/types'
import { RequirementDocuments } from './RequirementDocuments'


export function isRequirementInputWorkflowNode(node?: { type: string; name: string; config?: Record<string, unknown> }) {
  return Boolean(node && (node.type === 'human_task' || node.type === 'form') && (
    node.config?.requirement_input === true || node.name.includes('需求录入') || node.name.includes('需求收集')
  ))
}

export function WorkflowRequirementEntry({ workspaceId, requirement, readOnly = false }: { workspaceId: number; requirement: Requirement; readOnly?: boolean }) {
  return <section className="grid gap-4 rounded-lg border border-blue-100 dark:border-blue-900 bg-blue-50/30 dark:bg-blue-950/25 p-4" aria-labelledby="workflow-requirement-entry-title">
    <header className="flex items-start gap-3">
      <IconBadge icon={PhFileText} tone="blue" size="lg" />
      <div><h3 id="workflow-requirement-entry-title" className="m-0 text-sm font-semibold text-foreground">{subjectCopy.entryTitle}</h3><p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{subjectCopy.entryDescription}</p></div>
    </header>
    <RequirementDocuments workspaceId={workspaceId} requirementId={requirement.id} readOnly={readOnly} />
  </section>
}
