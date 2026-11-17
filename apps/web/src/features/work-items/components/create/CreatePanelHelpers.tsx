import { WORK_ITEMS_COPY } from '@/shared/copy'
export { FormField as Field } from '@/shared/ui'
import type { WorkItemCreateValidation } from '../../model/types'

export function validationMessage(value: WorkItemCreateValidation | undefined): string | undefined {
  if (!value) return undefined
  const messages: Record<WorkItemCreateValidation, string> = {
    workspace: WORK_ITEMS_COPY.create.validation.workspace,
    requirement: WORK_ITEMS_COPY.create.validation.requirement,
    title: WORK_ITEMS_COPY.create.validation.title,
    status: WORK_ITEMS_COPY.create.validation.status,
    priority: WORK_ITEMS_COPY.create.validation.priority,
    client: WORK_ITEMS_COPY.create.validation.client,
    assignee: WORK_ITEMS_COPY.create.validation.assignee,
    'estimated-hours': WORK_ITEMS_COPY.create.validation.estimatedHours,
    'date-range': WORK_ITEMS_COPY.create.validation.dateRange,
  }
  return messages[value]
}
