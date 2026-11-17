import { copy } from '@/shared/copy'
import { Button } from '@/shared/ui'
import type { ProjectRefreshState } from '../model'

const contentCopy = copy.projectManagement

export function ContentRefreshFeedback({ refresh }: { refresh?: ProjectRefreshState }) {
  if (refresh?.status === 'refreshing') {
    return <span role="status" className="sr-only">{contentCopy.common.refreshing}</span>
  }
  if (refresh?.status === 'stale_error') {
    return (
      <div role="alert" className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border bg-muted px-3 py-2 text-sm">
        <span>{contentCopy.common.staleError}</span>
        <Button type="button" size="xs" variant="outline" onClick={refresh.retry}>
          {contentCopy.common.retry}
        </Button>
      </div>
    )
  }
  return null
}