import { useMemo, useState, type FormEvent } from 'react'
import { PhTrash } from '@/shared/ui/icons'

import { QUALITY_COPY } from '@/shared/copy'
import {
  Button,
  ConfirmDialog,
  Input,
  ResponsivePanel,
  TableActionButton,
} from '@/shared/ui'
import {
  QUALITY_DEFAULT_ENVIRONMENT,
  qualityEnvironmentMutationError,
} from '../../model'
import type {
  QualitySceneOption,
  QualitySceneOptionKind,
  QualitySceneOptionMutations,
  QualitySceneOptionsState,
} from '../../model/types'
import { QualitySelectField } from '../controls'

type SceneOptionNameError = 'required' | 'invalid' | 'duplicate' | 'in_use' | 'request' | ''

export function normalizeEnvironmentNameInput(raw: string): string {
  return raw.trim().replace(/ +/g, ' ')
}

function validateSceneOptionName(
  raw: string,
  options: readonly QualitySceneOption[],
): { name: string; error: SceneOptionNameError } {
  const name = normalizeEnvironmentNameInput(raw)
  if (!name) return { name, error: 'required' }
  if ([...name].length > 64 || !/^[\p{L}\p{N}._ /-]+$/u.test(name)) {
    return { name, error: 'invalid' }
  }
  const normalized = name.toLocaleLowerCase()
  if (options.some((item) => item.name.trim().toLocaleLowerCase() === normalized)) {
    return { name, error: 'duplicate' }
  }
  return { name, error: '' }
}

function sceneOptionErrorCopy(
  error: SceneOptionNameError,
  kind: QualitySceneOptionKind,
): string | undefined {
  const itemCopy = QUALITY_COPY.form.sceneOptions[kind]
  switch (error) {
    case 'required': return itemCopy.required
    case 'invalid': return QUALITY_COPY.form.sceneOptionNameInvalid
    case 'duplicate': return itemCopy.duplicate
    case 'in_use': return QUALITY_COPY.form.sceneOptionDeleteBlocked
    case 'request': return QUALITY_COPY.form.sceneOptionRequestFailed
    default: return undefined
  }
}

function SceneOptionManager({
  open,
  kind,
  state,
  mutations,
  onOpenChange,
  onSelect,
}: {
  open: boolean
  kind: QualitySceneOptionKind
  state: QualitySceneOptionsState
  mutations: QualitySceneOptionMutations
  onOpenChange: (open: boolean) => void
  onSelect: (value: string) => void
}) {
  const copy = QUALITY_COPY.form.sceneOptions[kind]
  const [nameDraft, setNameDraft] = useState('')
  const [validation, setValidation] = useState<SceneOptionNameError>('')
  const [deleteTarget, setDeleteTarget] = useState<QualitySceneOption | null>(null)
  const creating = mutations.createState.status === 'pending'
  const deleting = mutations.deleteState.status === 'pending'
  const pending = creating || deleting

  const close = (nextOpen: boolean) => {
    if (!nextOpen && !pending) {
      setNameDraft('')
      setValidation('')
      setDeleteTarget(null)
      mutations.reset()
    }
    onOpenChange(nextOpen)
  }
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    event.stopPropagation()
    const checked = validateSceneOptionName(nameDraft, state.rows)
    setNameDraft(checked.name)
    setValidation(checked.error)
    if (checked.error) return
    try {
      const created = await mutations.create({ kind, name: checked.name })
      onSelect(created.name)
      close(false)
    } catch (error) {
      setValidation(qualityEnvironmentMutationError(error))
    }
  }
  const confirmDelete = async () => {
    if (!deleteTarget?.id || !mutations.canDelete(deleteTarget)) return
    try {
      await mutations.remove({ kind, option: deleteTarget })
      setDeleteTarget(null)
      setValidation('')
    } catch (error) {
      setValidation(qualityEnvironmentMutationError(error))
      setDeleteTarget(null)
    }
  }

  return (
    <>
      <ResponsivePanel
        open={open}
        onOpenChange={close}
        pending={pending}
        title={copy.dialogTitle}
        closeLabel={QUALITY_COPY.form.cancel}
        className="max-w-lg"
      >
        <div className="grid gap-4">
          <section className="grid gap-2" aria-label={copy.list}>
            <p className="m-0 text-sm font-medium">{copy.list}</p>
            <div className="h-52 overflow-y-auto overscroll-contain rounded-md border border-border bg-muted/20 p-1">
              {state.status === 'loading' && state.rows.length === 0 ? (
                <p role="status" className="m-0 px-3 py-2 text-sm text-muted-foreground">
                  {QUALITY_COPY.list.loading}
                </p>
              ) : state.rows.length ? state.rows.map((item) => {
                const usageCount = item.usageCount || 0
                const canDelete = mutations.canDelete(item)
                return (
                  <div
                    key={`${item.id || item.name}-${item.name}`}
                    className="flex min-w-0 items-center gap-1 rounded-sm hover:bg-muted"
                  >
                    <Button
                      type="button"
                      variant="ghost"
                      className="h-auto min-w-0 flex-1 justify-start rounded-sm px-3 py-2 text-left font-normal hover:bg-transparent"
                      onClick={() => {
                        onSelect(item.name)
                        close(false)
                      }}
                    >
                      <span className="min-w-0 flex-1 truncate">{item.name}</span>
                      {item.builtIn ? (
                        <span className="text-xs text-muted-foreground">{QUALITY_COPY.form.sceneOptionBuiltIn}</span>
                      ) : item.legacy ? (
                        <span className="text-xs text-muted-foreground">{QUALITY_COPY.form.sceneOptionLegacy}</span>
                      ) : usageCount > 0 ? (
                        <span className="text-xs text-muted-foreground">
                          {QUALITY_COPY.form.sceneOptionUsage(usageCount)}
                        </span>
                      ) : null}
                    </Button>
                    {item.id && !item.builtIn && !item.legacy ? (
                      <TableActionButton
                        label={canDelete
                          ? QUALITY_COPY.form.sceneOptionDelete(item.name)
                          : QUALITY_COPY.form.sceneOptionDeleteInUse(usageCount)}
                        icon={<PhTrash aria-hidden="true" />}
                        disabled={!canDelete}
                        onClick={() => setDeleteTarget(item)}
                      />
                    ) : null}
                  </div>
                )
              }) : (
                <p className="m-0 px-3 py-2 text-sm text-muted-foreground">{copy.empty}</p>
              )}
            </div>
            {state.status === 'error' ? (
              <div role="alert" className="flex items-center justify-between gap-2 text-sm text-destructive">
                <span>{QUALITY_COPY.form.sceneOptionLoadFailed}</span>
                <Button type="button" size="sm" variant="outline" onClick={() => void state.retry()}>
                  {QUALITY_COPY.form.sceneOptionRetry}
                </Button>
              </div>
            ) : null}
          </section>
          {mutations.canCreate ? <form className="grid gap-1.5" onSubmit={(event) => void submit(event)}>
            <div className="flex items-start gap-2">
              <Input
                value={nameDraft}
                maxLength={64}
                placeholder={copy.namePlaceholder}
                aria-label={copy.namePlaceholder}
                aria-invalid={Boolean(validation) || undefined}
                className="min-w-0 flex-1"
                disabled={pending}
                onBlur={() => setNameDraft(normalizeEnvironmentNameInput(nameDraft))}
                onChange={(event) => {
                  setNameDraft(event.currentTarget.value)
                  if (validation) setValidation('')
                }}
              />
              <Button type="submit" disabled={pending}>
                {creating ? QUALITY_COPY.form.sceneOptionCreating : QUALITY_COPY.form.sceneOptionCreate}
              </Button>
            </div>
            {sceneOptionErrorCopy(validation, kind) ? (
              <p role="alert" className="m-0 text-xs text-destructive">
                {sceneOptionErrorCopy(validation, kind)}
              </p>
            ) : null}
          </form> : null}
        </div>
      </ResponsivePanel>
      <ConfirmDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(nextOpen) => {
          if (!nextOpen && !deleting) setDeleteTarget(null)
        }}
        pending={deleting}
        title={QUALITY_COPY.form.sceneOptionDeleteTitle}
        description={QUALITY_COPY.form.sceneOptionDeleteDescription(deleteTarget?.name || '')}
        cancelLabel={QUALITY_COPY.form.cancel}
        confirmLabel={QUALITY_COPY.form.sceneOptionDeleteConfirm}
        onConfirm={() => void confirmDelete()}
      />
    </>
  )
}

export function SceneOptionField({
  kind,
  id,
  label,
  placeholder,
  value,
  state,
  mutations,
  onValueChange,
}: {
  kind: QualitySceneOptionKind
  id: string
  label: string
  placeholder: string
  value: string
  state: QualitySceneOptionsState
  mutations?: QualitySceneOptionMutations
  onValueChange: (value: string) => void
}) {
  const [managerOpen, setManagerOpen] = useState(false)
  const canManage = Boolean(mutations?.canCreate || state.rows.some((item) => mutations?.canDelete(item)))
  const options = useMemo(() => {
    const fallback = kind === 'environment' ? [{
      name: QUALITY_DEFAULT_ENVIRONMENT, builtIn: true, legacy: false, usageCount: 0,
    }] : []
    const rows = state.rows.length ? state.rows : fallback
    if (!value || rows.some((item) => item.name === value)) return rows
    return [...rows, { name: value, builtIn: false, legacy: true, usageCount: 0 }]
  }, [kind, state.rows, value])

  return (
    <div className="grid min-w-0 grid-rows-[auto_1.25rem] gap-1">
      <QualitySelectField
        id={id}
        label={label}
        value={value}
        options={options.map((item) => ({ value: item.name, label: item.name }))}
        emptyLabel={QUALITY_COPY.none}
        placeholder={placeholder}
        disabled={state.status === 'loading' && state.rows.length === 0}
        onValueChange={onValueChange}
      />
      {mutations && canManage ? (
        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="h-auto w-fit px-0 py-0.5 text-xs text-primary hover:bg-transparent hover:text-primary/80"
          onClick={() => setManagerOpen(true)}
        >
          {QUALITY_COPY.form.sceneOptions[kind].manage}
        </Button>
      ) : <span aria-hidden="true" />}
      {mutations && canManage ? (
        <SceneOptionManager
          open={managerOpen}
          kind={kind}
          state={state}
          mutations={mutations}
          onOpenChange={setManagerOpen}
          onSelect={onValueChange}
        />
      ) : null}
    </div>
  )
}

export const EnvironmentField = SceneOptionField
