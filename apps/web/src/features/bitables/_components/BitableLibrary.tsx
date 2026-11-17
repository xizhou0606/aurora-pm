import { useRef, useState } from 'react'
import { Button, Popover, PopoverTrigger, PopoverContent, PopoverTitle, AlertBanner } from '@/shared/ui'
import { Icon, ICON_CONTEXT_SIZES } from '@/shared/ui/icons'
import { BITABLE_COPY as copy } from '@/shared/copy/bitables'
import { useBitableLibrary } from '../hooks/useBitableLibrary'
import { useBitableDirectory } from '../hooks/useBitableDirectory'
import { ResourceItem } from './library/ResourceItem'
import { ResourceDialogs } from './library/ResourceDialogs'
import { BITABLE_LIBRARY_COPY as libraryCopy } from '@/shared/copy/bitables-library'

export function BitableDirectory({ selected, onSelect, parentId, depth = 0, favorites = false, favoritesAvailable = false, folders = [] }: { selected?: string; onSelect: (id: string) => void; parentId?: number; depth?: number; favorites?: boolean; favoritesAvailable?: boolean; folders?: readonly { id: number; name: string }[] }) {
  const flow = useBitableLibrary()
  const directory = useBitableDirectory(flow.resources, flow.canRead, selected, onSelect)
  if (!flow.canRead) return null
  const entries = directory.entries.filter(entry => favorites ? !entry.shortcut && entry.favorited : entry.parentFolderId === parentId || (!parentId && !folders.some(folder => folder.id === entry.parentFolderId)))
  const fallbackFavorites = !parentId && !favorites && !favoritesAvailable ? directory.entries.filter(entry => !entry.shortcut && entry.favorited) : []
  return <>
    {flow.error && <li className="p-2"><AlertBanner tone="error" action={<Button variant="ghost" onClick={flow.retry}>{copy.retry}</Button>}>{flow.error}</AlertBanner></li>}
    {directory.error && <li className="p-2"><AlertBanner tone="error" action={directory.unknown ? <Button variant="ghost" onClick={directory.retry}>{libraryCopy.retry}</Button> : undefined}>{directory.error}</AlertBanner></li>}
    {directory.feedback && <li role="status" className="px-2 text-xs text-muted-foreground">{directory.feedback}</li>}
    {fallbackFavorites.length ? <li><details open><summary className="cursor-pointer rounded-md px-2 py-1.5 text-sm hover:bg-muted/60">{libraryCopy.favoritesFolder}</summary><ul className="m-0 list-none p-0">{fallbackFavorites.map(item => <ResourceItem key={item.id} item={item} selected={item.targetId === selected} depth={1} pending={directory.pending} onSelect={onSelect} onAction={(id, action) => { void directory.act(id, action) }} />)}</ul></details></li> : null}
    {entries.map(item => <ResourceItem key={item.id} item={item} selected={item.targetId === selected} depth={depth} pending={directory.pending} onSelect={onSelect} onAction={(id, action) => { void directory.act(id, action) }} />)}
    <ResourceDialogs dialog={directory.dialog} folders={folders} owners={directory.owners} pending={directory.pending} unknown={directory.unknown} error={directory.error} resourceAccess={directory.resourceAccess} update={directory.update} close={directory.close} submit={directory.submit} retry={directory.retry} />
  </>
}
export function BitableCreateButton({ createDocument, onCreated }: { createDocument?: () => void; onCreated: (id: string) => void }) {
  const flow = useBitableLibrary(), [open, setOpen] = useState(false)
  const name = useRef('')
  if (!flow.canCreate && !createDocument) return null
  const submit = async () => {
    name.current ||= copy.untitled(Date.now())
    const resource = await flow.create(name.current)
    if (resource) { setOpen(false); name.current = ''; onCreated(resource.id) }
  }
  return <Popover open={open} onOpenChange={setOpen}>
    <PopoverTrigger render={<Button size="icon-sm" variant="ghost" />} aria-label={copy.createResource} title={copy.createResource}><Icon name="hugeicons:add-01" size={ICON_CONTEXT_SIZES.control} aria-hidden="true" /></PopoverTrigger>
    <PopoverContent className="w-48 p-1" align="end">
      <PopoverTitle className="sr-only">{copy.createResource}</PopoverTitle>
      {createDocument && <Button className="w-full justify-start" variant="ghost" disabled={flow.pending} onClick={() => { setOpen(false); createDocument() }}><Icon name="hugeicons:file-01" />{copy.document}</Button>}
      {flow.canCreate && <Button className="w-full justify-start" variant="ghost" disabled={flow.pending} onClick={() => void submit()}><Icon name="hugeicons:table-01" />{copy.title}</Button>}
      {flow.error && <AlertBanner tone="error">{flow.error}</AlertBanner>}
    </PopoverContent>
  </Popover>
}
