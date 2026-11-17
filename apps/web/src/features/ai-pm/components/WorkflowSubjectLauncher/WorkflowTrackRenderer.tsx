import { PhCalendarCheck, PhCheckCircle, PhCaretRight, PhWarningCircle, PhClock, PhGitBranch, PhSparkle, PhUser } from '@/shared/ui/icons'
import { copy } from '@/shared/copy'
import { AiLoading, Button } from '@/shared/ui'
import {
  stageTone,
  stageIcon,
  formatActualCompletedAt,
  formatNodePlan,
  resolveNodePlanDates,
  nodeAssignee,
  structuralNodeTypes,
  type TimelineNode,
} from './utils'
import { workflowNodeIsOverdue, workflowSchedule } from '../WorkflowSchedule'
import type { WorkflowInstance } from '../../model/types'
import type { WorkflowRuntimeLane, WorkflowRuntimeTrackItem } from '../../utils/workflow-runtime-track'

const subjectCopy = copy.aiPM.workflowSubject

export function renderStage({
  node,
  instance,
  activeNodeById,
  density = 'track',
  compactChrome = false,
  vertical = false,
  openWorkflowNode,
}: {
  node: TimelineNode
  instance: WorkflowInstance
  activeNodeById: Map<string, WorkflowInstance['current_nodes'][number]>
  density?: 'track' | 'branch'
  compactChrome?: boolean
  vertical?: boolean
  openWorkflowNode: (node: TimelineNode) => void
}) {
  const activeNode = activeNodeById.get(node.id)
  const aiStatus = node.ai_execution?.status
  const aiBusy = aiStatus === 'queued' || aiStatus === 'running'
  const aiFinished = aiStatus === 'completed' || aiStatus === 'applied'
  const completedAt = formatActualCompletedAt(node.actual_completed_at)
  const planDates = resolveNodePlanDates(instance, node)
  const subjectSchedule = workflowSchedule(instance)
  const overdue = workflowNodeIsOverdue({ ...node, planned_end_at: planDates.planned_end_at }, subjectSchedule)
  const supportText = completedAt ? subjectCopy.actualCompletedAt(completedAt) : formatNodePlan(planDates.planned_start_at, planDates.planned_end_at)
  const assignee = nodeAssignee(node)
  const stageMeta = assignee ? subjectCopy.nodeAssigneeMeta(assignee, supportText) : supportText
  const supportTone = completedAt ? 'text-emerald-700 dark:text-emerald-400' : overdue ? 'text-red-600 dark:text-red-400' : planDates.planned_start_at || planDates.planned_end_at ? 'text-muted-foreground' : 'text-amber-700 dark:text-amber-400'
  const hasPlan = Boolean(planDates.planned_start_at || planDates.planned_end_at)
  const showSupport = Boolean(completedAt || overdue || assignee || (hasPlan && supportText !== subjectCopy.nodePlanUnset) || (!vertical && supportText))
  const compact = density === 'branch'
  const showTodoMark = !vertical || Boolean(node.ai_execution || activeNode)
  const nodeChrome = vertical
    ? `pm-workflow-node pm-workflow-node--rail group relative grid h-auto min-h-8 w-full cursor-pointer items-center gap-1.5 whitespace-normal rounded-md border-transparent px-1.5 py-1 text-left shadow-none ${activeNode ? 'pm-workflow-node--active bg-primary/10 hover:bg-primary/10' : node.ai_execution ? 'pm-workflow-node--ai hover:bg-muted/70' : 'pm-workflow-node--standard hover:bg-muted/70'}`
    : `pm-workflow-node group relative grid h-auto w-full cursor-pointer items-center gap-2 whitespace-normal rounded-lg px-2 text-left ${compactChrome ? 'min-h-9 py-0.5' : compact ? 'min-h-11 py-1' : 'min-h-[3.75rem] py-1.5'} ${activeNode ? 'pm-workflow-node--active border-primary/40 bg-primary/10 hover:bg-primary/10' : node.ai_execution ? 'pm-workflow-node--ai border-primary/25 bg-card hover:border-primary/45' : 'pm-workflow-node--standard border-border bg-card hover:border-primary/35 hover:bg-muted/60'}`
  return <div key={node.id} role="listitem" data-testid={`workflow-stage-${node.id}`} data-density={density} data-overdue={overdue || undefined} className={vertical ? 'w-full' : `shrink-0 ${compact ? 'w-[8.5rem]' : 'w-[9.25rem]'}`}>
    <Button
      type="button"
      variant={vertical ? 'ghost' : 'outline'}
      data-testid={`workflow-node-action-${node.id}`}
      aria-current={activeNode ? 'step' : undefined}
      aria-label={node.ai_execution ? subjectCopy.openAIReport(node.name) : activeNode ? subjectCopy.openTodo(node.name) : subjectCopy.openNode(node.name)}
      className={`${nodeChrome} transition-[transform,box-shadow,border-color,background-color] active:translate-y-0 focus-visible:ring-2 focus-visible:ring-inset`}
      onClick={() => openWorkflowNode(node)}
    >
      <span className={`relative grid place-items-center ${vertical ? 'size-6' : 'size-7'}`}>
        <span className={`grid place-items-center rounded-full border-2 ${vertical ? 'size-5' : 'size-6'} ${stageTone(node.status)}`}>{stageIcon(node.status)}</span>
        {showTodoMark ? <span data-testid={`workflow-todo-indicator-${node.id}`} aria-hidden="true" className={`pm-workflow-todo absolute grid size-3.5 place-items-center rounded-full border-2 border-card text-white ${node.ai_execution ? 'bg-primary' : activeNode ? 'bg-red-500' : 'bg-muted-foreground'}`}>{aiBusy ? <AiLoading size={10} label={null} /> : aiFinished || node.ai_execution ? <PhSparkle className="size-2" /> : activeNode ? <PhWarningCircle className="size-2" /> : <PhCalendarCheck className="size-2" />}</span> : <span data-testid={`workflow-todo-indicator-${node.id}`} className="sr-only" />}
      </span>
      <span className="min-w-0">
        <span className={`block truncate font-semibold leading-4 ${vertical ? 'text-xs' : 'text-xs'} ${node.ai_execution && !activeNode ? 'text-primary' : activeNode ? 'text-primary' : 'text-foreground group-hover:text-foreground'}`}>{node.name}</span>
        {showSupport ? <span data-testid={`workflow-stage-assignee-${node.id}`} title={stageMeta} className={`pm-workflow-node__support mt-0.5 flex min-w-0 items-center gap-1 truncate text-[9px] font-medium ${supportTone}`}>{completedAt ? <PhCheckCircle className="size-2.5 shrink-0" /> : overdue ? <span data-testid={`workflow-overdue-${node.id}`} role="img" aria-label={subjectCopy.nodeOverdueLabel(node.name)} title={subjectCopy.nodeOverdue} className="grid shrink-0 place-items-center"><PhClock aria-hidden="true" className="pm-workflow-overdue size-3" /></span> : assignee ? <PhUser className="size-2.5 shrink-0" /> : null}{assignee ? <span className="min-w-0 truncate">{assignee}</span> : null}{assignee && (completedAt || overdue || hasPlan) ? <span aria-hidden="true">{subjectCopy.nodeMetaSeparator}</span> : null}{completedAt || overdue || hasPlan || !vertical ? <span className="shrink-0">{supportText}</span> : null}</span> : <span data-testid={`workflow-stage-assignee-${node.id}`} className="sr-only" />}
      </span>
    </Button>
  </div>
}

export function trackItemKey(item: WorkflowRuntimeTrackItem) {
  if (item.kind === 'node') return item.node.id
  if (item.kind === 'parallel') return `parallel-${item.split.id}`
  if (item.kind === 'fanout') return `fanout-${item.id}`
  return `choice-${item.condition.id}`
}