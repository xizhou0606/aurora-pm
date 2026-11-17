import { DatePicker } from '@/shared/ui'
import { useState, type FormEvent, type RefObject } from 'react'

import type { PMScopeOptions } from '@/shared/api'
import { WORK_ITEMS_COPY, copy } from '@/shared/copy'
import {
  Button,
  Input,
  LazyFileUploader,
  LazyRichTextField,
  PMScopeCascaderField,
  ResponsivePanel,
  UserSelect,
  type RichTextPastedImage,
} from '@/shared/ui'
import type {
  WorkItemCreateDraft,
  WorkItemCreateState,
  WorkItemRequirementOption,
  WorkItemUserOption,
  WorkItemWorkflowStatus,
  WorkItemClientOptionMutations,
  WorkItemClientOptionsState,
} from '../../model/types'
import { selectedRequirementOption } from '../../model/create'
import { WORK_ITEM_PRIORITY_OPTIONS, workItemUserSelectOption } from '../../model/utils'
import {
  isWorkItemAttachmentFile,
  isWorkItemInlineImage,
  WORK_ITEM_ATTACHMENT_MAX_BYTES,
} from '../../model/attachments'
import { WorkItemsSelect } from '../collection/SelectField'
import { validationMessage, Field } from './CreatePanelHelpers'
import { ClientField } from './ClientField'

const FORM_ID = 'work-items-create-form'

export function CreatePanel({
  state,
  scopeOptions,
  requirements,
  users,
  usersState,
  statuses,
  clients,
  clientMutations,
  finalFocus,
  onOpenChange,
  onUpdate,
  canAssignToMe,
  canUploadAttachments,
  onAssignToMe,
  onAddAttachment,
  onRemoveAttachment,
  onUploadRichTextImage,
  onSubmit,
}: {
  state: WorkItemCreateState
  scopeOptions?: PMScopeOptions
  requirements: readonly WorkItemRequirementOption[]
  users: readonly WorkItemUserOption[]
  usersState: 'hidden' | 'loading' | 'error' | 'ready'
  statuses: readonly WorkItemWorkflowStatus[]
  clients: WorkItemClientOptionsState
  clientMutations: WorkItemClientOptionMutations
  finalFocus: RefObject<HTMLElement | null>
  onOpenChange: (open: boolean) => void
  onUpdate: <TKey extends keyof WorkItemCreateDraft>(
    key: TKey,
    value: WorkItemCreateDraft[TKey],
  ) => void
  canAssignToMe: boolean
  canUploadAttachments: boolean
  onAssignToMe: () => void
  onAddAttachment: (file: File) => void
  onRemoveAttachment: (file: File) => void
  onUploadRichTextImage: (file: File) => Promise<string | null>
  onSubmit: () => Promise<boolean>
}) {
  const [pasteError, setPasteError] = useState<string>()
  const editing = state.status === 'editing' ? state : undefined
  const draft = editing?.draft
  const pending = editing?.submit === 'pending'
  const validation = validationMessage(editing?.validation)
  const feedback = validation || (editing?.submit === 'error' ? WORK_ITEMS_COPY.create.failed : undefined)
  const selectedRequirement = draft
    ? selectedRequirementOption(draft.requirementScope, requirements)
    : undefined
  const submit = (event: FormEvent) => {
    event.preventDefault()
    void onSubmit()
  }
  const uploadPastedFiles = async (
    files: readonly File[],
  ): Promise<readonly RichTextPastedImage[]> => {
    setPasteError(undefined)
    const images: RichTextPastedImage[] = []
    let failed = false
    for (const file of files) {
      if (isWorkItemInlineImage(file)) {
        try {
          const src = await onUploadRichTextImage(file)
          if (src) images.push({ src, alt: file.name })
          else failed = true
        } catch {
          failed = true
        }
      } else if (isWorkItemAttachmentFile(file)) {
        onAddAttachment(file)
      } else {
        failed = true
      }
    }
    if (failed) setPasteError(copy.ui.richText.pasteFailed)
    return images
  }

  return (
    <ResponsivePanel
      open={Boolean(editing)}
      onOpenChange={onOpenChange}
      title={WORK_ITEMS_COPY.create.title}
      closeLabel={WORK_ITEMS_COPY.create.cancel}
      pending={pending}
      finalFocus={finalFocus}
      className="max-w-3xl"
      footer={(
        <Button type="submit" form={FORM_ID} disabled={pending}>
          {pending ? WORK_ITEMS_COPY.create.pending : WORK_ITEMS_COPY.create.submit}
        </Button>
      )}
    >
      {draft && scopeOptions ? (
        <form id={FORM_ID} className="grid gap-5" onSubmit={submit}>
          <section className="grid gap-4 rounded-lg border border-border bg-card p-4">
            <h2 className="m-0 text-base font-semibold">{WORK_ITEMS_COPY.create.details}</h2>
            <Field id="work-items-create-title" label={WORK_ITEMS_COPY.create.name}>
              <div className="relative">
                <Input
                  id="work-items-create-title"
                  className="pr-16"
                  autoFocus
                  maxLength={256}
                  value={draft.title}
                  placeholder={WORK_ITEMS_COPY.create.namePlaceholder}
                  aria-invalid={editing?.validation === 'title' || undefined}
                  onChange={(event) => onUpdate('title', event.currentTarget.value)}
                />
                <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-muted-foreground">
                  {WORK_ITEMS_COPY.create.titleCount(draft.title.length)}
                </span>
              </div>
            </Field>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field id="work-items-create-kind" label={WORK_ITEMS_COPY.create.kind}>
                <WorkItemsSelect
                  id="work-items-create-kind"
                  label={WORK_ITEMS_COPY.create.kind}
                  value="task"
                  options={[{ value: 'task', label: WORK_ITEMS_COPY.create.taskKind }]}
                  disabled
                  onValueChange={() => undefined}
                />
              </Field>
              <Field id="work-items-create-priority" label={WORK_ITEMS_COPY.create.priority}>
                <WorkItemsSelect
                  id="work-items-create-priority"
                  label={WORK_ITEMS_COPY.create.priority}
                  value={draft.priority}
                  options={WORK_ITEM_PRIORITY_OPTIONS}
                  onValueChange={(value) => onUpdate('priority', value)}
                />
              </Field>
              <Field id="work-items-create-client" label={WORK_ITEMS_COPY.create.client}>
                <ClientField
                  id="work-items-create-client"
                  value={draft.taskClientId}
                  state={clients}
                  mutations={clientMutations}
                  disabled={pending}
                  onValueChange={(value) => onUpdate('taskClientId', value)}
                />
              </Field>
            </div>
            <PMScopeCascaderField
              idPrefix="work-items-create-requirement"
              state={{ status: 'ready', options: scopeOptions }}
              value={draft.requirementScope}
              selectionMode="single-requirement"
              labels={{
                label: WORK_ITEMS_COPY.create.requirement,
                description: WORK_ITEMS_COPY.create.requirementHint,
                hierarchyHint: WORK_ITEMS_COPY.create.requirementHierarchyHint,
                projectSet: WORK_ITEMS_COPY.scope.projectSet,
                project: WORK_ITEMS_COPY.scope.pmProject,
                requirement: WORK_ITEMS_COPY.scope.requirement,
                selectProjectSetFirst: WORK_ITEMS_COPY.scope.selectProjectSetFirst,
                selectProjectFirst: WORK_ITEMS_COPY.scope.selectProjectFirst,
              }}
              summary={selectedRequirement?.label ?? WORK_ITEMS_COPY.create.requirementPlaceholder}
              onChange={(selection) => onUpdate('requirementScope', selection)}
            />
            <div className="grid gap-4 sm:grid-cols-2">
              <Field id="work-items-create-assignee" label={WORK_ITEMS_COPY.create.assignee}>
                <div className="grid gap-1.5">
                  <UserSelect
                    id="work-items-create-assignee"
                    label={WORK_ITEMS_COPY.create.assignee}
                    value={draft.assigneeId}
                    options={users.map(workItemUserSelectOption)}
                    placeholder={WORK_ITEMS_COPY.create.unassigned}
                    disabled={usersState !== 'ready'}
                    onValueChange={(value) => {
                      if (value !== undefined) onUpdate('assigneeId', value)
                    }}
                  />
                  <Button
                    type="button"
                    variant="link"
                    className="h-8 w-fit px-0"
                    disabled={!canAssignToMe}
                    onClick={onAssignToMe}
                  >
                    {WORK_ITEMS_COPY.create.assignToMe}
                  </Button>
                </div>
              </Field>
              <Field id="work-items-create-status" label={WORK_ITEMS_COPY.create.status}>
                <div className="grid gap-1.5">
                  <WorkItemsSelect
                    id="work-items-create-status"
                    label={WORK_ITEMS_COPY.create.status}
                    value={draft.status}
                    options={statuses.map((status) => ({ value: status.key, label: status.label }))}
                    onValueChange={(value) => onUpdate('status', value)}
                  />
                  <p className="m-0 flex h-8 items-center text-xs text-muted-foreground">
                    {WORK_ITEMS_COPY.create.statusHint}
                  </p>
                </div>
              </Field>
              <Field id="work-items-create-starts-at" label={WORK_ITEMS_COPY.create.startsAt}>
                <DatePicker
                  id="work-items-create-starts-at"

                  value={draft.startsAt}
                  onChange={(value) => onUpdate('startsAt', value)}
                />
              </Field>
              <Field id="work-items-create-due-at" label={WORK_ITEMS_COPY.create.dueAt}>
                <DatePicker
                  id="work-items-create-due-at"

                  value={draft.dueAt}
                  onChange={(value) => onUpdate('dueAt', value)}
                />
              </Field>
              <Field id="work-items-create-estimated-hours" label={WORK_ITEMS_COPY.create.estimatedHours}>
                <Input
                  id="work-items-create-estimated-hours"
                  type="number"
                  min="0"
                  max="16666"
                  step="0.5"
                  value={draft.estimatedHours}
                  placeholder={WORK_ITEMS_COPY.create.estimatedHoursPlaceholder}
                  onChange={(event) => onUpdate('estimatedHours', event.currentTarget.value)}
                />
              </Field>
            </div>
            <LazyRichTextField
              label={WORK_ITEMS_COPY.create.descriptionLabel}
              value={draft.description}
              documentId="new-task"
              scope={draft.requirementScope.requirementIds[0] || 'unselected'}
              minHeight={180}
              serialization="json"
              placeholder={WORK_ITEMS_COPY.create.descriptionPlaceholder}
              error={pasteError}
              onPasteFiles={canUploadAttachments ? uploadPastedFiles : undefined}
              onChange={(value) => onUpdate('description', value)}
            />
            <LazyFileUploader
              label={WORK_ITEMS_COPY.create.attachments}
              hint={WORK_ITEMS_COPY.create.attachmentHint}
              inputLabel={WORK_ITEMS_COPY.create.chooseAttachments}
              multiple
              maxBytes={WORK_ITEM_ATTACHMENT_MAX_BYTES}
              disabled={pending || !canUploadAttachments}
              items={draft.attachments.map((file) => ({
                url: `pending:${file.name}:${file.size}:${file.lastModified}`,
                name: file.name,
                file,
              }))}
              onUpload={(file) => onAddAttachment(file)}
              onRemove={(item) => onRemoveAttachment(item.file)}
            />
          </section>
          {feedback ? (
            <p id="work-items-create-feedback" role="alert" className="m-0 text-sm text-destructive">
              {feedback}
            </p>
          ) : null}
        </form>
      ) : null}
    </ResponsivePanel>
  )
}
