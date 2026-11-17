// Query ownership for the AI task-plan batch panel. Transport calls, cache
// invalidation and polling live here so the panel component stays a pure
// renderer (native-feature-architecture gate).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import { notifyOperationError, notifyOperationSuccess } from '@/shared/ui'

import {
  applyTaskPlanRun,
  createTaskPlanRun,
  fetchTaskPlanRun,
  type TaskPlanRun,
} from '../../api/ai-plan'
import { invalidateTaskProgressReads, workItemsKey } from '../../model/query-keys'

const aiPlanCopy = WORK_ITEMS_COPY.aiPlan
const BATCH_POLL_MS = 2000

export type TaskPlanQueueStatus = 'queued' | 'running' | 'completed' | 'applying' | 'applied' | 'failed'

export type TaskPlanQueueItem = {
  runId: number
  requirementId: number
  title: string
  status: TaskPlanQueueStatus
  tasksCount?: number
  plan?: NonNullable<NonNullable<TaskPlanRun['output']>['taskPlan']>
  assignees: { frontend: number | null; backend: number | null }
  createdCount?: number
  error?: string
}

export function isGeneratingStatus(status: TaskPlanQueueStatus): boolean {
  return status === 'queued' || status === 'running'
}

function isTerminalFetch(status: string | undefined): boolean {
  return status === 'completed' || status === 'failed' || status === 'applied'
}

export function useTaskPlanRuns(params: {
  workspaceId: number
  open: boolean
  prompt: string
  queue: TaskPlanQueueItem[]
  setQueue: React.Dispatch<React.SetStateAction<TaskPlanQueueItem[]>>
  handledRuns: React.RefObject<Set<number>>
  /** Extra UI reset when the panel closes (panel keeps its own input state). */
  onCloseReset?: () => void
}) {
  const { workspaceId, open, prompt, queue, setQueue, handledRuns, onCloseReset } = params
  const client = useQueryClient()

  useEffect(() => {
    if (!open) {
      onCloseReset?.()
      create.reset()
      apply.reset()
    }
  }, [open])

  const activeRunIds = queue.filter((item) => isGeneratingStatus(item.status)).map((item) => item.runId)
  const pollQuery = useQuery<TaskPlanRun[]>({
    queryKey: ['work-items', 'ai-plan-batch', workspaceId, activeRunIds.join(',')],
    enabled: open && activeRunIds.length > 0,
    refetchInterval: BATCH_POLL_MS,
    refetchIntervalInBackground: false,
    queryFn: async ({ signal }) => (
      Promise.all(activeRunIds.map((runId) => fetchTaskPlanRun(runId, signal)))
    ),
  })

  const create = useMutation({
    mutationFn: async (targets: TaskPlanTarget[]) => (
      Promise.all(targets.map((target) => createTaskPlanRun(target.id, prompt.trim() || undefined)
        .then((run) => ({ run, target }))))
    ),
    onSuccess: (results) => {
      setQueue(results.map(({ run, target }) => ({
        runId: run.id,
        requirementId: target.id,
        title: target.title || aiPlanCopy.requirementFallback(target.id),
        status: (run.status === 'completed' ? 'completed' : 'queued') as TaskPlanQueueStatus,
        assignees: { frontend: null, backend: null },
      })))
      handledRuns.current = new Set()
      notifyOperationSuccess({ operationFeedback: {
        success: aiPlanCopy.batchStarted(results.length),
        failure: aiPlanCopy.failed,
      } })
    },
    onError: (error) => {
      notifyOperationError({ operationFeedback: {
        success: aiPlanCopy.batchStarted(0),
        failure: errorMessage(error),
      } }, error)
    },
  })

  const apply = useMutation({
    mutationFn: (vars: {
      runId: number
      title: string
      assignees: { frontend: number | null; backend: number | null }
    }) => applyTaskPlanRun(vars.runId, {
      frontend: vars.assignees.frontend,
      backend: vars.assignees.backend,
    }),
    onSuccess: (applied, vars) => {
      const createdCount = (applied.output?.createdTasks ?? []).length
      setQueue((previous) => previous.map((item) => (
        item.runId === vars.runId
          ? { ...item, status: 'applied' as const, createdCount }
          : item
      )))
      invalidateTaskProgressReads(client, workspaceId)
      void client.invalidateQueries({ queryKey: workItemsKey(workspaceId, 'collection') })
      void client.invalidateQueries({ queryKey: workItemsKey(workspaceId, 'collection', 'gantt') })
      notifyOperationSuccess({ operationFeedback: {
        success: aiPlanCopy.batchApplied(vars.title, createdCount),
        failure: aiPlanCopy.failed,
      } })
    },
    onError: (error, vars) => {
      setQueue((previous) => previous.map((item) => (
        item.runId === vars.runId ? { ...item, status: 'failed' as const, error: errorMessage(error) } : item
      )))
      notifyOperationError({ operationFeedback: {
        success: aiPlanCopy.batchApplied(vars.title, 0),
        failure: `${aiPlanCopy.failed}（${vars.title}）：${errorMessage(error)}`,
      } }, error)
    },
  })

  // A completed plan waits for manual assignees + per-row apply; failures
  // surface as toasts and stay in the queue list.
  useEffect(() => {
    if (!pollQuery.data) return
    for (const run of pollQuery.data) {
      if (!isTerminalFetch(run.status) || handledRuns.current.has(run.id)) continue
      handledRuns.current.add(run.id)
      const item = queue.find((entry) => entry.runId === run.id)
      if (run.status === 'completed') {
        const plan = run.output?.taskPlan
        const tasksCount = (plan?.chains ?? [])
          .reduce((total, chain) => total + (chain.tasks?.length ?? 0), 0)
        setQueue((previous) => previous.map((entry) => (
          entry.runId === run.id ? { ...entry, status: 'completed' as const, tasksCount, plan } : entry
        )))
        continue
      }
      const message = run.error_message || aiPlanCopy.failed
      setQueue((previous) => previous.map((entry) => (
        entry.runId === run.id ? { ...entry, status: 'failed' as const, error: message } : entry
      )))
      notifyOperationError({ operationFeedback: {
        success: aiPlanCopy.batchStarted(0),
        failure: `${aiPlanCopy.failed}（${item?.title ?? run.subject_id ?? ''}）：${message}`,
      } }, new Error(message))
    }
  }, [pollQuery.data])

  return { create, apply }
}

type TaskPlanTarget = { id: number; title?: string }

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}
