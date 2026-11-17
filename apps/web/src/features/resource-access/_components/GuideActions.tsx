import { RESOURCE_ACCESS_COPY } from '@/shared/copy/resource-access'
import type { GuideMembership } from '../api/project-guide'
import { ProjectAccessRequests } from './ProjectAccessRequests'

// Entry states come from the single detail response; this component performs no reads.
export function GuideActions({ memberships }: { memberships: GuideMembership[] }) {
  return <div className="flex flex-wrap items-center gap-2">
    {memberships.map(item => {
      const label = RESOURCE_ACCESS_COPY.projectGuide.membership[item.kind]
      if (item.status === 'joined' || item.status === 'pending' || item.status === 'unavailable') {
        return <span key={item.kind} role="status" className="inline-flex h-7 items-center rounded-md border border-border bg-muted px-2.5 text-xs text-muted-foreground">{label[item.status]}</span>
      }
      return <ProjectAccessRequests key={item.kind} prominent target={{ kind: item.kind, id: item.id }} label={label.apply} />
    })}
  </div>
}
