import { BITABLE_LIBRARY_COPY as copy } from '@/shared/copy/bitables-library'
import { ResourceAccessPanel, type ResourceAccessWorkflow } from '@/features/resource-access'
import { AlertBanner, Button, ConfirmDialog, Label, ResponsivePanel, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, UserDirectorySelect, type UserDirectoryState } from '@/shared/ui'
import { BitableInput as Input } from '../controls'

import type { ResourceDialog } from '../../model/library'

export function ResourceDialogs({ dialog, folders, owners, pending, unknown, error, resourceAccess, update, close, submit, retry }: {
  dialog: ResourceDialog; folders: readonly { id: number; name: string }[]; owners: UserDirectoryState
  resourceAccess: ResourceAccessWorkflow
  pending: boolean; unknown: boolean; error: string; update: (patch: { name?: string; parentFolderId?: number; ownerUserId?: number }) => void
  close: () => void; submit: () => void; retry: () => void
}) {
  const titles = { name: copy.rename, duplicate: copy.duplicate, shortcuts: copy.shortcut, location: copy.move, owner: copy.owner, share: copy.share }
  if (dialog.kind === 'closed') return null
  if (dialog.kind === 'share') return <ResourceAccessPanel purpose={dialog.purpose ?? 'share'} open view={resourceAccess.view} onCloseRequest={resourceAccess.onCloseRequest} onOpenChange={open => { if (!open) close() }} />
  if (dialog.kind === 'delete') return <ConfirmDialog open title={dialog.shortcut ? copy.deleteShortcutTitle : copy.deleteTitle} description={dialog.shortcut ? copy.deleteShortcutDescription : copy.deleteDescription(dialog.name)} confirmLabel={unknown ? copy.retry : copy.remove} cancelLabel={copy.cancel} confirmVariant="destructive" pending={pending && !unknown} onOpenChange={open => { if (!open) close() }} onConfirm={unknown ? retry : submit} error={error || undefined} />
  const editable = true
  const destination = ['duplicate', 'shortcuts', 'location'].includes(dialog.kind)
  const required = editable && ((dialog.kind === 'name' && !dialog.name.trim()) || (dialog.kind === 'owner' && !dialog.ownerUserId))
  return <ResponsivePanel open title={titles[dialog.kind]} onOpenChange={open => { if (!open) close() }} pending={pending && !unknown} className="max-w-md"
    footer={editable ? <><Button variant="outline" disabled={pending} onClick={close}>{copy.cancel}</Button><Button disabled={Boolean(required) || (pending && !unknown)} onClick={unknown ? retry : submit}>{unknown ? copy.retry : copy.save}</Button></> : undefined}>
    <div className="grid gap-4">
      {dialog.kind === 'name' || dialog.kind === 'duplicate' ? <div className="grid gap-2"><Label htmlFor="bitable-resource-name">{copy.name}</Label><Input id="bitable-resource-name" maxLength={200} value={dialog.name} disabled={pending} onChange={event => update({ name: event.target.value })} /></div> : null}
      {editable && destination ? <div className="grid gap-2"><Label htmlFor="bitable-resource-folder">{copy.destination}</Label><Select value={dialog.parentFolderId ? String(dialog.parentFolderId) : 'root'} disabled={pending} onValueChange={value => update({ parentFolderId: value && value !== 'root' ? Number(value) : undefined })}>
        <SelectTrigger id="bitable-resource-folder"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="root">{copy.root}</SelectItem>{folders.map(folder => <SelectItem key={folder.id} value={String(folder.id)}>{folder.name}</SelectItem>)}</SelectContent>
      </Select></div> : null}
      {dialog.kind === 'owner' ? <UserDirectorySelect id="bitable-resource-owner" label={copy.ownerLabel} value={dialog.ownerUserId} state={owners} disabled={pending} onValueChange={value => update({ ownerUserId: value })} /> : null}
      {error ? <AlertBanner tone="error">{error}</AlertBanner> : null}
    </div>
  </ResponsivePanel>
}
