import { Button, AlertBanner } from '@/shared/ui'
import { PhPencil } from '@/shared/ui/icons'
import { BITABLE_LIBRARY_COPY as copy } from '@/shared/copy/bitables-library'
import { useBitableDirectory } from '../../hooks/useBitableDirectory'
import type { Resource } from '../../model/types'
import { ResourceDialogs } from './ResourceDialogs'

export function ResourceTitle({ resource, readOnly }: { resource: Resource; readOnly: boolean }) {
  const flow = useBitableDirectory([resource], true, resource.id, () => undefined)
  const canRename = !readOnly && flow.entries.some(entry => entry.id === resource.id && entry.actions.includes('name'))
  return <>
    <div className="flex min-w-0 items-center gap-1">
      <h2 className="m-0 truncate text-sm font-medium" title={resource.name}>{resource.name}</h2>
      {canRename ? <Button size="icon-xs" variant="ghost" disabled={flow.pending} aria-label={copy.rename} title={copy.rename} onClick={() => { void flow.act(resource.id, 'name') }}><PhPencil aria-hidden="true" /></Button> : null}
    </div>
    {flow.error ? <AlertBanner tone="error">{flow.error}</AlertBanner> : null}
    <ResourceDialogs dialog={flow.dialog} folders={[]} owners={flow.owners} pending={flow.pending} unknown={flow.unknown} error={flow.error} resourceAccess={flow.resourceAccess} update={flow.update} close={flow.close} submit={flow.submit} retry={flow.retry} />
  </>
}
