import {
  useEffect,
  useState,
  type FormEvent,
} from 'react'

import type { PMScopeOptions, PMScopeSelection } from '@/shared/api'
import { OPERATION_FEEDBACK_COPY, QUALITY_COPY } from '@/shared/copy'
import {
  Button,
  ConfirmDialog,
  LazyRichTextField,
  richTextVisibleCharacterCount,
  notifyOperationError,
  operationFeedback,
  type RichTextPastedImage,
} from '@/shared/ui'
import type {
  QualityBugDraft,
  QualitySceneOptionCatalogs,
  QualitySceneOptionMutations,
  QualityMaterial,
  QualityMutationState,
  QualityUsersState,
} from '../../model/types'
import {
  isQualityRichTextEmpty,
  isQualityInlineImage,
  isQualityPasteMaterialFile,
  qualityRequirementSelection,
  qualityReproductionTemplatePayload,
} from '../../model'
import { QUALITY_NOTE_MAX_CHARACTERS } from '../../model/payloads'
import { MaterialEditor } from '../dialogs/Fields'
import { BasicSection } from './BasicSection'
import { SceneSection } from './SceneSection'
import { validateForm, type FormValidationField } from './validation'

export function Form({
  formId,
  actions = 'inline',
  initial,
  scope,
  users,
  mutation,
  uploadState,
  canUpload,
  sceneOptions = {
    environment: { status: 'ready', rows: [{ name: 'test', builtIn: true, legacy: false }] },
    operating_system: { status: 'ready', rows: [] },
    browser: { status: 'ready', rows: [] },
  },
  sceneOptionMutations,
  allowStatusEdit = false,
  submitLabel,
  onSubmit,
  onUpload,
  onCancel,
}: {
  formId?: string
  actions?: 'inline' | 'external'
  initial: QualityBugDraft
  scope: PMScopeOptions | null
  users: QualityUsersState
  mutation: QualityMutationState
  uploadState: QualityMutationState
  canUpload: boolean
  sceneOptions?: QualitySceneOptionCatalogs
  sceneOptionMutations?: QualitySceneOptionMutations
  allowStatusEdit?: boolean
  submitLabel: string
  onSubmit: (draft: QualityBugDraft) => Promise<void>
  onUpload: (requirementId: number, file: File) => Promise<QualityMaterial>
  onCancel?: () => void
}) {
  const [draft, setDraft] = useState(initial)
  const [requirementSelection, setRequirementSelection] = useState<PMScopeSelection>(() => (
    qualityRequirementSelection(scope, initial.requirementId)
  ))
  const [validation, setValidation] = useState<FormValidationField>('')
  const [templateConfirmOpen, setTemplateConfirmOpen] = useState(false)

  useEffect(() => {
    setDraft(initial)
    setTemplateConfirmOpen(false)
  }, [initial])

  useEffect(() => {
    setRequirementSelection(qualityRequirementSelection(scope, initial.requirementId))
  }, [initial.requirementId, scope])

  const update = <TKey extends keyof QualityBugDraft>(
    key: TKey,
    value: QualityBugDraft[TKey],
  ) => setDraft((current) => ({ ...current, [key]: value }))

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    const result = validateForm(draft, setValidation)
    if (!result.valid) return
    setValidation('')
    await onSubmit(draft).catch(() => undefined)
  }

  const userOptions = users.rows.map((user) => ({
    id: user.id,
    name: user.name,
    secondary: user.username,
    avatarUrl: user.avatarUrl,
  }))
  const usersUnavailable = users.status !== 'ready' && users.rows.length === 0
  const mutationPending = mutation.status === 'pending'
  const uploadPending = uploadState.status === 'pending'
  const resolutionNoteTooLong = richTextVisibleCharacterCount(draft.resolutionNote) >
    QUALITY_NOTE_MAX_CHARACTERS

  const applyTemplate = () => {
    const payload = qualityReproductionTemplatePayload()
    if (!payload) return
    if (!isQualityRichTextEmpty(draft.reproductionSteps)) {
      setTemplateConfirmOpen(true)
      return
    }
    update('reproductionSteps', payload)
  }
  const confirmTemplate = () => {
    update('reproductionSteps', qualityReproductionTemplatePayload())
    setTemplateConfirmOpen(false)
  }
  const attachMaterial = (material: QualityMaterial) => {
    setDraft((current) => ({
      ...current,
      reproductionMaterials: [...current.reproductionMaterials, material],
    }))
  }
  const uploadPastedMaterials = async (
    incomingFiles: readonly File[],
  ): Promise<readonly RichTextPastedImage[]> => {
    const files = incomingFiles.filter(isQualityPasteMaterialFile)
    if (!files.length) return []
    if (!canUpload || mutationPending || uploadPending) {
      notifyOperationError(
        operationFeedback(OPERATION_FEEDBACK_COPY.quality.upload),
        new Error('PM_QUALITY_PASTE_UPLOAD_UNAVAILABLE'),
      )
      return []
    }
    const images: RichTextPastedImage[] = []
    for (const file of files) {
      try {
        const material = await onUpload(draft.requirementId || 0, file)
        if (isQualityInlineImage(file)) {
          images.push({ src: material.url, alt: file.name })
        } else {
          attachMaterial(material)
        }
      } catch {
        // useUploadMutation owns the failure Toast and permits the remaining
        // clipboard files to continue uploading.
      }
    }
    return images
  }

  return (
    <form id={formId} className="grid gap-5" onSubmit={(event) => void submit(event)}>
      <BasicSection
        draft={draft}
        userOptions={userOptions}
        usersUnavailable={usersUnavailable}
        users={users}
        scope={scope}
        requirementSelection={requirementSelection}
        allowStatusEdit={allowStatusEdit}
        mutationPending={mutationPending}
        uploadPending={uploadPending}
        validation={validation}
        resolutionNoteTooLong={resolutionNoteTooLong}
        onUpdate={update}
        onRequirementSelectionChange={(selection) => {
          setRequirementSelection(selection)
          update('requirementId', selection.requirementIds[0])
        }}
        onClearValidation={() => {
          if (validation === 'resolutionNote') setValidation('')
        }}
        onPasteFiles={uploadPastedMaterials}
      />

      <SceneSection
        draft={draft}
        sceneOptions={sceneOptions}
        sceneOptionMutations={sceneOptionMutations}
        onUpdate={update}
      />

      <section className="grid gap-3 rounded-lg border border-border bg-card p-4">
        <h2 className="text-base font-semibold">{QUALITY_COPY.form.results}</h2>
        <LazyRichTextField
          label={QUALITY_COPY.form.reproduction}
          value={draft.reproductionSteps}
          serialization="json"
          minHeight={180}
          toolbarAction={(
            <Button type="button" size="sm" variant="ghost" disabled={mutationPending} onClick={applyTemplate}>
              {QUALITY_COPY.form.applyTemplate}
            </Button>
          )}
          onPasteFiles={uploadPastedMaterials}
          onChange={(value) => update('reproductionSteps', value)}
        />
      </section>

      <section
        aria-label={QUALITY_COPY.form.materials}
        className="grid gap-3 rounded-lg border border-border bg-card p-4"
      >
        <MaterialEditor
          items={draft.reproductionMaterials}
          requirementId={draft.requirementId}
          canUpload={canUpload && !mutationPending}
          state={uploadState}
          onChange={(items) => update('reproductionMaterials', items)}
          onUpload={onUpload}
        />
      </section>

      {actions === 'inline' ? (
        <div className="flex flex-wrap justify-end gap-2">
          {onCancel ? <Button type="button" variant="outline" disabled={mutationPending} onClick={onCancel}>{QUALITY_COPY.form.cancel}</Button> : null}
          <Button type="submit" disabled={mutationPending || resolutionNoteTooLong}>{mutationPending ? QUALITY_COPY.mutation.pending : submitLabel}</Button>
        </div>
      ) : null}
      <ConfirmDialog
        open={templateConfirmOpen}
        onOpenChange={setTemplateConfirmOpen}
        title={QUALITY_COPY.form.templateConfirmTitle}
        description={QUALITY_COPY.form.templateConfirmDescription}
        cancelLabel={QUALITY_COPY.form.cancel}
        confirmLabel={QUALITY_COPY.form.applyTemplate}
        confirmVariant="default"
        onConfirm={confirmTemplate}
      />
    </form>
  )
}