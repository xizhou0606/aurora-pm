import { useEffect, useRef, useState } from 'react'
import { AlertBanner, Button, EmptyState, ResponsivePanel, type RichTextBitableReference } from '@/shared/ui'
import { BitableInput as Input } from './controls'
import { BITABLE_COPY as copy } from '@/shared/copy/bitables'
import { bitableClient } from '../api/client'
import { useBitableLibrary } from '../hooks/useBitableLibrary'
import { useSessionStore, useWorkspaceStore } from '@/app/state'

// Only the application calls this public integration hook. The editor owns no table queries.
export function useBitablePicker() {
  const flow = useBitableLibrary(), [open, setOpen] = useState(false), [name, setName] = useState(''), [error, setError] = useState(false), [pending, setPending] = useState(false)
  const resolver = useRef<((value: RichTextBitableReference | null) => void) | null>(null)
  const busy = useRef(false), generation = useRef(0)
  const workspaceId = useWorkspaceStore(state => state.currentWorkspaceId), sessionRevision = useSessionStore(state => state.revision)
  useEffect(() => { generation.current++; resolver.current?.(null); resolver.current = null; setOpen(false); setPending(false); busy.current = false }, [workspaceId, sessionRevision])
  useEffect(() => () => { generation.current++; resolver.current?.(null) }, [])
  const finish = (value: RichTextBitableReference | null) => { resolver.current?.(value); resolver.current = null; setOpen(false) }
  const select = async (resourceId: string, tableId: string) => {
    if (busy.current) return
    const run = generation.current; busy.current = true; setPending(true); setError(false)
    try {
      const bootstrap = await bitableClient.bootstrap({ resourceId, tableId })
      if (run !== generation.current) return
      const viewId = bootstrap.views[0]?.id
      if (!bootstrap.capabilities.canRead || !viewId) { setError(true); return }
      finish({ schemaVersion: 1, resourceId, tableId, viewId, height: 420 })
    } catch { if (run === generation.current) setError(true) }
    finally { if (run === generation.current) { busy.current = false; setPending(false) } }
  }
  return {
    pick: () => new Promise<RichTextBitableReference | null>(resolve => { resolver.current?.(null); resolver.current = resolve; setError(false); setOpen(true) }),
    panel: <ResponsivePanel title={copy.selectResource} open={open} onOpenChange={value => { if (!value && !pending) finish(null) }} pending={pending || flow.pending}>
      {(error || flow.error) && <AlertBanner tone="error">{copy.error}</AlertBanner>}
      <div className="grid max-h-80 gap-1 overflow-auto">{flow.resources.map(resource => resource.tables.map(table => <Button key={table.id} variant="ghost" disabled={pending} onClick={() => void select(resource.id, table.id)}>{copy.tableLabel(resource.name, table.name)}</Button>))}</div>
      {!flow.loading && !flow.resources.length && <EmptyState title={copy.noResources} />}
      {flow.canCreate && <div className="mt-3 flex gap-2"><Input aria-label={copy.name} value={name} maxLength={200} disabled={pending || flow.pending} onChange={event => setName(event.target.value)} /><Button disabled={!name.trim() || pending || flow.pending} onClick={() => { void flow.create(name).then(resource => { if (resource?.tables[0]) void select(resource.id, resource.tables[0].id) }) }}>{copy.create}</Button></div>}
    </ResponsivePanel>,
  }
}
