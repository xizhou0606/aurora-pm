import { copy } from '@/shared/copy'
import type { PMSafeScheduleConflict } from '@/shared/api'
import {
  Button,
  DatePicker,
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
  UserDirectorySelect,
} from '@/shared/ui'
import {
  PROJECT_STATUS_OPTIONS,
  hasStaleSchedule,
  type OwnerDirectoryState,
  type ProjectDraft,
  type ProjectEditorState,
  type TeamDirectoryState,
} from '../model'
import { WriteFormField } from './WriteFormField'
import { ScheduleMutationNotice } from '@/features/ai-pm'

const writeCopy = copy.projectManagement.write

export type ProjectEditorControl = {
  teams: TeamDirectoryState
  owners: OwnerDirectoryState
  archiveAllowed: boolean
  scheduleReloading?: boolean
  resolveScheduleSubjectName?: (type: PMSafeScheduleConflict['subject_type'], id?: number) => string | undefined
  onChange: (draft: ProjectDraft) => void
  onReloadSchedule?: () => void
  onClose: () => void
  onSubmit: () => void
}

function projectValidationMessage(
  state: Extract<ProjectEditorState, { status: 'editing' }>,
  field: keyof typeof state.validation,
): string | undefined {
  if (!state.validationVisible || !state.validation[field]) return undefined
  switch (field) {
    case 'name':
      return writeCopy.validation.nameRequired
    case 'teamId':
      return writeCopy.validation.projectContextUnavailable
    case 'ownerUserId':
      return writeCopy.validation.ownerInvalid
    case 'plannedStartAt':
    case 'plannedEndAt':
      return writeCopy.validation.dateOrder
    case 'status':
      return writeCopy.validation.statusInvalid
    case 'projectId':
      return writeCopy.validation.contextChanged
  }
}

function PreparingProject({ teams }: { teams: TeamDirectoryState }) {
  switch (teams.status) {
    case 'loading':
      return <LoadingSkeleton rows={3} label={writeCopy.projectForm.contextLoading} />
    case 'error':
      return (
        <EmptyState
          title={writeCopy.projectForm.contextLoadFailed}
          action={<Button type="button" variant="outline" onClick={teams.retry}>{writeCopy.actions.retry}</Button>}
        />
      )
    case 'empty':
      return (
        <EmptyState
          title={writeCopy.projectForm.contextEmpty}
        />
      )
    case 'ready':
      return <LoadingSkeleton rows={2} label={writeCopy.projectForm.contextLoading} />
  }
}

export function ProjectEditorPanel({
  state,
  control,
}: {
  state: ProjectEditorState
  control: ProjectEditorControl
}) {
  const editing = state.status === 'editing' ? state : undefined
  const createDraft = editing?.draft.mode === 'create' ? editing.draft : undefined
  const create = state.status === 'preparing' || Boolean(createDraft)
  const submitting = editing?.submit.status === 'pending'
  const scheduleStale = editing ? hasStaleSchedule(editing.submit) : false
  const pending = Boolean(submitting || control.scheduleReloading)
  const locked = Boolean(pending || scheduleStale)
  const scheduleConflict = editing?.submit.status === 'error' ? editing.submit.schedule : undefined
  const submitError = editing?.submit.status === 'error' && !scheduleConflict
    ? writeCopy.feedback.saveFailed
    : undefined

  return (
    <ResponsivePanel
      open={state.status !== 'closed'}
      onOpenChange={(open) => { if (!open) control.onClose() }}
      title={create ? writeCopy.projectForm.createTitle : writeCopy.projectForm.editTitle}
      description={create ? undefined : writeCopy.projectForm.editDescription}
      closeLabel={writeCopy.actions.cancel}
      pending={pending}
      className="max-w-lg"
      footer={editing ? (
        <Button type="submit" form="pm-project-editor-form" disabled={locked}>
          {submitting ? writeCopy.actions.saving : writeCopy.actions.save}
        </Button>
      ) : undefined}
    >
      {state.status === 'preparing' ? <PreparingProject teams={control.teams} /> : null}
      {editing ? (
        <form id="pm-project-editor-form" className="grid gap-4" onSubmit={(event) => { event.preventDefault(); control.onSubmit() }}>
          <WriteFormField
            id="pm-project-name"
            label={writeCopy.projectForm.name}
            required
            error={projectValidationMessage(editing, 'name')}
          >
            <Input
              id="pm-project-name"
              autoFocus={create}
              value={editing.draft.name}
              disabled={locked}
              required
              aria-invalid={Boolean(projectValidationMessage(editing, 'name')) || undefined}
              aria-describedby={projectValidationMessage(editing, 'name') ? 'pm-project-name-error' : undefined}
              placeholder={writeCopy.projectForm.namePlaceholder}
              onChange={(event) => control.onChange({ ...editing.draft, name: event.target.value })}
            />
          </WriteFormField>

          <WriteFormField id="pm-project-description" label={writeCopy.projectForm.description}>
            <Textarea
              id="pm-project-description"
              value={editing.draft.description}
              disabled={locked}
              placeholder={writeCopy.projectForm.descriptionPlaceholder}
              onChange={(event) => control.onChange({ ...editing.draft, description: event.target.value })}
            />
          </WriteFormField>

          {createDraft ? (
            <UserDirectorySelect
              id="pm-project-owner"
              label={writeCopy.projectForm.owner}
              value={createDraft.ownerUserId}
              disabled={locked}
              state={control.owners}
              error={projectValidationMessage(editing, 'ownerUserId')}
              onValueChange={(value) => control.onChange({ ...createDraft, ownerUserId: value ?? null })}
            />
          ) : null}

          <div className="grid gap-3 sm:grid-cols-2">
            <WriteFormField
              id="pm-project-start"
              label={writeCopy.projectForm.plannedStart}
              error={projectValidationMessage(editing, 'plannedStartAt')}
            >
              <DatePicker
                id="pm-project-start"
                value={editing.draft.plannedStartAt}
                disabled={locked}
                aria-invalid={Boolean(projectValidationMessage(editing, 'plannedStartAt')) || undefined}
                aria-describedby={projectValidationMessage(editing, 'plannedStartAt') ? 'pm-project-start-error' : undefined}
                onChange={(value) => control.onChange({ ...editing.draft, plannedStartAt: value })}
              />
            </WriteFormField>
            <WriteFormField
              id="pm-project-end"
              label={writeCopy.projectForm.plannedEnd}
              error={projectValidationMessage(editing, 'plannedEndAt')}
            >
              <DatePicker
                id="pm-project-end"
                value={editing.draft.plannedEndAt}
                defaultMonth={editing.draft.plannedStartAt}
                disabled={locked}
                aria-invalid={Boolean(projectValidationMessage(editing, 'plannedEndAt')) || undefined}
                aria-describedby={projectValidationMessage(editing, 'plannedEndAt') ? 'pm-project-end-error' : undefined}
                onChange={(value) => control.onChange({ ...editing.draft, plannedEndAt: value })}
              />
            </WriteFormField>
          </div>

          <WriteFormField
            id="pm-project-status"
            label={writeCopy.projectForm.status}
            error={projectValidationMessage(editing, 'status')}
          >
            <Select
              items={PROJECT_STATUS_OPTIONS}
              value={editing.draft.status}
              disabled={locked}
              onValueChange={(value) => control.onChange({
                ...editing.draft,
                status: String(value),
              })}
            >
              <SelectTrigger
                id="pm-project-status"
                aria-invalid={Boolean(projectValidationMessage(editing, 'status')) || undefined}
                aria-describedby={projectValidationMessage(editing, 'status') ? 'pm-project-status-error' : undefined}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PROJECT_STATUS_OPTIONS.map((option) => (
                  <SelectItem
                    key={option.value}
                    value={option.value}
                    disabled={option.value === 'archived' && !control.archiveAllowed}
                  >
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </WriteFormField>

          {scheduleConflict ? <ScheduleMutationNotice code={editing.submit.status === 'error' ? editing.submit.errorCode : undefined} schedule={scheduleConflict} pending={pending} resolveSubjectName={control.resolveScheduleSubjectName} onAdjust={scheduleStale ? control.onReloadSchedule : () => control.onChange(editing.draft)} /> : null}
          {submitError ? <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{writeCopy.feedback.saveFailed}</p> : null}
        </form>
      ) : null}
    </ResponsivePanel>
  )
}
