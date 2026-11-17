type RetryIntent = () => void

export type ProjectRefreshState =
  | { status: 'idle' }
  | { status: 'refreshing' }
  | { status: 'stale_error'; retry: RetryIntent }
