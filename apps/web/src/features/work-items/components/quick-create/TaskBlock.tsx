import type { PMScopeOptions } from '@/shared/api'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import {
  Button,
  Input,
  DatePicker,
  Label,
  PMScopeCascader,
  TableActionButton,
  UserSelect,
} from '@/shared/ui'
import type {
  QuickTaskChildDraft,
  QuickTaskChildField,
  QuickTaskDraft,
  QuickTaskDraftError,
  QuickTaskField,
  WorkItemUserOption,
  WorkItemClientOption,
} from '../../model/types'
import { selectedRequirementOption, buildRequirementOptions } from '../../model/create'
import { WORK_ITEM_PRIORITY_OPTIONS, workItemUserSelectOption } from '../../model/utils'
import { WorkItemsSelect } from '../collection/SelectField'

function errorMessage(error: QuickTaskDraftError | undefined) {
  if (!error) return undefined
  const copy = WORK_ITEMS_COPY.quickCreate.validation
  return {
    title: copy.title,
    requirement: copy.requirement,
    priority: copy.priority,
    assignee: copy.assignee,
    'estimated-hours': copy.estimatedHours,
    'date-range': copy.dateRange,
  }[error]
}

function fieldError(
  errors: Readonly<Record<string, QuickTaskDraftError>>,
  clientId: string,
  ...names: string[]
) {
  return names.map((name) => errors[`${clientId}:${name}`]).find(Boolean)
}

function DraftInput({
  id,
  label,
  value,
  type = 'text',
  error,
  disabled,
  onChange,
}: {
  id: string
  label: string
  value: string
  type?: 'text' | 'date' | 'number'
  error?: QuickTaskDraftError
  disabled: boolean
  onChange: (value: string) => void
}) {
  return (
    <div className="grid min-w-0 gap-1">
      <Label htmlFor={id} className="sr-only">{label}</Label>
      {type === 'date' ? <DatePicker id={id} value={value} placeholder={label} disabled={disabled}
        aria-invalid={Boolean(error) || undefined} aria-describedby={error ? `${id}-error` : undefined}
        onChange={onChange} /> : <Input
        id={id}
        type={type}
        value={value}
        maxLength={type === 'text' ? 220 : undefined}
        min={type === 'number' ? 0 : undefined}
        max={type === 'number' ? 16666 : undefined}
        step={type === 'number' ? 0.5 : undefined}
        placeholder={label}
        aria-invalid={Boolean(error) || undefined}
        aria-describedby={error ? `${id}-error` : undefined}
        disabled={disabled}
        onChange={(event) => onChange(event.currentTarget.value)}
      />}
      {error ? <span id={`${id}-error`} className="sr-only">{errorMessage(error)}</span> : null}
    </div>
  )
}

function ChildRow({
  parentId,
  child,
  index,
  errors,
  disabled,
  users,
  clients,
  onUpdate,
  onRemove,
}: {
  parentId: string
  child: QuickTaskChildDraft
  index: number
  errors: Readonly<Record<string, QuickTaskDraftError>>
  disabled: boolean
  users: readonly WorkItemUserOption[]
  clients: readonly WorkItemClientOption[]
  onUpdate: <TKey extends QuickTaskChildField>(parentId: string, childId: string, field: TKey, value: QuickTaskChildDraft[TKey]) => void
  onRemove: (parentId: string, childId: string) => void
}) {
  const prefix = `work-items-quick-${child.clientId}`
  return (
    <tr className="border-t border-border bg-card align-middle">
      <td className="whitespace-nowrap px-3 py-2 text-center text-xs tabular-nums text-muted-foreground">
        {index + 1}
      </td>
      <td className="whitespace-nowrap px-3 py-2 text-xs font-medium text-muted-foreground">
        {WORK_ITEMS_COPY.quickCreate.childNature}
      </td>
      <td className="min-w-64 px-2 py-2">
        <DraftInput id={`${prefix}-title`} label={WORK_ITEMS_COPY.create.namePlaceholder}
          value={child.title} error={fieldError(errors, child.clientId, 'title')} disabled={disabled}
          onChange={(value) => onUpdate(parentId, child.clientId, 'title', value)} />
      </td>
      <td className="min-w-36 px-2 py-2">
        <WorkItemsSelect id={`${prefix}-priority`} label={WORK_ITEMS_COPY.create.priority}
          value={child.priority} options={WORK_ITEM_PRIORITY_OPTIONS} disabled={disabled}
          onValueChange={(value) => onUpdate(parentId, child.clientId, 'priority', value)} />
      </td>
      <td className="min-w-40 px-2 py-2">
        <WorkItemsSelect id={`${prefix}-client`} label={WORK_ITEMS_COPY.create.client}
          value={child.taskClientId ? String(child.taskClientId) : ''}
          options={clients.map((option) => ({ value: String(option.id), label: option.name }))}
          clearable disabled={disabled}
          onValueChange={(value) => onUpdate(parentId, child.clientId, 'taskClientId', value ? Number(value) : undefined)} />
      </td>
      <td className="min-w-56 px-2 py-2 text-xs text-muted-foreground">
        {WORK_ITEMS_COPY.quickCreate.inheritedRequirement}
      </td>
      <td className="min-w-52 px-2 py-2">
        <UserSelect id={`${prefix}-assignee`} label={WORK_ITEMS_COPY.create.assignee}
          value={child.assigneeId} options={users.map(workItemUserSelectOption)} disabled={disabled}
          placeholder={WORK_ITEMS_COPY.create.assignee}
          aria-invalid={Boolean(fieldError(errors, child.clientId, 'assigneeId', 'assignee_id')) || undefined}
          onValueChange={(value) => { if (value !== undefined) onUpdate(parentId, child.clientId, 'assigneeId', value) }} />
      </td>
      <td className="min-w-40 px-2 py-2"><DraftInput id={`${prefix}-starts`} label={WORK_ITEMS_COPY.create.startsAt}
        type="date" value={child.startsAt} error={fieldError(errors, child.clientId, 'dateRange', 'starts_at')}
        disabled={disabled} onChange={(value) => onUpdate(parentId, child.clientId, 'startsAt', value)} /></td>
      <td className="min-w-40 px-2 py-2"><DraftInput id={`${prefix}-due`} label={WORK_ITEMS_COPY.create.dueAt}
        type="date" value={child.dueAt} error={fieldError(errors, child.clientId, 'dateRange', 'due_at')}
        disabled={disabled} onChange={(value) => onUpdate(parentId, child.clientId, 'dueAt', value)} /></td>
      <td className="min-w-32 px-2 py-2"><DraftInput id={`${prefix}-hours`} label={WORK_ITEMS_COPY.create.estimatedHours}
        type="number" value={child.estimatedHours} error={fieldError(errors, child.clientId, 'estimatedHours', 'estimated_minutes')}
        disabled={disabled} onChange={(value) => onUpdate(parentId, child.clientId, 'estimatedHours', value)} /></td>
      <td className="sticky right-0 z-10 min-w-40 border-l border-border bg-card px-2 py-2 text-center shadow-[var(--design-shadow-edge)]">
        <TableActionButton text={WORK_ITEMS_COPY.quickCreate.remove} label={WORK_ITEMS_COPY.quickCreate.removeChild(index + 1)}
          disabled={disabled} onClick={() => onRemove(parentId, child.clientId)} />
      </td>
    </tr>
  )
}

export function TaskBlock({
  task,
  index,
  errors,
  scopeOptions,
  users,
  clients,
  disabled,
  removable,
  onUpdate,
  onRemove,
  onAddChild,
  onApplyToAll,
  onUpdateChild,
  onRemoveChild,
}: {
  task: QuickTaskDraft
  index: number
  errors: Readonly<Record<string, QuickTaskDraftError>>
  scopeOptions: PMScopeOptions
  users: readonly WorkItemUserOption[]
  clients: readonly WorkItemClientOption[]
  disabled: boolean
  removable: boolean
  onUpdate: <TKey extends QuickTaskField>(clientId: string, field: TKey, value: QuickTaskDraft[TKey]) => void
  onRemove: (clientId: string) => void
  onAddChild: (clientId: string) => void
  onApplyToAll: (clientId: string) => void
  onUpdateChild: <TKey extends QuickTaskChildField>(parentId: string, childId: string, field: TKey, value: QuickTaskChildDraft[TKey]) => void
  onRemoveChild: (parentId: string, childId: string) => void
}) {
  const prefix = `work-items-quick-${task.clientId}`
  const requirement = selectedRequirementOption(task.requirementScope, buildRequirementOptions(scopeOptions))
  return (
    <section className="overflow-hidden rounded-lg border border-border bg-card">
      <header className="flex items-center border-b border-border bg-muted/40 px-3 py-2">
        <h2 className="m-0 text-sm font-semibold">{WORK_ITEMS_COPY.quickCreate.task(index + 1)}</h2>
      </header>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[1680px] border-collapse text-sm">
          <thead className="bg-muted text-left text-xs text-muted-foreground">
            <tr>
              <th className="w-16 whitespace-nowrap px-3 py-2 text-center">{WORK_ITEMS_COPY.quickCreate.sequence}</th>
              <th className="w-20 whitespace-nowrap px-3 py-2">{WORK_ITEMS_COPY.quickCreate.nature}</th>
              <th className="px-2 py-2">{WORK_ITEMS_COPY.create.name}</th>
              <th className="px-2 py-2">{WORK_ITEMS_COPY.create.priority}</th>
              <th className="px-2 py-2">{WORK_ITEMS_COPY.create.client}</th>
              <th className="px-2 py-2">{WORK_ITEMS_COPY.create.requirement}</th>
              <th className="px-2 py-2">{WORK_ITEMS_COPY.create.assignee}</th>
              <th className="px-2 py-2">{WORK_ITEMS_COPY.create.startsAt}</th>
              <th className="px-2 py-2">{WORK_ITEMS_COPY.create.dueAt}</th>
              <th className="px-2 py-2">{WORK_ITEMS_COPY.create.estimatedHours}</th>
              <th className="sticky right-0 z-20 min-w-40 border-l border-border bg-muted px-2 py-2 text-center shadow-[var(--design-shadow-edge)]">{WORK_ITEMS_COPY.detail.childActions}</th>
            </tr>
          </thead>
          <tbody>
            <tr className="align-middle">
              <td className="px-3 py-2" aria-label={WORK_ITEMS_COPY.quickCreate.sequence} />
              <td className="whitespace-nowrap px-3 py-2 text-xs font-medium">{WORK_ITEMS_COPY.quickCreate.taskNature}</td>
              <td className="min-w-64 px-2 py-2"><DraftInput id={`${prefix}-title`} label={WORK_ITEMS_COPY.create.namePlaceholder}
                value={task.title} error={fieldError(errors, task.clientId, 'title')} disabled={disabled}
                onChange={(value) => onUpdate(task.clientId, 'title', value)} /></td>
              <td className="min-w-36 px-2 py-2"><WorkItemsSelect id={`${prefix}-priority`} label={WORK_ITEMS_COPY.create.priority}
                value={task.priority} options={WORK_ITEM_PRIORITY_OPTIONS} disabled={disabled}
                onValueChange={(value) => onUpdate(task.clientId, 'priority', value)} /></td>
              <td className="min-w-40 px-2 py-2"><WorkItemsSelect id={`${prefix}-client`} label={WORK_ITEMS_COPY.create.client}
                value={task.taskClientId ? String(task.taskClientId) : ''}
                options={clients.map((option) => ({ value: String(option.id), label: option.name }))}
                clearable disabled={disabled}
                onValueChange={(value) => onUpdate(task.clientId, 'taskClientId', value ? Number(value) : undefined)} /></td>
              <td className="min-w-64 px-2 py-2">
                <PMScopeCascader idPrefix={`${prefix}-requirement`} state={{ status: 'ready', options: scopeOptions }}
                  value={task.requirementScope} selectionMode="single-requirement" presentation="control"
                  labels={{ label: WORK_ITEMS_COPY.create.requirement, projectSet: WORK_ITEMS_COPY.scope.projectSet,
                    project: WORK_ITEMS_COPY.scope.pmProject, requirement: WORK_ITEMS_COPY.scope.requirement,
                    selectProjectSetFirst: WORK_ITEMS_COPY.scope.selectProjectSetFirst,
                    selectProjectFirst: WORK_ITEMS_COPY.scope.selectProjectFirst }}
                  summary={requirement?.label ?? WORK_ITEMS_COPY.create.requirementPlaceholder}
                  aria-invalid={Boolean(fieldError(errors, task.clientId, 'requirementScope', 'requirement_id')) || undefined}
                  onChange={(value) => onUpdate(task.clientId, 'requirementScope', value)} />
              </td>
              <td className="min-w-52 px-2 py-2"><UserSelect id={`${prefix}-assignee`} label={WORK_ITEMS_COPY.create.assignee}
                value={task.assigneeId} options={users.map(workItemUserSelectOption)} disabled={disabled}
                placeholder={WORK_ITEMS_COPY.create.assignee}
                aria-invalid={Boolean(fieldError(errors, task.clientId, 'assigneeId', 'assignee_id')) || undefined}
                onValueChange={(value) => { if (value !== undefined) onUpdate(task.clientId, 'assigneeId', value) }} /></td>
              <td className="min-w-40 px-2 py-2"><DraftInput id={`${prefix}-starts`} label={WORK_ITEMS_COPY.create.startsAt}
                type="date" value={task.startsAt} error={fieldError(errors, task.clientId, 'dateRange', 'starts_at')}
                disabled={disabled} onChange={(value) => onUpdate(task.clientId, 'startsAt', value)} /></td>
              <td className="min-w-40 px-2 py-2"><DraftInput id={`${prefix}-due`} label={WORK_ITEMS_COPY.create.dueAt}
                type="date" value={task.dueAt} error={fieldError(errors, task.clientId, 'dateRange', 'due_at')}
                disabled={disabled} onChange={(value) => onUpdate(task.clientId, 'dueAt', value)} /></td>
              <td className="min-w-32 px-2 py-2"><DraftInput id={`${prefix}-hours`} label={WORK_ITEMS_COPY.create.estimatedHours}
                type="number" value={task.estimatedHours} error={fieldError(errors, task.clientId, 'estimatedHours', 'estimated_minutes')}
                disabled={disabled} onChange={(value) => onUpdate(task.clientId, 'estimatedHours', value)} /></td>
              <td className="sticky right-0 z-10 min-w-40 border-l border-border bg-card px-2 py-2 shadow-[var(--design-shadow-edge)]">
                <div className="flex items-center justify-center gap-1 whitespace-nowrap">
                  <Button type="button" size="sm" variant="link" className="h-auto px-1 py-1" disabled={disabled} onClick={() => onAddChild(task.clientId)}>
                    {WORK_ITEMS_COPY.quickCreate.addChild}
                  </Button>
                  <TableActionButton text={WORK_ITEMS_COPY.quickCreate.applyToAll} label={WORK_ITEMS_COPY.quickCreate.applyToAllLabel(index + 1)}
                    disabled={disabled || !removable} onClick={() => onApplyToAll(task.clientId)} />
                  <TableActionButton text={WORK_ITEMS_COPY.quickCreate.remove} label={WORK_ITEMS_COPY.quickCreate.removeTask(index + 1)}
                    disabled={disabled || !removable} onClick={() => onRemove(task.clientId)} />
                </div>
              </td>
            </tr>
            {task.children.map((child, childIndex) => (
              <ChildRow key={child.clientId} parentId={task.clientId} child={child} index={childIndex}
                errors={errors} disabled={disabled} users={users}
                clients={clients} onUpdate={onUpdateChild} onRemove={onRemoveChild} />
            ))}
          </tbody>
        </table>
      </div>
    </section>
  )
}
