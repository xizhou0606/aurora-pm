import { accessRequestsCopy, RESOURCE_ACCESS_COPY as copy } from '@/shared/copy'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/shared/ui'
import type { AccessRequestResourceType, RequestGrantTerms } from '../model/requests'

export function RequestGrantFields({ resourceType, terms, busy, change }: {
 resourceType: AccessRequestResourceType; terms: RequestGrantTerms; busy: boolean; change: (patch: Partial<RequestGrantTerms>) => void
}) {
 const labels = accessRequestsCopy(resourceType)
 return <div className="grid grid-cols-2 gap-2">
  <Select value={terms.grantLevel} disabled={busy} onValueChange={value => { if (value === 'viewer' || value === 'editor') change({ grantLevel: value }) }}>
   <SelectTrigger aria-label={labels.grantLevel}><SelectValue>{copy.level[terms.grantLevel]}</SelectValue></SelectTrigger>
   <SelectContent><SelectItem value="viewer">{copy.level.viewer}</SelectItem><SelectItem value="editor">{copy.level.editor}</SelectItem></SelectContent>
  </Select>
  <Select value={terms.grantDuration} disabled={busy} onValueChange={value => { if (value === '24h' || value === '7d' || value === 'never') change({ grantDuration: value }) }}>
   <SelectTrigger aria-label={labels.grantDuration}><SelectValue>{({ '24h': copy.links.expiry24h, '7d': copy.links.expiry7d, never: copy.links.expiryNever })[terms.grantDuration]}</SelectValue></SelectTrigger>
   <SelectContent><SelectItem value="24h">{copy.links.expiry24h}</SelectItem><SelectItem value="7d">{copy.links.expiry7d}</SelectItem><SelectItem value="never">{copy.links.expiryNever}</SelectItem></SelectContent>
  </Select>
 </div>
}
