import { useEffect, useState } from 'react'

import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import { Button, Checkbox, Input, Label, ResponsivePanel, Textarea } from '@/shared/ui'
import type { ManagedWorkspace, WorkspaceDraft } from '../model'

type WorkspaceEditorPanelProps = {
  workspace: ManagedWorkspace | null
  open: boolean
  pending: boolean
  error: boolean
  onClose: () => void
  onSubmit: (draft: WorkspaceDraft) => void
}

export function WorkspaceEditorPanel({
  workspace,
  open,
  pending,
  error,
  onClose,
  onSubmit,
}: WorkspaceEditorPanelProps) {
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [description, setDescription] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [invalid, setInvalid] = useState(false)

  useEffect(() => {
    if (!open) return
    setName(workspace?.name ?? '')
    setSlug(workspace?.slug ?? '')
    setDescription(workspace?.description ?? '')
    setEnabled(workspace?.enabled ?? true)
    setInvalid(false)
  }, [open, workspace])

  const submit = () => {
    if (!name.trim() || !slug.trim()) {
      setInvalid(true)
      return
    }
    onSubmit({
      ...(workspace ? { id: workspace.id } : {}),
      name: name.trim(),
      slug: slug.trim().toLowerCase(),
      description: description.trim(),
      enabled,
    })
  }

  return (
    <ResponsivePanel
      open={open}
      onOpenChange={(next) => { if (!next) onClose() }}
      title={workspace ? COPY.workspaces.editTitle : COPY.workspaces.createTitle}
      description={COPY.workspaces.formDescription}
      closeLabel={COPY.common.cancel}
      pending={pending}
      className="max-w-xl"
      footer={(
        <>
          <Button type="button" variant="outline" disabled={pending} onClick={onClose}>{COPY.common.cancel}</Button>
          <Button type="button" disabled={pending} onClick={submit}>{pending ? COPY.common.saving : COPY.common.save}</Button>
        </>
      )}
    >
      <div className="grid gap-4">
        <div className="grid gap-2">
          <Label htmlFor="workspace-admin-name">{COPY.workspaces.nameLabel}</Label>
          <Input id="workspace-admin-name" value={name} placeholder={COPY.workspaces.namePlaceholder} onChange={(event) => { setName(event.target.value); setInvalid(false) }} />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="workspace-admin-slug">{COPY.workspaces.slugLabel}</Label>
          <Input id="workspace-admin-slug" value={slug} disabled={Boolean(workspace)} placeholder={COPY.workspaces.slugPlaceholder} onChange={(event) => { setSlug(event.target.value); setInvalid(false) }} />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="workspace-admin-description">{COPY.workspaces.descriptionLabel}</Label>
          <Textarea id="workspace-admin-description" value={description} placeholder={COPY.workspaces.descriptionPlaceholder} onChange={(event) => setDescription(event.target.value)} />
        </div>
        {workspace && !workspace.isDefault ? (
          <label className="flex items-center gap-2 text-sm">
            <Checkbox checked={enabled} onCheckedChange={(checked) => setEnabled(checked === true)} aria-label={COPY.workspaces.enabledLabel} />
            <span>{COPY.workspaces.enabledLabel}</span>
          </label>
        ) : null}
        {invalid ? <p role="alert" className="text-sm text-destructive">{COPY.workspaces.required}</p> : null}
        {error ? <p role="alert" className="text-sm text-destructive">{COPY.workspaces.saveError}</p> : null}
      </div>
    </ResponsivePanel>
  )
}
