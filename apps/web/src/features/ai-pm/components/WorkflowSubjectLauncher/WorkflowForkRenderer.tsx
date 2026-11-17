import { PhCaretRight, PhGitBranch } from '@/shared/ui/icons'
import { copy } from '@/shared/copy'
import { renderStage, trackItemKey } from './WorkflowTrackRenderer'
import { structuralNodeTypes, type TimelineNode } from './utils'
import type { WorkflowInstance } from '../../model/types'
import type { WorkflowRuntimeLane, WorkflowRuntimeTrackItem } from '../../utils/workflow-runtime-track'

const subjectCopy = copy.aiPM.workflowSubject

function renderForkLane({
  lane,
  laneIndex,
  groupID,
  choice = false,
  vertical,
  activeNodeById,
  instance,
  compactChrome,
  openWorkflowNode,
}: {
  lane: WorkflowRuntimeLane
  laneIndex: number
  groupID: string
  choice?: boolean
  vertical: boolean
  activeNodeById: Map<string, WorkflowInstance['current_nodes'][number]>
  instance: WorkflowInstance
  compactChrome: boolean
  openWorkflowNode: (node: TimelineNode) => void
}) {
  const label = lane.label || subjectCopy.parallelBranch(laneIndex + 1)
  const branchTitle = choice ? label : subjectCopy.parallelBranch(laneIndex + 1)
  return <div
    key={`${groupID}-${lane.id}`}
    data-testid={`workflow-fork-lane-${groupID}-${laneIndex}`}
    role="group"
    aria-label={label}
    className={vertical
      ? 'grid min-w-0 gap-1 px-1.5 py-1'
      : 'grid min-h-11 grid-cols-[max-content_max-content] items-center gap-1.5'}
  >
    {vertical
      ? <span className={`block truncate text-[9px] font-semibold tracking-wide ${choice ? 'text-sky-700 dark:text-sky-300' : 'text-primary'}`}>{branchTitle}</span>
      : <span aria-hidden="true" className={`grid min-h-5 min-w-5 place-items-center whitespace-nowrap rounded-full px-1.5 text-xs font-bold ${choice ? 'bg-sky-100 text-sky-700' : 'bg-primary/10 text-primary'}`}>{choice ? label : laneIndex + 1}</span>}
    {lane.items.length ? <div role="list" className={vertical ? 'grid w-full gap-0.5' : 'flex items-center gap-1.5'}>{renderTrackItems({ items: lane.items, density: 'branch', vertical, activeNodeById, instance, compactChrome, openWorkflowNode })}</div> : <span className="pr-2 text-xs text-muted-foreground">{lane.directTarget && !structuralNodeTypes.has(lane.directTarget.type) ? subjectCopy.directToStage(lane.directTarget.name) : subjectCopy.choiceDirect}</span>}
  </div>
}

function renderForkLanes({
  lanes,
  groupID,
  choice = false,
  vertical,
  activeNodeById,
  instance,
  compactChrome,
  openWorkflowNode,
}: {
  lanes: WorkflowRuntimeLane[]
  groupID: string
  choice?: boolean
  vertical: boolean
  activeNodeById: Map<string, WorkflowInstance['current_nodes'][number]>
  instance: WorkflowInstance
  compactChrome: boolean
  openWorkflowNode: (node: TimelineNode) => void
}) {
  if (!vertical) return <div className="flex flex-col gap-1.5">{lanes.map((lane, laneIndex) => renderForkLane({ lane, laneIndex, groupID, choice, vertical, activeNodeById, instance, compactChrome, openWorkflowNode }))}</div>
  return <div
    data-testid={`workflow-fork-lanes-${groupID}`}
    className={lanes.length > 1 ? 'pm-workflow-fork__lanes grid grid-cols-2' : 'grid gap-1'}
  >
    {lanes.map((lane, laneIndex) => renderForkLane({ lane, laneIndex, groupID, choice, vertical, activeNodeById, instance, compactChrome, openWorkflowNode }))}
  </div>
}

export function renderTrackItem({
  item,
  density,
  vertical,
  activeNodeById,
  instance,
  compactChrome,
  openWorkflowNode,
}: {
  item: WorkflowRuntimeTrackItem
  density: 'track' | 'branch'
  vertical: boolean
  activeNodeById: Map<string, WorkflowInstance['current_nodes'][number]>
  instance: WorkflowInstance
  compactChrome: boolean
  openWorkflowNode: (node: TimelineNode) => void
}) {
  if (item.kind === 'node') return renderStage({ node: item.node, instance, activeNodeById, density, compactChrome, vertical, openWorkflowNode })
  if (item.kind === 'parallel') return <section
    data-testid={`workflow-parallel-${item.split.id}`}
    data-layout={vertical ? 'vertical-parallel-columns' : 'compact-parallel-stack'}
    role="listitem"
    aria-label={subjectCopy.parallelGroupLabel(item.lanes.length)}
    className={vertical
      ? 'pm-workflow-fork w-full overflow-hidden rounded-md border border-dashed border-primary/40'
      : 'shrink-0 border-l-2 border-primary/25 py-1.5 pr-2 pl-3'}
  >
    <header className={`flex items-center gap-1.5 text-xs font-semibold text-primary ${vertical ? 'justify-center border-b border-dashed border-primary/40 px-2 py-1' : 'mb-1 px-0.5'}`}>
      <PhGitBranch aria-hidden="true" className="size-3.5 shrink-0" />
      <span>{subjectCopy.parallelBranchCount(item.lanes.length)}</span>
      <span className="text-xs font-medium text-muted-foreground">{item.split.status !== 'completed' ? subjectCopy.parallelPending : item.join.status === 'completed' ? subjectCopy.parallelCompleted : item.join.parallel_total ? subjectCopy.parallelProgress(item.join.parallel_completed ?? 0, item.join.parallel_total) : subjectCopy.parallelWaiting}</span>
    </header>
    {renderForkLanes({ lanes: item.lanes, groupID: item.split.id, vertical, activeNodeById, instance, compactChrome, openWorkflowNode })}
  </section>
  if (item.kind === 'fanout') return <section
    data-testid={`workflow-concurrent-${item.id}`}
    data-layout="compact-concurrent-stack"
    role="listitem"
    aria-label={subjectCopy.concurrentGroupLabel(item.lanes.length)}
    className={vertical
      ? 'pm-workflow-fork w-full overflow-hidden rounded-md border border-dashed border-border'
      : 'shrink-0 border-l-2 border-border/70 py-1.5 pr-2 pl-3'}
  >
    <header className={`flex items-center gap-1.5 text-xs font-semibold text-muted-foreground ${vertical ? 'justify-center border-b border-dashed border-border px-2 py-1' : 'mb-1 px-0.5'}`}><PhGitBranch aria-hidden="true" className="size-3.5 shrink-0" /><span>{subjectCopy.concurrentBranchCount(item.lanes.length)}</span></header>
    {renderForkLanes({ lanes: item.lanes, groupID: item.id, vertical, activeNodeById, instance, compactChrome, openWorkflowNode })}
  </section>
  return <section
    data-testid={`workflow-choice-${item.condition.id}`}
    data-layout="compact-choice-stack"
    role="listitem"
    aria-label={subjectCopy.choiceGroupLabel(item.lanes.length)}
    className={vertical
      ? 'pm-workflow-fork w-full overflow-hidden rounded-md border border-dashed border-sky-400/40'
      : 'shrink-0 border-l-2 border-sky-300/50 py-1.5 pr-2 pl-3'}
  >
    <header className={`flex items-center gap-1.5 text-xs font-semibold text-sky-700 dark:text-sky-300 ${vertical ? 'justify-center border-b border-dashed border-sky-400/40 px-2 py-1' : 'mb-1 px-0.5'}`}><PhGitBranch aria-hidden="true" className="size-3.5 shrink-0" /><span>{subjectCopy.choiceBranchCount(item.lanes.length)}</span></header>
    {renderForkLanes({ lanes: item.lanes, groupID: item.condition.id, choice: true, vertical, activeNodeById, instance, compactChrome, openWorkflowNode })}
  </section>
}

export function renderTrackItems({
  items,
  density,
  vertical,
  activeNodeById,
  instance,
  compactChrome,
  openWorkflowNode,
}: {
  items: WorkflowRuntimeTrackItem[]
  density: 'track' | 'branch'
  vertical: boolean
  activeNodeById: Map<string, WorkflowInstance['current_nodes'][number]>
  instance: WorkflowInstance
  compactChrome: boolean
  openWorkflowNode: (node: TimelineNode) => void
}) {
  return items.map((item, index) => <div
    key={trackItemKey(item)}
    className={vertical ? 'flex w-full flex-col items-stretch' : 'flex shrink-0 items-center gap-1.5'}
  >
    {renderTrackItem({ item, density, vertical, activeNodeById, instance, compactChrome, openWorkflowNode })}
    {index < items.length - 1 ? (
      vertical ? (
        <span aria-hidden="true" className="ml-3 h-3 w-px bg-border" />
      ) : (
        <span aria-hidden="true" className="flex shrink-0 items-center text-muted-foreground/70">
          <span className="h-px w-2 bg-current" />
          <PhCaretRight className="size-3.5" />
        </span>
      )
    ) : null}
  </div>)
}