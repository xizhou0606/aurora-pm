import { useCallback } from 'react'

import type { InboxEnvironment } from '../components/environment'
import type { InboxBug, InboxTask, InboxWorkflowTodo } from '../model'

export function useInboxNavigation(navigate: InboxEnvironment['navigate']) {
  const openTask = useCallback((row: InboxTask) => {
    void navigate(`/workspace/work-items?task_id=${row.id}&from=inbox`)
  }, [navigate])
  const openBug = useCallback((row: InboxBug) => {
    void navigate(`/workspace/quality-management?bug_id=${row.id}&from=inbox`)
  }, [navigate])
  const openWorkflow = useCallback((row: InboxWorkflowTodo) => {
    void navigate(row.detailPath)
  }, [navigate])
  return { openTask, openBug, openWorkflow }
}
