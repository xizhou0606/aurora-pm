import { copy } from '@/shared/copy'
import type { PMSafeScheduleConflict } from '@/shared/api'
import {
  AlertBanner,
  Button,
  DatePicker,
  Input,
  ResponsivePanel,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Textarea,
  UserDirectorySelect,
} from '@/shared/ui'
import type {
  OwnerDirectoryState,
  DeliveryLabelManagerState,
  PlanUnitDraft,
  PlanUnitEditorState,
} from '../model'
import type { PublishedPlanUnitWorkflowTemplate } from '../api'
import {
  PLAN_UNIT_RISK_OPTIONS,
  PLAN_UNIT_TYPE_OPTIONS,
  hasStaleSchedule,
  projectItemTypeLabel,
  statusLabel,
} from '../model'
import { LabelManager, type LabelManagerControl } from './delivery'
import {
  PlanUnitDeliveryFields,
  type PlanUnitDeliveryControl,
} from './PlanUnitDeliveryFields'
import { WriteFormField } from './WriteFormField'
import { ScheduleMutationNotice } from '@/features/ai-pm'

const writeCopy = copy.projectManagement.write

export type PlanUnitEditorControl = {
  owners: OwnerDirectoryState
  workflowTemplates:
    | { status: 'loading' | 'empty' }
    | { status: 'error'; retry: () => void }
    | { status: 'ready'; items: PublishedPlanUnitWorkflowTemplate[] }
  deliveryBusy?: boolean
  scheduleReloading?: boolean
  resolveScheduleSubjectName?: (type: PMSafeScheduleConflict['subject_type'], id?: number) => string | undefined
  delivery: Omit<PlanUnitDeliveryControl, 'owners'>
  labelManager: {
    state: DeliveryLabelManagerState
    targetTitle?: string
    control: LabelManagerControl
  }
  onChange: (draft: PlanUnitDraft) => void
  onReloadSchedule?: () => void
  onClose: () => void
  onSubmit: () => void
}

function validationMessage(
  state: Extract<PlanUnitEditorState, { status: 'editing' }>,
  field: keyof typeof state.validation,
): string | undefined {
  if (!state.validationVisible || !state.validation[field]) return undefined
  switch (field) {
    case 'title': return writeCopy.validation.titleRequired
    case 'workflowTemplateId': return writeCopy.validation.workflowTemplateRequired
    case 'type': return writeCopy.validation.typeInvalid
    case 'ownerUserId': return writeCopy.validation.ownerInvalid
    case 'plannedStartAt':
    case 'plannedEndAt':
    case 'actualStartAt':
    case 'actualCompletedAt': return writeCopy.validation.dateOrder
    case 'riskLevel': return writeCopy.validation.riskInvalid
    case 'projectId':
    case 'planUnitId': return writeCopy.validation.contextChanged
  }
}

export function PlanUnitEditorPanel({ state, control }: {
  state: PlanUnitEditorState
  control: PlanUnitEditorControl
}) {
  const editing = state.status === 'editing' ? state : undefined
  const createDraft = editing?.draft.mode === 'create' ? editing.draft : undefined
  const create = Boolean(createDraft)
  const itemType = projectItemTypeLabel(editing?.draft.type)
  const pending = editing?.submit.status === 'pending'
  const scheduleStale = editing ? hasStaleSchedule(editing.submit) : false
  const busy = Boolean(pending || control.scheduleReloading || (!create && control.deliveryBusy))
  const locked = Boolean(busy || scheduleStale)
  const workflowTemplateReady = !create || (
    control.workflowTemplates.status === 'ready'
    && control.workflowTemplates.items.some((item) => item.id === createDraft?.workflowTemplateId)
  )
  const submitDisabled = Boolean(
    locked
    || !workflowTemplateReady
    || (create && !createDraft?.workflowTemplateId)
    || (!create && editing?.deliveryCatalog.status !== 'ready'),
  )
  const scheduleConflict = editing?.submit.status === 'error' ? editing.submit.schedule : undefined
  const submitError = editing?.submit.status === 'error' && !scheduleConflict ? writeCopy.feedback.saveFailed : undefined
  return (
    <>
    <ResponsivePanel
      open={Boolean(editing)}
      onOpenChange={(open) => { if (!open) control.onClose() }}
      title={create ? writeCopy.planUnitForm.createTitle : writeCopy.planUnitForm.editTitle(itemType)}
      description={create ? writeCopy.planUnitForm.createDescription : writeCopy.planUnitForm.editDescription(itemType)}
      closeLabel={writeCopy.actions.cancel}
      pending={busy}
      className="max-w-2xl"
      footer={editing ? <Button type="submit" form="pm-plan-unit-editor-form" disabled={submitDisabled}>{busy ? writeCopy.actions.saving : writeCopy.actions.save}</Button> : undefined}
    >
      {editing ? (
        <form id="pm-plan-unit-editor-form" className="grid gap-4" onSubmit={(event) => {
          event.preventDefault()
          if (!submitDisabled) control.onSubmit()
        }}>
          <WriteFormField id="pm-plan-unit-title" label={writeCopy.planUnitForm.title} required error={validationMessage(editing, 'title')}>
            <Input id="pm-plan-unit-title" autoFocus value={editing.draft.title} required disabled={locked} aria-invalid={Boolean(validationMessage(editing, 'title')) || undefined} aria-describedby={validationMessage(editing, 'title') ? 'pm-plan-unit-title-error' : undefined} placeholder={writeCopy.planUnitForm.titlePlaceholder} onChange={(event) => control.onChange({ ...editing.draft, title: event.target.value })} />
          </WriteFormField>
          <WriteFormField id="pm-plan-unit-type" label={writeCopy.planUnitForm.type} required error={validationMessage(editing, 'type')}>
            <Select items={PLAN_UNIT_TYPE_OPTIONS} value={editing.draft.type} disabled={locked} onValueChange={(value) => control.onChange({ ...editing.draft, type: String(value) })}>
              <SelectTrigger id="pm-plan-unit-type" aria-required="true" aria-invalid={Boolean(validationMessage(editing, 'type')) || undefined} aria-describedby={validationMessage(editing, 'type') ? 'pm-plan-unit-type-error' : undefined}><SelectValue /></SelectTrigger>
              <SelectContent>
                {PLAN_UNIT_TYPE_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </WriteFormField>
          {create ? <WriteFormField id="pm-plan-unit-workflow" label={writeCopy.planUnitForm.workflowTemplate} required error={validationMessage(editing, 'workflowTemplateId')}>
            {control.workflowTemplates.status === 'ready' ? <Select items={control.workflowTemplates.items.map((item) => ({ value: String(item.id), label: writeCopy.planUnitForm.workflowTemplateOption(item.name, item.published_version) }))} value={createDraft?.workflowTemplateId ? String(createDraft.workflowTemplateId) : ''} disabled={locked} onValueChange={(value) => { if (createDraft) control.onChange({ ...createDraft, workflowTemplateId: Number(value) || null }) }}>
              <SelectTrigger id="pm-plan-unit-workflow" aria-required="true" aria-invalid={Boolean(validationMessage(editing, 'workflowTemplateId')) || undefined}><SelectValue placeholder={writeCopy.planUnitForm.workflowTemplatePlaceholder} /></SelectTrigger>
              <SelectContent>{control.workflowTemplates.items.map((item) => <SelectItem key={item.id} value={String(item.id)}>{writeCopy.planUnitForm.workflowTemplateOption(item.name, item.published_version)}</SelectItem>)}</SelectContent>
            </Select> : <div className="rounded-md border border-dashed border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">{control.workflowTemplates.status === 'loading' ? writeCopy.planUnitForm.workflowTemplateLoading : control.workflowTemplates.status === 'error' ? <Button type="button" size="xs" variant="outline" onClick={control.workflowTemplates.retry}>{writeCopy.planUnitForm.workflowTemplateRetry}</Button> : writeCopy.planUnitForm.workflowTemplateEmpty}</div>}
            <p role="note" className="text-xs text-muted-foreground">{writeCopy.planUnitForm.workflowTemplateHint}</p>
          </WriteFormField> : null}
          <UserDirectorySelect id="pm-plan-unit-owner" label={writeCopy.planUnitForm.owner} value={editing.draft.ownerUserId} disabled={locked} state={control.owners} error={validationMessage(editing, 'ownerUserId')} onValueChange={(value) => control.onChange({ ...editing.draft, ownerUserId: value ?? null })} />
          <div className="grid gap-3 sm:grid-cols-2">
            <WriteFormField id="pm-plan-unit-start" label={writeCopy.planUnitForm.plannedStart} error={validationMessage(editing, 'plannedStartAt')}><DatePicker id="pm-plan-unit-start" value={editing.draft.plannedStartAt} disabled={locked} aria-invalid={Boolean(validationMessage(editing, 'plannedStartAt')) || undefined} aria-describedby={validationMessage(editing, 'plannedStartAt') ? 'pm-plan-unit-start-error' : undefined} onChange={(value) => control.onChange({ ...editing.draft, plannedStartAt: value })} /></WriteFormField>
            <WriteFormField id="pm-plan-unit-end" label={writeCopy.planUnitForm.plannedEnd} error={validationMessage(editing, 'plannedEndAt')}><DatePicker id="pm-plan-unit-end" value={editing.draft.plannedEndAt} defaultMonth={editing.draft.plannedStartAt} disabled={locked} aria-invalid={Boolean(validationMessage(editing, 'plannedEndAt')) || undefined} aria-describedby={validationMessage(editing, 'plannedEndAt') ? 'pm-plan-unit-end-error' : undefined} onChange={(value) => control.onChange({ ...editing.draft, plannedEndAt: value })} /></WriteFormField>
          </div>
          {!create ? (
            <PlanUnitDeliveryFields
              draft={editing.draft}
              catalog={editing.deliveryCatalog}
              locked={locked}
              control={{
                owners: control.owners,
                ...control.delivery,
              }}
            />
          ) : null}
          <div className="grid gap-3 sm:grid-cols-2">
            <WriteFormField id="pm-plan-unit-actual-start" label={writeCopy.planUnitForm.actualStart} error={validationMessage(editing, 'actualStartAt')}><DatePicker id="pm-plan-unit-actual-start" value={editing.draft.actualStartAt} defaultMonth={editing.draft.plannedStartAt} disabled={locked} aria-invalid={Boolean(validationMessage(editing, 'actualStartAt')) || undefined} aria-describedby={validationMessage(editing, 'actualStartAt') ? 'pm-plan-unit-actual-start-error' : undefined} onChange={(value) => control.onChange({ ...editing.draft, actualStartAt: value })} /></WriteFormField>
            <WriteFormField id="pm-plan-unit-actual-end" label={writeCopy.planUnitForm.actualEnd} error={validationMessage(editing, 'actualCompletedAt')}><DatePicker id="pm-plan-unit-actual-end" value={editing.draft.actualCompletedAt} defaultMonth={editing.draft.actualStartAt || editing.draft.plannedEndAt} disabled={locked} aria-invalid={Boolean(validationMessage(editing, 'actualCompletedAt')) || undefined} aria-describedby={validationMessage(editing, 'actualCompletedAt') ? 'pm-plan-unit-actual-end-error' : undefined} onChange={(value) => control.onChange({ ...editing.draft, actualCompletedAt: value })} /></WriteFormField>
          </div>
          <WriteFormField id="pm-plan-unit-risk" label={writeCopy.planUnitForm.risk} error={validationMessage(editing, 'riskLevel')}>
            <Select items={PLAN_UNIT_RISK_OPTIONS} value={editing.draft.riskLevel} disabled={locked} onValueChange={(value) => control.onChange({ ...editing.draft, riskLevel: String(value) })}>
              <SelectTrigger id="pm-plan-unit-risk" aria-invalid={Boolean(validationMessage(editing, 'riskLevel')) || undefined} aria-describedby={validationMessage(editing, 'riskLevel') ? 'pm-plan-unit-risk-error' : undefined}><SelectValue /></SelectTrigger>
              <SelectContent>
                {PLAN_UNIT_RISK_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </WriteFormField>
          <WriteFormField id="pm-plan-unit-goal" label={writeCopy.planUnitForm.goal}><Textarea id="pm-plan-unit-goal" value={editing.draft.goal} disabled={locked} placeholder={writeCopy.planUnitForm.goalPlaceholder} onChange={(event) => control.onChange({ ...editing.draft, goal: event.target.value })} /></WriteFormField>
          <div className="grid gap-1.5" aria-label={writeCopy.planUnitForm.status}>
            <span className="text-sm font-medium">{writeCopy.planUnitForm.status}</span>
            <div className="flex min-h-9 flex-wrap items-center gap-x-3 gap-y-1 rounded-md border border-border bg-muted/30 px-3 py-2 text-sm">
              <span className="font-medium">{editing.draft.mode === 'update'
                ? statusLabel(editing.draft.businessStatus)
                : writeCopy.planUnitForm.statusPending}</span>
              <span className="text-xs text-muted-foreground">{writeCopy.planUnitForm.statusWorkflowOwned}</span>
            </div>
          </div>
          {scheduleConflict ? <ScheduleMutationNotice code={editing.submit.status === 'error' ? editing.submit.errorCode : undefined} schedule={scheduleConflict} pending={busy} resolveSubjectName={control.resolveScheduleSubjectName} onAdjust={scheduleStale ? control.onReloadSchedule : () => control.onChange(editing.draft)} /> : null}
          {submitError ? <AlertBanner tone="error">{submitError}</AlertBanner> : null}
        </form>
      ) : null}
    </ResponsivePanel>
    {!create ? (
      <LabelManager
        state={control.labelManager.state}
        targetTitle={control.labelManager.targetTitle}
        control={control.labelManager.control}
      />
    ) : null}
    </>
  )
}
