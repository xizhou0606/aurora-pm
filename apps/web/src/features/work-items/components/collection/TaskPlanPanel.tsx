import { PhCaretDown, PhCaretRight, PhWarningCircle, PhCheckCircle, PhCircleNotch, PhMagicWand } from '@/shared/ui/icons'
import { useEffect, useRef, useState } from 'react'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import {
  AiLoading,
  AiLoadingOverlay,
  Button,
  notifyOperationError,
  notifyOperationSuccess,
  PMScopeCascader,
  ResponsivePanel,
  Textarea,
  UserSelect,
  type PMScopeCascaderState,
} from '@/shared/ui'
import { isGeneratingStatus, useTaskPlanRuns, type TaskPlanQueueItem, type TaskPlanQueueStatus } from '../../hooks/collection/useTaskPlanRuns'
import type { PMScopeSelection } from '@/shared/api'
import type { ScopeOptionsState, WorkItemUserOption } from '../../model/types'

const aiPlanCopy = WORK_ITEMS_COPY.aiPlan
const scopeCopy = WORK_ITEMS_COPY.scope
const BATCH_POLL_MS = 2000
// 一次误选整个版本会批量烧掉大量 AI 调用；超过上限直接拒绝本次勾选。
const MAX_TASK_PLAN_BATCH = 10

type TaskPlanTarget = { id: number; title?: string }

function scopeStatusToCascader(state: ScopeOptionsState): PMScopeCascaderState {
  if (state.status === 'loading') return { status: 'loading' }
  if (state.status === 'error') return { status: 'error', onRetry: state.retry }
  return { status: 'ready', options: state.data }
}

function errorMessage(error: unknown): string {
  if (error && typeof error === 'object' && 'message' in error) {
    const message = (error as { message?: unknown }).message
    if (typeof message === 'string' && message.trim()) return message
  }
  return aiPlanCopy.failed
}

function laneLabel(lane: string | undefined): string {
  if (lane === 'frontend') return '前端'
  if (lane === 'backend') return '后端'
  return aiPlanCopy.laneFallback
}

export function TaskPlanPanel({
  open,
  onOpenChange,
  workspaceId,
  scope,
  requirements = [],
  users,
  canApply,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  workspaceId: number
  /** Workspace scope options (already permission-filtered) for the requirement picker. */
  scope: ScopeOptionsState
  /** Preselected requirements. Empty to pick inside the panel. */
  requirements?: TaskPlanTarget[]
  users: readonly WorkItemUserOption[]
  canApply: boolean
}) {
  const [selected, setSelected] = useState<TaskPlanTarget[]>(requirements)
  const [scopeValue, setScopeValue] = useState<PMScopeSelection>({
    projectSetIds: [],
    projectIds: [],
    requirementIds: requirements.map((item) => item.id),
  })
  const [prompt, setPrompt] = useState('')
  const [queue, setQueue] = useState<TaskPlanQueueItem[]>([])
  const handledRuns = useRef<Set<number>>(new Set())

  // Query ownership lives in the hook; the panel only renders and collects
  // UI state (scope/prompt/queue rows).
  const { create, apply } = useTaskPlanRuns({
    workspaceId, open, prompt, queue, setQueue, handledRuns,
    onCloseReset: () => {
      setPrompt('')
      setQueue([])
      handledRuns.current = new Set()
    },
  })

  const userOptions = users.map((user) => ({
    id: user.id,
    name: user.label,
    username: user.username,
    avatarUrl: user.avatarUrl,
  }))
  const generating = create.isPending || queue.length > 0
  const allSettled = queue.length > 0 && queue.every((item) => item.status === 'applied' || item.status === 'failed')

  return (
    <ResponsivePanel
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next)
        if (!next) {
          setPrompt('')
          setQueue([])
          handledRuns.current = new Set()
          create.reset()
          apply.reset()
        }
      }}
      title={aiPlanCopy.panelTitle}
      description={aiPlanCopy.panelDescriptionGeneric}
    >
      <div className="relative grid gap-4">
        {!generating ? (
          <>
            <div className="grid gap-1.5">
              <span className="text-xs font-semibold">{aiPlanCopy.requirementLabel}</span>
              <PMScopeCascader
                idPrefix="work-items-task-plan"
                state={scopeStatusToCascader(scope)}
                labels={{
                  label: aiPlanCopy.requirementLabel,
                  projectSet: scopeCopy.projectSet,
                  project: scopeCopy.pmProject,
                  requirement: scopeCopy.requirement,
                  selectProjectSetFirst: scopeCopy.selectProjectSetFirst,
                  selectProjectFirst: scopeCopy.selectProjectFirst,
                }}
                summary={selected.length > 0
                  ? aiPlanCopy.requirementCount(selected.length)
                  : aiPlanCopy.pickRequirements}
                selectionMode="multiple"
                presentation="embedded"
                value={scopeValue}
                onChange={(selection) => {
                  if (selection.requirementIds.length > MAX_TASK_PLAN_BATCH) {
                    notifyOperationError({ operationFeedback: {
                      success: aiPlanCopy.pickRequirements,
                      failure: aiPlanCopy.batchLimit,
                    } }, new Error(aiPlanCopy.batchLimit))
                    return
                  }
                  // 保留完整路径（项目集/版本），否则级联会把选择判为无效并清空需求列。
                  setScopeValue(selection)
                  const requirementById = new Map(
                    scope.status === 'ready'
                      ? scope.data.requirements.map((option) => [option.id, option.label])
                      : [],
                  )
                  setSelected(selection.requirementIds.map((id) => ({
                    id,
                    title: requirementById.get(id),
                  })))
                }}
              />
              {selected.length > 0 ? (
                <div className="flex flex-wrap gap-1.5">
                  {selected.map((item) => (
                    <span key={item.id} className="inline-flex max-w-64 items-center rounded-full border border-border bg-muted/40 px-2.5 py-0.5 text-xs text-foreground">
                      <span className="min-w-0 truncate">{item.title || aiPlanCopy.requirementFallback(item.id)}</span>
                    </span>
                  ))}
                </div>
              ) : null}
            </div>

            <label className="grid gap-1.5 text-xs font-semibold">
              {aiPlanCopy.prompt} <span className="font-normal text-muted-foreground">{aiPlanCopy.promptOptional}</span>
              <Textarea
                name="work-items-task-plan-prompt"
                className="min-h-24 font-normal"
                maxLength={4000}
                value={prompt}
                placeholder={aiPlanCopy.promptPlaceholder}
                onChange={(event) => setPrompt(event.currentTarget.value)}
              />
            </label>
            <Button
              type="button"
              disabled={selected.length === 0 || create.isPending}
              title={selected.length === 0 ? aiPlanCopy.pickRequirements : undefined}
              onClick={() => create.mutate(selected)}
            >
              {create.isPending ? <AiLoading size={16} label={null} /> : <PhMagicWand />}
              {create.isPending
                ? aiPlanCopy.generating
                : aiPlanCopy.generateCount(selected.length)}
            </Button>
            {create.isError ? (
              <p role="alert" className="m-0 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {errorMessage(create.error)}
              </p>
            ) : null}
          </>
        ) : (
          <>
            <div className="grid gap-2">
              {queue.map((item) => (
                <TaskPlanQueueRow
                  key={item.runId}
                  item={item}
                  userOptions={userOptions}
                  canApply={canApply}
                  applying={apply.isPending && item.status === 'applying'}
                  onAssign={(lane, userId) => setQueue((previous) => previous.map((entry) => (
                    entry.runId === item.runId
                      ? { ...entry, assignees: { ...entry.assignees, [lane]: userId } }
                      : entry
                  )))}
                  onApply={() => apply.mutate({
                    runId: item.runId,
                    title: item.title,
                    assignees: item.assignees,
                  })}
                />
              ))}
            </div>
            {allSettled ? (
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setQueue([])
                  handledRuns.current = new Set()
                }}
              >
                {aiPlanCopy.anotherBatch}
              </Button>
            ) : null}
          </>
        )}
        {create.isPending || queue.some((item) => isGeneratingStatus(item.status)) ? (
          <AiLoadingOverlay
            label={create.isPending
              ? aiPlanCopy.generating
              : aiPlanCopy.batchRunning(
                queue.filter((item) => isGeneratingStatus(item.status)).length || queue.length,
              )}
          />
        ) : null}
      </div>
    </ResponsivePanel>
  )
}

function TaskPlanQueueRow({
  item,
  userOptions,
  canApply,
  applying,
  onAssign,
  onApply,
}: {
  item: TaskPlanQueueItem
  userOptions: readonly { id: number; name: string; username?: string; avatarUrl?: string }[]
  canApply: boolean
  applying: boolean
  onAssign: (lane: 'frontend' | 'backend', userId: number | null) => void
  onApply: () => void
}) {
  const ready = item.status === 'completed'
  const [detailOpen, setDetailOpen] = useState(true)
  const chains = item.plan?.chains ?? []
  return (
    <div className="grid gap-2 rounded-lg border border-border px-3 py-2">
      <div className="flex items-center justify-between gap-2">
        <span className="min-w-0 truncate text-sm text-foreground">{item.title}</span>
        <QueueStatus item={item} />
      </div>
      {ready ? (
        <>
          <div className="grid gap-1">
            <Button
              type="button"
              variant="ghost"
              size="xs"
              className="h-6 w-fit justify-start gap-1 px-0 text-xs text-muted-foreground hover:bg-transparent"
              aria-expanded={detailOpen}
              onClick={() => setDetailOpen((previous) => !previous)}
            >
              {detailOpen ? <PhCaretDown aria-hidden="true" className="size-3.5" /> : <PhCaretRight aria-hidden="true" className="size-3.5" />}
              {detailOpen ? aiPlanCopy.detailHide : aiPlanCopy.detailShow}
            </Button>
            {detailOpen ? (
              <div className="grid gap-2">
                {chains.map((chain, chainIndex) => {
                  const laneName = laneLabel(chain.lane)
                  const taskCount = chain.tasks?.length ?? 0
                  return (
                    <div key={chainIndex} className="rounded-md bg-muted/40 p-2.5">
                      <div className="flex items-center justify-between gap-2">
                        <span className="text-xs font-semibold text-foreground">
                          {aiPlanCopy.mainTask(item.title + '-' + laneName)}
                        </span>
                        <span className="shrink-0 text-xs text-muted-foreground">{aiPlanCopy.laneTasks(taskCount)}</span>
                      </div>
                      {chain.summary ? <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{chain.summary}</p> : null}
                      <ul className="mt-1.5 mb-0 grid gap-0.5 pl-4">
                        {(chain.tasks ?? []).map((task, taskIndex) => (
                          <li key={taskIndex} className="list-disc text-xs leading-5 text-foreground">
                            {task.title || aiPlanCopy.noTaskTitle}
                          </li>
                        ))}
                      </ul>
                    </div>
                  )
                })}
              </div>
            ) : null}
          </div>
          <div className="grid gap-2 border-t border-border pt-2 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
            <div className="grid gap-0.5 text-xs font-semibold text-foreground">
              {aiPlanCopy.assigneeFrontend}
              <UserSelect
                label={aiPlanCopy.assigneeFrontend}
                value={item.assignees.frontend ?? undefined}
                options={userOptions}
                clearable
                emptyLabel={aiPlanCopy.assigneeUnassigned}
                onValueChange={(value) => onAssign('frontend', value ?? null)}
              />
            </div>
            <div className="grid gap-0.5 text-xs font-semibold text-foreground">
              {aiPlanCopy.assigneeBackend}
              <UserSelect
                label={aiPlanCopy.assigneeBackend}
                value={item.assignees.backend ?? undefined}
                options={userOptions}
                clearable
                emptyLabel={aiPlanCopy.assigneeUnassigned}
                onValueChange={(value) => onAssign('backend', value ?? null)}
              />
            </div>
            <Button
              type="button"
              size="sm"
              disabled={!canApply || applying}
              title={!canApply ? aiPlanCopy.applyNotAllowed : undefined}
              onClick={onApply}
            >
              {applying ? <PhCircleNotch className="animate-spin" /> : <PhCheckCircle />}
              {aiPlanCopy.createTask}
            </Button>
          </div>
        </>
      ) : null}
      {item.status === 'failed' && item.error ? (
        <p className="m-0 text-xs text-muted-foreground">{item.error}</p>
      ) : null}
    </div>
  )
}


function QueueStatus({ item }: { item: TaskPlanQueueItem }) {
  if (item.status === 'applied') {
    return (
      <span className="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-emerald-700 dark:text-emerald-300">
        <PhCheckCircle className="size-3.5" aria-hidden="true" />
        {aiPlanCopy.applied(item.createdCount ?? 0)}
      </span>
    )
  }
  if (item.status === 'failed') {
    return (
      <span className="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-destructive" title={item.error}>
        <PhWarningCircle className="size-3.5" aria-hidden="true" />
        {aiPlanCopy.failed}
      </span>
    )
  }
  if (item.status === 'completed') {
    return (
      <span className="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-amber-700 dark:text-amber-300">
        {aiPlanCopy.statusReady(item.tasksCount ?? 0)}
      </span>
    )
  }
  return (
    <span className="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-primary">
      {item.status === 'applying' ? <PhCircleNotch className="size-3.5 animate-spin" aria-hidden="true" /> : <AiLoading size={14} label={null} />}
      {item.status === 'applying' ? aiPlanCopy.applying : aiPlanCopy.running}
    </span>
  )
}
