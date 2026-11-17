import { DatePicker } from '@/shared/ui'
import type { PMScopeOptions, PMScopeSelection } from '@/shared/api'
import { QUALITY_COPY } from '@/shared/copy'
import {
  Input,
  PMScopeCascaderField,
  UserSelect,
  type RichTextPastedImage,
} from '@/shared/ui'
import type {
  QualityBugDraft,
  QualityUsersState,
} from '../../model/types'
import {
  qualityRequirementLabel,
  qualityStatusFilterOptions,
} from '../../model'
import { QualitySelectField } from '../controls'
import { UserDirectoryStatus } from '../list/UserDirectoryStatus'
import { Field } from './Field'
import { ResolutionFields } from './ResolutionFields'
import type { FormValidationField } from './validation'

interface UserOption {
  id: number
  name: string
  secondary?: string
  avatarUrl?: string
}

export function BasicSection({
  draft,
  userOptions,
  usersUnavailable,
  users,
  scope,
  requirementSelection,
  allowStatusEdit,
  mutationPending,
  uploadPending,
  validation,
  resolutionNoteTooLong,
  onUpdate,
  onRequirementSelectionChange,
  onClearValidation,
  onPasteFiles,
}: {
  draft: QualityBugDraft
  userOptions: readonly UserOption[]
  usersUnavailable: boolean
  users: QualityUsersState
  scope: PMScopeOptions | null
  requirementSelection: PMScopeSelection
  allowStatusEdit: boolean
  mutationPending: boolean
  uploadPending: boolean
  validation: FormValidationField
  resolutionNoteTooLong: boolean
  onUpdate: <TKey extends keyof QualityBugDraft>(key: TKey, value: QualityBugDraft[TKey]) => void
  onRequirementSelectionChange: (selection: PMScopeSelection) => void
  onClearValidation: () => void
  onPasteFiles: (files: readonly File[]) => Promise<readonly RichTextPastedImage[]>
}) {
  return (
    <section
      className="grid gap-3 rounded-lg border border-border bg-card p-4"
      aria-busy={users.status === 'loading' || undefined}
    >
      <h2 className="text-base font-semibold">{QUALITY_COPY.form.basic}</h2>
      <UserDirectoryStatus state={users} />
      <Field id="quality-form-title" label={QUALITY_COPY.form.title}>
        <Input
          id="quality-form-title"
          value={draft.title}
          maxLength={160}
          placeholder={QUALITY_COPY.form.titlePlaceholder}
          aria-invalid={validation === 'title' || undefined}
          aria-describedby={validation === 'title' ? 'quality-form-title-error' : undefined}
          onChange={(event) => onUpdate('title', event.currentTarget.value)}
        />
        {validation === 'title' ? <p id="quality-form-title-error" role="alert" className="m-0 text-xs text-destructive">{QUALITY_COPY.form.titleRequired}</p> : null}
      </Field>
      <PMScopeCascaderField
        idPrefix="quality-form-requirement"
        state={scope ? { status: 'ready', options: scope } : { status: 'loading' }}
        value={requirementSelection}
        selectionMode="single-requirement"
        labels={{
          label: QUALITY_COPY.form.requirement,
          hierarchyHint: QUALITY_COPY.scope.hierarchyHint,
          projectSet: QUALITY_COPY.scope.projectSet,
          project: QUALITY_COPY.scope.project,
          requirement: QUALITY_COPY.scope.requirement,
          selectProjectSetFirst: QUALITY_COPY.scope.selectProjectSetFirst,
          selectProjectFirst: QUALITY_COPY.scope.selectProjectFirst,
        }}
        summary={qualityRequirementLabel(scope, draft.requirementId) || QUALITY_COPY.none}
        error={validation === 'requirement' ? QUALITY_COPY.form.requirementRequired : undefined}
        onChange={(selection) => {
          onRequirementSelectionChange(selection)
          onUpdate('requirementId', selection.requirementIds[0])
        }}
      />
      <div className={allowStatusEdit
        ? 'grid gap-3 sm:grid-cols-2 lg:grid-cols-6'
        : 'grid gap-3 sm:grid-cols-2 lg:grid-cols-5'}>
        {allowStatusEdit ? (
          <QualitySelectField
            id="quality-form-status"
            label={QUALITY_COPY.form.status}
            value={draft.status}
            options={qualityStatusFilterOptions(draft.status)
              .filter(({ value }) => value !== 'closed')
              .map(({ value, label }) => ({ value, label }))}
            emptyLabel={QUALITY_COPY.none}
            onValueChange={(value) => onUpdate('status', value)}
          />
        ) : null}
        <Field id="quality-form-assignee" label={QUALITY_COPY.form.assignee}>
          <UserSelect
            id="quality-form-assignee"
            label={QUALITY_COPY.form.assignee}
            value={draft.assigneeUserId}
            options={userOptions}
            emptyLabel={QUALITY_COPY.none}
            disabled={usersUnavailable}
            onValueChange={(value) => onUpdate('assigneeUserId', value)}
          />
        </Field>
        <QualitySelectField
          id="quality-form-type"
          label={QUALITY_COPY.form.type}
          value={draft.bugType}
          options={Object.entries(QUALITY_COPY.enums.type).map(([value, label]) => ({ value, label }))}
          emptyLabel={QUALITY_COPY.none}
          onValueChange={(value) => onUpdate('bugType', value)}
        />
        <QualitySelectField
          id="quality-form-defect-domain"
          label={QUALITY_COPY.form.defectDomain}
          value={draft.defectDomain}
          options={Object.entries(QUALITY_COPY.enums.defectDomain).map(([value, label]) => ({ value, label }))}
          emptyLabel={QUALITY_COPY.none}
          onValueChange={(value) => onUpdate('defectDomain', value === 'ui' ? 'ui' : 'functional')}
        />
        <QualitySelectField
          id="quality-form-severity"
          label={QUALITY_COPY.form.severity}
          value={draft.severity}
          options={Object.entries(QUALITY_COPY.enums.severity).map(([value, label]) => ({ value, label }))}
          emptyLabel={QUALITY_COPY.none}
          onValueChange={(value) => onUpdate('severity', value)}
        />
        <Field id="quality-form-due-date" label={QUALITY_COPY.form.dueDate}>
          <DatePicker id="quality-form-due-date"  value={draft.dueDate} onChange={(value) => onUpdate('dueDate', value)} />
        </Field>
      </div>
      {allowStatusEdit && (draft.status === 'review' || draft.status === 'done') ? (
        <ResolutionFields
          draft={draft}
          userOptions={userOptions}
          usersUnavailable={usersUnavailable}
          mutationPending={mutationPending}
          uploadPending={uploadPending}
          validation={validation}
          resolutionNoteTooLong={resolutionNoteTooLong}
          onUpdate={onUpdate}
          onClearValidation={onClearValidation}
          onPasteFiles={onPasteFiles}
        />
      ) : null}
    </section>
  )
}
