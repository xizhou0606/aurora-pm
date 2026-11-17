import { QUALITY_COPY } from '@/shared/copy'
import { Button } from '@/shared/ui'

import type { QualityUsersState } from '../../model/types'

export function UserDirectoryStatus({ state }: { state: QualityUsersState }) {
  if (state.status === 'error') {
    return (
      <div role="alert" className="flex flex-wrap items-center gap-2 text-xs text-destructive">
        <span>{QUALITY_COPY.users.error}</span>
        <Button type="button" size="xs" variant="outline" onClick={() => void state.retry()}>
          {QUALITY_COPY.users.retry}
        </Button>
      </div>
    )
  }
  return null
}
