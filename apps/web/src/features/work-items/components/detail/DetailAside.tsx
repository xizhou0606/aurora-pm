import { workItemStatusTone } from '@/shared/model'
import { DatePicker } from '@/shared/ui'
import {
  Input,
  Label,
  MetadataItem,
  MetadataList,
  MetadataRail,
  StatusPill,
  UserPreview,
  UserSelect,
} from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { workItemPriorityTone } from '@/shared/model'
import { shouldUseClientNavigation } from '@/shared/navigation'

import type {
  ResourceState,
  UserOptionsState,
  WorkItem,
  WorkItemRelated,
  WorkItemStatusOption,
  WorkItemClientOption,
} from '../../model/types'
import type { WorkItemDetailDraft } from '../../hooks'
import {
  formatDateTime,
  priorityLabel,
  resolveWorkItemAssignee,
  statusLabel,
  workItemRequirementPath,
  workItemUserSelectOption,
} from '../../model/utils'
import { WorkItemsSelect } from '../collection/SelectField'

export function DetailAside({
  item,
  draft,
  editable,
  statusEditable = false,
  saveState,
  users,
  clients = [],
  workflow,
  onOpenRequirement,
  parent,
  onOpenParent,
  onChange,
}: {
  item: WorkItem
  draft: WorkItemDetailDraft
  editable: boolean
  statusEditable?: boolean
  saveState: 'idle' | 'pending' | 'error'
  users?: UserOptionsState
  clients?: readonly WorkItemClientOption[]
  workflow: ResourceState<readonly WorkItemStatusOption[]>
  onOpenRequirement?: (item: WorkItem) => void
  parent?: WorkItemRelated
  onOpenParent?: (id: number) => void
  onChange: <TKey extends keyof WorkItemDetailDraft>(
    key: TKey,
    value: WorkItemDetailDraft[TKey],
  ) => void
}) {
  const pending = saveState === 'pending'
  const rawUserOptions = users?.status === 'ready' ? users.data : []
  const assignee = resolveWorkItemAssignee(item, rawUserOptions)
  const reporter = rawUserOptions.find((option) => (
    option.id === item.reporterId
    || option.label === item.reporterName
    || option.username === item.reporterName
  ))
  const requirementLabel = item.requirementTitle || WORK_ITEMS_COPY.list.noRequirement
  const requirementPath = workItemRequirementPath(item)
  const requirementLink = requirementPath ? (
    <a
      href={requirementPath}
      className="text-primary underline-offset-4 hover:underline focus-visible:underline"
      onClick={(event) => {
        if (!onOpenRequirement || !shouldUseClientNavigation(event)) return
        event.preventDefault()
        onOpenRequirement(item)
      }}
    >
      {requirementLabel}
    </a>
  ) : requirementLabel
  const parentLabel = parent?.title || (item.parentId
    ? WORK_ITEMS_COPY.list.fallbackId(item.parentId)
    : WORK_ITEMS_COPY.enums.fallback)
  const parentLink = item.parentId && parent ? (
    <a
      href={`/workspace/work-items?task_id=${parent.id}`}
      className="text-primary underline-offset-4 hover:underline focus-visible:underline"
      onClick={(event) => {
        if (!onOpenParent || !shouldUseClientNavigation(event)) return
        event.preventDefault()
        onOpenParent(parent.id)
      }}
    >
      {parentLabel}
    </a>
  ) : parentLabel
  const metadata = [
    { label: WORK_ITEMS_COPY.detail.requirement, value: requirementLink },
    { label: WORK_ITEMS_COPY.detail.parent, value: parentLink },
    { label: WORK_ITEMS_COPY.detail.createdAt, value: formatDateTime(item.createdAt) || WORK_ITEMS_COPY.enums.fallback },
    { label: WORK_ITEMS_COPY.detail.updatedAt, value: formatDateTime(item.updatedAt) || WORK_ITEMS_COPY.enums.fallback },
  ]
  const statusOptions = workflow.status === 'ready'
    ? [
        ...(!workflow.data.some((option) => option.value === draft.status) && draft.status
          ? [{ value: draft.status, label: statusLabel(draft.status) }]
          : []),
        ...workflow.data,
      ]
    : []
  const userOptions = rawUserOptions.map(workItemUserSelectOption)
  const clientName = clients.find(({ id }) => id === draft.taskClientId)?.name || WORK_ITEMS_COPY.list.noClient

  return (
    <MetadataRail title={WORK_ITEMS_COPY.detail.metadataTitle}>
      {editable || statusEditable ? <div className="grid gap-3">
        <div className="grid gap-1.5">
          <Label htmlFor="work-items-detail-status">{WORK_ITEMS_COPY.detail.status}</Label>
          {statusOptions.length ? (
            <WorkItemsSelect
              id="work-items-detail-status"
              label={WORK_ITEMS_COPY.detail.status}
              value={draft.status}
              options={statusOptions}
              disabled={pending || !statusEditable}
              onValueChange={(value) => onChange('status', value)}
            />
          ) : <span>{statusLabel(draft.status)}</span>}
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="work-items-detail-priority">{WORK_ITEMS_COPY.detail.priority}</Label>
          <WorkItemsSelect
            id="work-items-detail-priority"
            label={WORK_ITEMS_COPY.detail.priority}
            value={draft.priority}
            options={Object.keys(WORK_ITEMS_COPY.enums.priorities).map((priority) => ({
              value: priority,
              label: priorityLabel(priority),
            }))}
            disabled={pending || !editable}
            onValueChange={(value) => onChange('priority', value)}
          />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="work-items-detail-assignee">{WORK_ITEMS_COPY.detail.assignee}</Label>
          {users?.status === 'ready' ? (
            <UserSelect
              id="work-items-detail-assignee"
              label={WORK_ITEMS_COPY.detail.assignee}
              value={draft.assigneeId}
              options={userOptions}
              placeholder={WORK_ITEMS_COPY.list.unassigned}
              disabled={pending || !editable}
              onValueChange={(value) => {
                if (value !== undefined) onChange('assigneeId', value)
              }}
            />
          ) : <span>{assignee.name}</span>}
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="work-items-detail-client">{WORK_ITEMS_COPY.detail.client}</Label>
          <WorkItemsSelect
            id="work-items-detail-client"
            label={WORK_ITEMS_COPY.detail.client}
            value={draft.taskClientId ? String(draft.taskClientId) : ''}
            options={clients.map(({ id, name }) => ({ value: String(id), label: name }))}
            clearable
            disabled={pending || !editable}
            onValueChange={(value) => onChange('taskClientId', value ? Number(value) : undefined)}
          />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="work-items-detail-start">{WORK_ITEMS_COPY.gantt.startDate}</Label>
          <DatePicker id="work-items-detail-start"  value={draft.startsAt} disabled={pending || !editable} onChange={(value) => onChange('startsAt', value)} />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="work-items-detail-due">{WORK_ITEMS_COPY.gantt.dueDate}</Label>
          <DatePicker id="work-items-detail-due"  value={draft.dueAt} disabled={pending || !editable} onChange={(value) => onChange('dueAt', value)} />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="work-items-detail-estimated-minutes">{WORK_ITEMS_COPY.detail.estimatedTime}</Label>
          <Input
            id="work-items-detail-estimated-minutes"
            type="number"
            min={0}
            max={999_960}
            value={draft.estimatedMinutes}
            disabled={pending || !editable}
            onChange={(event) => onChange('estimatedMinutes', event.currentTarget.value)}
          />
        </div>
        <MetadataList className="items-center gap-y-3 [&_[data-slot=user-preview]_span]:text-sm [&_dd]:leading-6">
          <MetadataItem label={WORK_ITEMS_COPY.detail.reporter}>
            <UserPreview
              name={reporter?.label || item.reporterName || WORK_ITEMS_COPY.enums.fallback}
              secondary={reporter?.username}
              avatarUrl={reporter?.avatarUrl}
              size="sm"
            />
          </MetadataItem>
          {metadata.map((entry) => (
            <MetadataItem key={entry.label} label={entry.label}>{entry.value}</MetadataItem>
          ))}
        </MetadataList>
      </div> : (
        <MetadataList className="items-center gap-y-3 [&_[data-slot=user-preview]_span]:text-sm [&_dd]:leading-6">
          <MetadataItem label={WORK_ITEMS_COPY.detail.status}><StatusPill tone={workItemStatusTone(draft.status)}>{statusLabel(draft.status)}</StatusPill></MetadataItem>
          <MetadataItem label={WORK_ITEMS_COPY.detail.priority}>
            <StatusPill tone={workItemPriorityTone(draft.priority)} className="w-fit min-w-0">
              {priorityLabel(draft.priority)}
            </StatusPill>
          </MetadataItem>
          <MetadataItem label={WORK_ITEMS_COPY.detail.assignee}>
            <UserPreview
              name={assignee.name}
              secondary={assignee.user?.username}
              avatarUrl={assignee.user?.avatarUrl}
              size="sm"
            />
          </MetadataItem>
          <MetadataItem label={WORK_ITEMS_COPY.detail.client}>{clientName}</MetadataItem>
          <MetadataItem label={WORK_ITEMS_COPY.detail.period}>
            {WORK_ITEMS_COPY.format.dateRange(draft.startsAt, draft.dueAt)}
          </MetadataItem>
          <MetadataItem label={WORK_ITEMS_COPY.detail.reporter}>
            <UserPreview
              name={reporter?.label || item.reporterName || WORK_ITEMS_COPY.enums.fallback}
              secondary={reporter?.username}
              avatarUrl={reporter?.avatarUrl}
              size="sm"
            />
          </MetadataItem>
          {metadata.map((entry) => (
            <MetadataItem key={entry.label} label={entry.label}>{entry.value}</MetadataItem>
          ))}
        </MetadataList>
      )}
    </MetadataRail>
  )
}
