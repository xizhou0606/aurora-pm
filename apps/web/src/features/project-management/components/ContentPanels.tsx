import { copy } from '@/shared/copy'
import { RequirementDocumentField } from '@/features/ai-pm'
import {
  AlertBanner,
  Button,
  ConfirmDialog,
  DateTimePicker,
  EmptyState,
  Input,
  LoadingSkeleton,
  ResponsivePanel,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Textarea,
  UserDirectoryMultiSelect,
  UserDirectorySelect,
} from '@/shared/ui'
import {
  hasStaleRequirementMove,
  CONTENT_PRIORITY_OPTIONS,
  REQUIREMENT_PRIORITY_OPTIONS,
  TASK_STATUS_OPTIONS,
  requirementStatusLabel,
  type ContentDeleteState,
  type ContentTargetState,
  type OwnerDirectoryState,
  type RequirementDraft,
  type RequirementEditorState,
  type RequirementEditableFields,
  type RequirementMoveDraft,
  type RequirementMoveState,
  type TaskDraft,
  type TaskEditorState,
  type TaskUpdateDraft,
} from '../model'
import { WriteFormField } from './WriteFormField'
import { ScheduleMutationNotice } from '@/features/ai-pm'

const contentCopy = copy.projectManagement.content

function TargetSelect({
  id,
  label,
  value,
  state,
  disabled,
  error,
  onChange,
}: {
  id: string
  label: string
  value: number | null
  state: ContentTargetState
  disabled?: boolean
  error?: string
  onChange: (value: number) => void
}) {
  let content: React.ReactNode
  switch (state.status) {
    case 'loading':
      content = <LoadingSkeleton rows={1} label={contentCopy.common.loadingOptions} />
      break
    case 'error':
      content = <EmptyState title={contentCopy.common.optionsFailed} action={<Button type="button" size="sm" variant="outline" onClick={state.retry}>{contentCopy.common.retry}</Button>} />
      break
    case 'empty':
      content = <EmptyState title={contentCopy.common.noOptions} />
      break
    case 'ready':
      content = (
        <Select items={state.items.map((item) => ({ value: String(item.id), label: item.label }))} value={value ? String(value) : null} disabled={disabled} onValueChange={(next) => onChange(Number(next))}>
          <SelectTrigger id={id} aria-invalid={Boolean(error) || undefined} aria-describedby={error ? `${id}-error` : undefined}>
            <SelectValue placeholder={contentCopy.common.noOptions} />
          </SelectTrigger>
          <SelectContent>{state.items.map((item) => <SelectItem key={item.id} value={String(item.id)}>{item.label}</SelectItem>)}</SelectContent>
        </Select>
      )
      break
    }
  return <WriteFormField id={id} label={label} required error={error}>{content}</WriteFormField>
}

function requirementValidationMessage(
  state: Extract<RequirementEditorState, { status: 'editing' }>,
  field: keyof typeof state.validation,
): string | undefined {
  if (!state.validationVisible || !state.validation[field]) return undefined
  switch (field) {
    case 'title': return contentCopy.requirement.titleRequired
    case 'projectId': return contentCopy.requirement.projectRequired
    case 'planUnitId': return contentCopy.requirement.planUnitRequired
    case 'productDocUrl': case 'designUrl': return contentCopy.requirement.invalidUrl
    case 'productOwnerUserId': case 'designOwnerUserId': case 'developerUserIds':
    case 'priority': case 'requirementId': return contentCopy.common.contextChanged
    case 'expectedVersion': return contentCopy.common.contextChanged
    case 'workflowTemplateId': return contentCopy.requirement.workflowTemplateInvalid
  }
}

function requirementSubmitError(editing: Extract<RequirementEditorState, { status: 'editing' }>): string | undefined {
  if (editing.submit.status !== 'error') return undefined
  return editing.submit.errorCode === 'REQUIREMENT_VERSION_CONFLICT' ? contentCopy.requirement.versionConflict : contentCopy.common.saveFailed
}

export function RequirementEditorPanel({
  state,
  projects,
  planUnits,
  workflowTemplates,
  users,
  fields,
  onChange,
  onClose,
  onSubmit,
}: {
  state: RequirementEditorState
  projects: ContentTargetState
  planUnits: ContentTargetState
  workflowTemplates?: ContentTargetState
  users: OwnerDirectoryState
  fields: RequirementEditableFields
  onChange: (draft: RequirementDraft) => void
  onClose: () => void
  onSubmit: () => void
}) {
  const editing = state.status === 'editing' ? state : undefined
  const create = editing?.draft.mode === 'create'
  const pending = editing?.submit.status === 'pending'
  const selectedWfId = editing?.draft.mode === 'create' ? editing.draft.workflowTemplateId : null
  const wfReady = !create || (workflowTemplates?.status === 'ready' && workflowTemplates.items.some((item) => item.id === selectedWfId))
  const submitDisabled = Boolean(pending || !wfReady)
  const titleError = editing ? requirementValidationMessage(editing, 'title') : undefined
  const projectError = editing ? requirementValidationMessage(editing, 'projectId') : undefined
  const planUnitError = editing ? requirementValidationMessage(editing, 'planUnitId') : undefined
  const documentError = editing ? requirementValidationMessage(editing, 'productDocUrl') : undefined
  const designError = editing ? requirementValidationMessage(editing, 'designUrl') : undefined
  const submitError = editing ? requirementSubmitError(editing) : undefined
  return (
    <ResponsivePanel
      open={Boolean(editing)}
      onOpenChange={(open) => { if (!open) onClose() }}
      title={create ? contentCopy.requirement.createTitle : contentCopy.requirement.editTitle}
      description={create ? contentCopy.requirement.createDescription : contentCopy.requirement.editDescription}
      closeLabel={contentCopy.common.cancel}
      pending={pending}
      className="max-w-2xl"
      footer={editing ? <Button type="submit" form="pm-requirement-editor" disabled={submitDisabled}>{pending ? contentCopy.common.saving : contentCopy.common.save}</Button> : undefined}
    >
      {editing ? (
        <form id="pm-requirement-editor" className="grid gap-4" onSubmit={(event) => { event.preventDefault(); if (!submitDisabled) onSubmit() }}>
          {editing.draft.mode === 'create' ? (
            <div className="grid gap-3 sm:grid-cols-2">
              <TargetSelect id="pm-requirement-project" label={contentCopy.requirement.fieldProject} value={editing.draft.projectId} state={projects} disabled={pending} error={projectError} onChange={(projectId) => onChange({ ...editing.draft, projectId, planUnitId: 0 })} />
              <TargetSelect id="pm-requirement-plan-unit" label={contentCopy.requirement.fieldPlanUnit} value={editing.draft.planUnitId} state={planUnits} disabled={pending} error={planUnitError} onChange={(planUnitId) => onChange({ ...editing.draft, planUnitId })} />
            </div>
          ) : null}
          {editing.draft.mode === 'create' ? <WriteFormField id="pm-requirement-workflow" label={contentCopy.requirement.workflowTemplate} required error={requirementValidationMessage(editing, 'workflowTemplateId')}>
            {workflowTemplates?.status === 'ready' ? <Select items={workflowTemplates.items.map((item) => ({ value: String(item.id), label: item.label }))} value={editing.draft.workflowTemplateId ? String(editing.draft.workflowTemplateId) : null} disabled={pending} onValueChange={(value) => { if (editing.draft.mode !== 'create') return; onChange({ ...editing.draft, workflowTemplateId: Number(value) || null }) }}>
              <SelectTrigger id="pm-requirement-workflow" aria-required="true" aria-invalid={Boolean(requirementValidationMessage(editing, 'workflowTemplateId')) || undefined} aria-describedby={requirementValidationMessage(editing, 'workflowTemplateId') ? 'pm-requirement-workflow-error' : undefined}><SelectValue placeholder={contentCopy.requirement.workflowTemplatePlaceholder} /></SelectTrigger>
              <SelectContent>{workflowTemplates.items.map((item) => <SelectItem key={item.id} value={String(item.id)}>{item.label}</SelectItem>)}</SelectContent>
            </Select> : workflowTemplates?.status === 'loading' ? <LoadingSkeleton rows={1} label={contentCopy.requirement.workflowTemplateLoading} /> : workflowTemplates?.status === 'error' ? <div className="rounded-md border border-dashed border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground"><Button type="button" size="xs" variant="outline" onClick={workflowTemplates.retry}>{contentCopy.common.retry}</Button></div> : <div className="rounded-md border border-dashed border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">{contentCopy.requirement.workflowTemplateEmpty}</div>}
            <p role="note" className="text-xs text-muted-foreground">{contentCopy.requirement.workflowTemplateHint}</p>
          </WriteFormField> : null}
          <div className="grid gap-1.5" aria-label={contentCopy.requirement.fieldStatus}>
            <span className="text-sm font-medium">{contentCopy.requirement.fieldStatus}</span>
            <div className="flex min-h-9 flex-wrap items-center gap-x-3 gap-y-1 rounded-md border border-border bg-muted/30 px-3 py-2 text-sm">
              <span className="font-medium">{editing.draft.mode === 'update' ? requirementStatusLabel(editing.draft.businessStatus) : contentCopy.requirement.statusPending}</span>
              <span className="text-xs text-muted-foreground">{contentCopy.requirement.statusWorkflowOwned}</span>
            </div>
          </div>
          <WriteFormField id="pm-requirement-title" label={contentCopy.requirement.fieldTitle} required error={titleError}><Input id="pm-requirement-title" autoFocus value={editing.draft.title} required disabled={pending || !fields.content} aria-invalid={Boolean(titleError) || undefined} aria-describedby={titleError ? 'pm-requirement-title-error' : undefined} placeholder={contentCopy.requirement.titlePlaceholder} onChange={(event) => onChange({ ...editing.draft, title: event.target.value })} /></WriteFormField>
          <WriteFormField id="pm-requirement-description" label={contentCopy.requirement.fieldDescription}><Textarea id="pm-requirement-description" value={editing.draft.description} disabled={pending || !fields.content} placeholder={contentCopy.requirement.descriptionPlaceholder} onChange={(event) => onChange({ ...editing.draft, description: event.target.value })} /></WriteFormField>
          <RequirementDocumentField id="pm-requirement-document" error={documentError} key={editing.draft.mode === 'update' ? editing.draft.requirementId : 'new'} requirementId={editing.draft.mode === 'update' ? editing.draft.requirementId : undefined} projectId={editing.draft.projectId} value={editing.draft.documentChoice} url={editing.draft.productDocUrl} disabled={pending || !fields.content} onChange={value => onChange({ ...editing.draft, documentChoice: value, productDocUrl: value.type === 'feishu' ? value.url : editing.draft.productDocUrl })} />
          <UserDirectorySelect id="pm-requirement-owner" label={contentCopy.requirement.fieldProductOwner} value={editing.draft.productOwnerUserId} state={users} disabled={pending || !fields.owner} onValueChange={(value) => onChange({ ...editing.draft, productOwnerUserId: value ?? null })} />
          <WriteFormField id="pm-requirement-design" label={contentCopy.requirement.fieldDesign} error={designError}><Input id="pm-requirement-design" type="url" value={editing.draft.designUrl} disabled={pending || !fields.design} aria-invalid={Boolean(designError) || undefined} aria-describedby={designError ? 'pm-requirement-design-error' : undefined} placeholder={contentCopy.requirement.urlPlaceholder} onChange={(event) => onChange({ ...editing.draft, designUrl: event.target.value })} /></WriteFormField>
          <UserDirectorySelect id="pm-requirement-design-owner" label={contentCopy.requirement.fieldDesignOwner} value={editing.draft.designOwnerUserId} state={users} disabled={pending || !fields.design} onValueChange={(value) => onChange({ ...editing.draft, designOwnerUserId: value ?? null })} />
          <UserDirectoryMultiSelect id="pm-requirement-developers" label={contentCopy.requirement.fieldDevelopers} description={contentCopy.requirement.developersDescription} placeholder={contentCopy.requirement.developersPlaceholder} value={editing.draft.developerUserIds} state={users} disabled={pending || !fields.governance} onValueChange={(value) => onChange({ ...editing.draft, developerUserIds: value })} />
          <WriteFormField id="pm-requirement-acceptance" label={contentCopy.requirement.fieldAcceptance}><Textarea id="pm-requirement-acceptance" value={editing.draft.acceptanceCriteria} disabled={pending || !fields.acceptance} placeholder={contentCopy.requirement.acceptancePlaceholder} onChange={(event) => onChange({ ...editing.draft, acceptanceCriteria: event.target.value })} /></WriteFormField>
          <WriteFormField id="pm-requirement-priority" label={contentCopy.requirement.fieldPriority}><Select items={REQUIREMENT_PRIORITY_OPTIONS} value={editing.draft.priority} disabled={pending || !fields.priority} onValueChange={(value) => onChange({ ...editing.draft, priority: String(value) })}><SelectTrigger id="pm-requirement-priority"><SelectValue /></SelectTrigger><SelectContent>{CONTENT_PRIORITY_OPTIONS.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}</SelectContent></Select></WriteFormField>
          {submitError ? <AlertBanner tone="error">{submitError}</AlertBanner> : null}
        </form>
      ) : null}
    </ResponsivePanel>
  )
}

function taskValidationMessage(
  state: Extract<TaskEditorState, { status: 'editing' }>,
  field: keyof typeof state.validation,
): string | undefined {
  if (!state.validationVisible || !state.validation[field]) return undefined
  switch (field) {
    case 'title': return contentCopy.task.titleRequired
    case 'requirementId': return contentCopy.task.requirementRequired
    case 'startsAt':
    case 'dueAt': return contentCopy.task.dateOrder
    case 'estimatedHours': return contentCopy.task.estimateInvalid
    case 'projectId':
    case 'planUnitId':
    case 'taskId':
    case 'assigneeId':
    case 'priority':
    case 'status': return contentCopy.common.contextChanged
  }
}

export function TaskEditorPanel({ state, users, onChange, onClose, onSubmit }: {
  state: TaskEditorState
  users: OwnerDirectoryState
  onChange: (draft: TaskDraft) => void
  onClose: () => void
  onSubmit: () => void
}) {
  const editing = state.status === 'editing' ? state : undefined
  const create = editing?.draft.mode === 'create'
  const pending = editing?.submit.status === 'pending'
  const titleError = editing ? taskValidationMessage(editing, 'title') : undefined
  const startError = editing ? taskValidationMessage(editing, 'startsAt') : undefined
  const dueError = editing ? taskValidationMessage(editing, 'dueAt') : undefined
  const estimateError = editing ? taskValidationMessage(editing, 'estimatedHours') : undefined
  return (
    <ResponsivePanel open={Boolean(editing)} onOpenChange={(open) => { if (!open) onClose() }} title={create ? contentCopy.task.createTitle : contentCopy.task.editTitle} description={create ? contentCopy.task.createDescription : contentCopy.task.editDescription} closeLabel={contentCopy.common.cancel} pending={pending} className="max-w-lg" footer={editing ? <Button type="submit" form="pm-task-editor" disabled={pending}>{pending ? contentCopy.common.saving : contentCopy.common.save}</Button> : undefined}>
      {editing ? <form id="pm-task-editor" className="grid gap-4" onSubmit={(event) => { event.preventDefault(); onSubmit() }}>
        <WriteFormField id="pm-task-title" label={contentCopy.task.fieldTitle} required error={titleError}><Input id="pm-task-title" autoFocus required value={editing.draft.title} disabled={pending} aria-invalid={Boolean(titleError) || undefined} aria-describedby={titleError ? 'pm-task-title-error' : undefined} placeholder={contentCopy.task.titlePlaceholder} onChange={(event) => onChange({ ...editing.draft, title: event.target.value })} /></WriteFormField>
        <WriteFormField id="pm-task-description" label={contentCopy.task.fieldDescription}><Textarea id="pm-task-description" value={editing.draft.description} disabled={pending} placeholder={contentCopy.task.descriptionPlaceholder} onChange={(event) => onChange({ ...editing.draft, description: event.target.value })} /></WriteFormField>
        <UserDirectorySelect id="pm-task-assignee" label={contentCopy.task.fieldAssignee} value={editing.draft.assigneeId} state={users} disabled={pending} onValueChange={(value) => onChange({ ...editing.draft, assigneeId: value ?? null })} />
        <WriteFormField id="pm-task-priority" label={contentCopy.task.fieldPriority}><Select items={CONTENT_PRIORITY_OPTIONS} value={editing.draft.priority} disabled={pending} onValueChange={(value) => onChange({ ...editing.draft, priority: String(value) })}><SelectTrigger id="pm-task-priority"><SelectValue /></SelectTrigger><SelectContent>{CONTENT_PRIORITY_OPTIONS.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}</SelectContent></Select></WriteFormField>
        <div className="grid gap-3 sm:grid-cols-2"><WriteFormField id="pm-task-start" label={contentCopy.task.fieldStart} error={startError}><DateTimePicker id="pm-task-start" value={editing.draft.startsAt} disabled={pending} aria-invalid={Boolean(startError) || undefined} aria-describedby={startError ? 'pm-task-start-error' : undefined} onChange={(value) => onChange({ ...editing.draft, startsAt: value })} /></WriteFormField><WriteFormField id="pm-task-due" label={contentCopy.task.fieldDue} error={dueError}><DateTimePicker id="pm-task-due" value={editing.draft.dueAt} defaultMonth={editing.draft.startsAt} disabled={pending} aria-invalid={Boolean(dueError) || undefined} aria-describedby={dueError ? 'pm-task-due-error' : undefined} onChange={(value) => onChange({ ...editing.draft, dueAt: value })} /></WriteFormField></div>
        <WriteFormField id="pm-task-estimate" label={contentCopy.task.fieldEstimate} error={estimateError}><Input id="pm-task-estimate" type="number" min="0" max="1000" step="0.5" value={editing.draft.estimatedHours} disabled={pending} aria-invalid={Boolean(estimateError) || undefined} aria-describedby={estimateError ? 'pm-task-estimate-error' : undefined} onChange={(event) => onChange({ ...editing.draft, estimatedHours: event.target.value })} /></WriteFormField>
        {editing.draft.mode === 'update' ? <WriteFormField id="pm-task-status" label={contentCopy.task.fieldStatus}><Select items={TASK_STATUS_OPTIONS} value={editing.draft.status} disabled={pending} onValueChange={(value) => onChange({ ...(editing.draft as TaskUpdateDraft), status: String(value) })}><SelectTrigger id="pm-task-status"><SelectValue /></SelectTrigger><SelectContent>{TASK_STATUS_OPTIONS.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}</SelectContent></Select></WriteFormField> : null}
        {editing.submit.status === 'error' ? <AlertBanner tone="error">{contentCopy.common.saveFailed}</AlertBanner> : null}
      </form> : null}
    </ResponsivePanel>
  )
}

function resolveSubjectName(
  selecting: Extract<RequirementMoveState, { status: 'selecting' }> | undefined,
  projects: ContentTargetState,
  planUnits: ContentTargetState,
  type: 'project_set' | 'plan_unit' | 'requirement' | undefined,
  id?: number,
) {
  if (type === 'requirement' && selecting && id === selecting.draft.requirementId) return selecting.draft.title
  const src = type === 'project_set' ? projects : type === 'plan_unit' ? planUnits : undefined
  return src?.status === 'ready' ? src.items.find((item) => item.id === id)?.label : undefined
}

export function RequirementMovePanel({ state, projects, planUnits, onChange, onClose, onSubmit, onReloadLatest, onConfirmSchedule }: {
  state: RequirementMoveState
  projects: ContentTargetState
  planUnits: ContentTargetState
  onChange: (draft: RequirementMoveDraft) => void
  onClose: () => void
  onSubmit: () => void
  onReloadLatest?: () => void
  onConfirmSchedule?: () => void
}) {
  const selecting = state.status === 'selecting' ? state : undefined
  const pending = selecting?.submit.status === 'pending'
  const schedule = selecting?.submit.status === 'error' ? selecting.submit.schedule : undefined
  const stale = hasStaleRequirementMove(selecting?.submit)
  const blocked = stale || (selecting?.submit.status === 'error' && selecting.submit.errorCode === 'RESOURCE_CHANGED')
  const error = selecting?.submit.status === 'error' && !schedule
    ? hasStaleRequirementMove(selecting.submit) ? contentCopy.requirement.moveStale : selecting.submit.errorCode === 'REQUIREMENT_VERSION_CONFLICT' ? contentCopy.requirement.moveConflict : selecting.submit.errorCode === 'RESOURCE_CHANGED' ? contentCopy.common.contextChanged : contentCopy.common.moveFailed
    : undefined
  return (
    <ResponsivePanel open={Boolean(selecting)} onOpenChange={(open) => { if (!open) onClose() }} title={contentCopy.requirement.moveTitle} description={selecting ? contentCopy.requirement.moveDescription(selecting.draft.title) : contentCopy.requirement.moveTitle} closeLabel={contentCopy.common.cancel} pending={pending} className="max-w-lg" footer={selecting ? <Button type="button" disabled={pending || blocked || !selecting.draft.targetPlanUnitId} onClick={onSubmit}>{pending ? contentCopy.common.saving : contentCopy.common.move}</Button> : undefined}>
      {selecting ? (
        <div className="grid min-w-0 gap-4">
          <TargetSelect id="pm-requirement-move-project" label={contentCopy.requirement.fieldProject} value={selecting.draft.targetProjectId} state={projects} disabled={pending || blocked} onChange={(targetProjectId) => onChange({ ...selecting.draft, targetProjectId, targetPlanUnitId: null })} />
          <TargetSelect id="pm-requirement-move-plan-unit" label={contentCopy.requirement.moveTarget} value={selecting.draft.targetPlanUnitId} state={planUnits} disabled={pending || blocked} onChange={(targetPlanUnitId) => onChange({ ...selecting.draft, targetPlanUnitId })} />
          {schedule ? <ScheduleMutationNotice code={selecting.submit.status === 'error' ? selecting.submit.errorCode : undefined} schedule={schedule} pending={pending} resolveSubjectName={(type, id) => resolveSubjectName(selecting, projects, planUnits, type, id)} onAdjust={() => onChange(selecting.draft)} onConfirm={onConfirmSchedule} /> : null}
          {error ? <AlertBanner tone="error" action={stale && onReloadLatest ? <Button type="button" variant="outline" onClick={onReloadLatest}>{contentCopy.requirement.moveReloadLatest}</Button> : undefined}>{error}</AlertBanner> : null}
        </div>
      ) : null}
    </ResponsivePanel>
  )
}

export function ContentDeleteConfirm({ state, onClose, onConfirm }: {
  state: ContentDeleteState
  onClose: () => void
  onConfirm: () => void
}) {
  const confirming = state.status === 'confirming' ? state : undefined
  const requirement = confirming?.target.kind === 'requirement'
  const pending = confirming?.submit.status === 'pending'
  const error = confirming?.submit.status === 'error' ? contentCopy.common.deleteFailed : undefined
  return <ConfirmDialog open={Boolean(confirming)} onOpenChange={(open) => { if (!open) onClose() }} onConfirm={onConfirm} title={requirement ? contentCopy.requirement.deleteTitle : contentCopy.task.deleteTitle} description={requirement ? contentCopy.requirement.deleteDescription(confirming?.target.title ?? '') : contentCopy.task.deleteDescription(confirming?.target.title ?? '')} cancelLabel={contentCopy.common.cancel} confirmLabel={pending ? contentCopy.common.deleting : requirement ? contentCopy.requirement.confirmDelete : contentCopy.task.confirmDelete} pending={pending} error={error} />
}
