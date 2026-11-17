import { INBOX_COPY } from '@/shared/copy'
import type { DataTableState } from '@/shared/ui'
import type { CollectionState, InboxFeedId, InboxFeedRow } from '../../model'

export function feedTableState<T extends InboxFeedRow>(
  id: InboxFeedId,
  state: CollectionState<T>,
): DataTableState<T> {
  const copy = INBOX_COPY.feed[id]
  switch (state.status) {
    case 'loading':
      return {
        status: 'loading',
        label: copy.loading,
        skeletonRows: id === 'mention' ? 4 : 6,
      }
    case 'error':
      return {
        status: 'error',
        title: copy.error,
        onRetry: state.retry,
        retryLabel: INBOX_COPY.actions.retry,
      }
    case 'empty':
      return {
        status: 'empty',
        title: copy.empty
      }
    case 'ready':
      return {
        status: 'ready',
        rows: state.rows,
        refresh: state.refresh,
        refreshingLabel: INBOX_COPY.page.updating,
        staleErrorLabel: INBOX_COPY.page.staleError,
      }
  }
}
