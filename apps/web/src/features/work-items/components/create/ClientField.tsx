import { useMemo, useState, type FormEvent } from 'react'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import { Button, ConfirmDialog, Input, Label, ResponsivePanel, TableActionButton } from '@/shared/ui'
import type {
  WorkItemClientOption,
  WorkItemClientOptionMutations,
  WorkItemClientOptionsState,
} from '../../model/types'
import { WorkItemsSelect } from '../collection/SelectField'

function normalizedName(value: string) {
  return value.trim().replace(/ +/g, ' ')
}

export function ClientField({
  id,
  value,
  state,
  mutations,
  disabled,
  onValueChange,
}: {
  id: string
  value?: number
  state: WorkItemClientOptionsState
  mutations: WorkItemClientOptionMutations
  disabled?: boolean
  onValueChange: (value?: number) => void
}) {
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState('')
  const [editing, setEditing] = useState<WorkItemClientOption>()
  const [deleting, setDeleting] = useState<WorkItemClientOption>()
  const [error, setError] = useState('')
  const rows = state.rows
  const options = useMemo(() => rows.map((option) => ({ value: String(option.id), label: option.name })), [rows])
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    const name = normalizedName(draft)
    if (!name || [...name].length > 64 || !/^[\p{L}\p{N}._ /-]+$/u.test(name)) {
      setError(WORK_ITEMS_COPY.clientOptions.invalid)
      return
    }
    if (rows.some((option) => option.id !== editing?.id && option.name.toLocaleLowerCase() === name.toLocaleLowerCase())) {
      setError(WORK_ITEMS_COPY.clientOptions.duplicate)
      return
    }
    try {
      const saved = editing
        ? await mutations.update(editing, name)
        : await mutations.create(name)
      onValueChange(saved.id)
      setDraft('')
      setEditing(undefined)
      setError('')
    } catch {
      setError(WORK_ITEMS_COPY.clientOptions.failed)
    }
  }
  const close = (next: boolean) => {
    if (!next && !mutations.pending) {
      setDraft('')
      setEditing(undefined)
      setError('')
    }
    setOpen(next)
  }
  return (
    <div className="grid min-w-0 gap-1.5">
      <WorkItemsSelect
        id={id}
        label={WORK_ITEMS_COPY.create.client}
        value={value ? String(value) : ''}
        options={options}
        placeholder={state.status === 'loading'
          ? WORK_ITEMS_COPY.clientOptions.loading
          : WORK_ITEMS_COPY.create.clientPlaceholder}
        clearable
        disabled={disabled || state.status === 'loading'}
        onValueChange={(next) => onValueChange(next ? Number(next) : undefined)}
      />
      {mutations.canCreate || mutations.canUpdate || rows.some(mutations.canDelete) ? (
        <Button type="button" variant="link" className="h-8 w-fit px-0" onClick={() => setOpen(true)}>
          {WORK_ITEMS_COPY.clientOptions.manage}
        </Button>
      ) : null}
      <ResponsivePanel
        open={open}
        onOpenChange={close}
        pending={mutations.pending}
        title={WORK_ITEMS_COPY.clientOptions.title}
        className="max-w-lg"
      >
        <div className="grid gap-4">
          <div className="h-52 overflow-y-auto rounded-md border border-border bg-muted/20 p-1">
            {rows.map((option) => (
              <div key={option.id} className="flex min-h-10 items-center justify-between gap-3 rounded px-2">
                <span className="min-w-0 truncate text-sm">{option.name}</span>
                <div className="flex shrink-0 items-center gap-2">
                  {mutations.canUpdate ? (
                    <TableActionButton text={WORK_ITEMS_COPY.clientOptions.edit} label={WORK_ITEMS_COPY.clientOptions.editLabel(option.name)} onClick={() => {
                      setEditing(option); setDraft(option.name); setError('')
                    }} />
                  ) : null}
                  {mutations.canDelete(option) ? (
                    <TableActionButton text={WORK_ITEMS_COPY.clientOptions.delete} label={WORK_ITEMS_COPY.clientOptions.deleteLabel(option.name)} onClick={() => setDeleting(option)} />
                  ) : option.usageCount > 0 ? (
                    <span className="text-xs text-muted-foreground">{WORK_ITEMS_COPY.clientOptions.inUse(option.usageCount)}</span>
                  ) : null}
                </div>
              </div>
            ))}
          </div>
          {(mutations.canCreate || (mutations.canUpdate && editing)) ? (
            <form className="grid gap-2" onSubmit={(event) => void submit(event)}>
              <Label htmlFor={`${id}-client-name`}>{editing ? WORK_ITEMS_COPY.clientOptions.editName : WORK_ITEMS_COPY.clientOptions.addName}</Label>
              <Input id={`${id}-client-name`} value={draft} maxLength={64} onChange={(event) => { setDraft(event.currentTarget.value); setError('') }} />
              {error ? <p role="alert" className="m-0 text-sm text-destructive">{error}</p> : null}
              <div className="flex justify-end gap-2">
                {editing ? <Button type="button" variant="outline" onClick={() => { setEditing(undefined); setDraft(''); setError('') }}>{WORK_ITEMS_COPY.create.cancel}</Button> : null}
                <Button type="submit" disabled={mutations.pending}>{editing ? WORK_ITEMS_COPY.clientOptions.save : WORK_ITEMS_COPY.clientOptions.add}</Button>
              </div>
            </form>
          ) : null}
        </div>
      </ResponsivePanel>
      <ConfirmDialog
        open={Boolean(deleting)}
        onOpenChange={(next) => { if (!next) setDeleting(undefined) }}
        pending={mutations.pending}
        title={WORK_ITEMS_COPY.clientOptions.deleteTitle}
        description={WORK_ITEMS_COPY.clientOptions.deleteDescription(deleting?.name || '')}
        cancelLabel={WORK_ITEMS_COPY.create.cancel}
        confirmLabel={WORK_ITEMS_COPY.clientOptions.delete}
        onConfirm={() => {
          if (!deleting) return
          void mutations.remove(deleting).then(() => {
            if (value === deleting.id) onValueChange(undefined)
            setDeleting(undefined)
          }).catch(() => setError(WORK_ITEMS_COPY.clientOptions.failed))
        }}
      />
    </div>
  )
}
