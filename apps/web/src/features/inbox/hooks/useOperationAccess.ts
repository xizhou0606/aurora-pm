import { useMemo } from 'react'

import { useOperationCapabilities } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'

export type InboxOperationAccess = {
  status: 'loading' | 'error' | 'ready'
  retry: () => void
  viewTodos: boolean
}

export function useInboxOperationAccess(): InboxOperationAccess {
  const operations = useOperationCapabilities('inbox')
  return useMemo(() => ({
    status: operations.status,
    retry: operations.retry,
    viewTodos: operations.allows(PM_ACTION.InboxTodosView),
  }), [operations.allows, operations.retry, operations.status])
}
