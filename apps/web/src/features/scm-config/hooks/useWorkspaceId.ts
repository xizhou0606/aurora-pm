import { useCallback, useState } from 'react'

import { useWorkspaceStore } from '@/shared/state'

export function useWorkspaceIdValue(): number {
  return useWorkspaceStore((state) => state.currentWorkspaceId)
}

export function useScmConfigError() {
  const [error, setError] = useState('')
  const reportError = useCallback((message: string) => setError(message), [])
  const clearError = useCallback(() => setError(''), [])
  return { error, reportError, clearError }
}
