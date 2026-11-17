import { useCallback } from 'react'
import { PhPath } from '@/shared/ui/icons'
import { useWorkspaceStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import { Button, ResponsivePanel, Skeleton, StatusPill, AlertBanner } from '@/shared/ui'
import { PhArrowsClockwise } from '@/shared/ui/icons'
import { useWorkflowInstances } from '../api/api'
import { WorkflowInstanceControl } from './WorkflowInstanceControl'
import { DismissibleDialogPanel } from './DismissibleDialogPanel'

type SubjectType = 'requirement' | 'plan_unit'
type WorkflowTrackLayout = 'horizontal' | 'vertical'
const subjectCopy = copy.aiPM.workflowSubject

export function WorkflowSubjectLauncher({
  workspaceId: explicitWorkspaceId,
  subjectType,
  subjectId,
  instanceId,
  designImageUrl,
  requirement,
  className = '',
  layout = 'horizontal',
  density = 'default',
  initialNodeId: explicitInitialNodeId,
  initialNodeName: explicitInitialNodeName,
  surface = 'workbench',
  onDismiss,
}: {
  workspaceId?: number
  subjectType: SubjectType
  subjectId: number
  instanceId?: number | null
  designImageUrl?: string
  requirement?: import('../model/types').Requirement
  className?: string
  layout?: WorkflowTrackLayout
  density?: 'default' | 'compact'
  initialNodeId?: string
  initialNodeName?: string
  surface?: 'workbench' | 'dialog'
  onDismiss?: () => void
}) {
  'use no memo'
  const currentWorkspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const workspaceId = explicitWorkspaceId ?? currentWorkspaceId
  const searchNodeId = surface === 'dialog'
    ? ''
    : new URLSearchParams(window.location.search).get('workflow_node')?.trim().slice(0, 120) || ''
  const initialNodeId = explicitInitialNodeId?.trim() || searchNodeId || undefined
  const initialNodeName = explicitInitialNodeName?.trim() || undefined
  const clearWorkflowNodeSearch = useCallback(() => {
    if (!searchNodeId) return
    const url = new URL(window.location.href)
    url.searchParams.delete('workflow_node')
    window.history.replaceState(window.history.state, '', `${url.pathname}${url.search}${url.hash}`)
    window.dispatchEvent(new PopStateEvent('popstate', { state: window.history.state }))
  }, [searchNodeId])
  const dismissDialog = useCallback(() => onDismiss?.(), [onDismiss])
  const instances = useWorkflowInstances(workspaceId, subjectType, subjectId)
  const latest = instanceId || instances.data?.[0]?.id || 0
  const title = subjectType === 'requirement' ? subjectCopy.requirementTitle : subjectCopy.planUnitTitle
  const fixedTitle = subjectType === 'requirement' ? subjectCopy.requirementTemplateFixedTitle : subjectCopy.planUnitTemplateFixedTitle
  const fixedDescription = subjectType === 'requirement' ? subjectCopy.requirementTemplateFixedDescription : subjectCopy.planUnitTemplateFixedDescription
  const dialogPendingTitle = initialNodeName ? subjectCopy.todoTitle(initialNodeName) : title
  const control = latest ? (
    <WorkflowInstanceControl
      key={`${workspaceId}:${latest}`}
      workspaceId={workspaceId}
      instanceId={latest}
      requirementId={subjectType === 'requirement' ? subjectId : 0}
      designImageUrl={designImageUrl}
      requirement={requirement}
      initialNodeId={initialNodeId}
      initialNodeName={initialNodeName}
      chrome={surface}
      layout={layout}
      density={density}
      onDismiss={dismissDialog}
      onInitialNodeClear={surface === 'dialog' ? dismissDialog : clearWorkflowNodeSearch}
    />
  ) : instances.isPending ? (
    surface === 'dialog' ? (
      <DismissibleDialogPanel title={dialogPendingTitle} onDismiss={dismissDialog} className="max-w-6xl">
        <div className="grid gap-3"><Skeleton className="h-4 w-40" /><Skeleton className="h-2 w-full" /><Skeleton className="h-24 w-full" /></div>
      </DismissibleDialogPanel>
    ) : <div className="grid gap-3"><Skeleton className="h-4 w-40" /><Skeleton className="h-14 w-full" /></div>
  ) : instances.isError ? (
    surface === 'dialog' ? (
      <DismissibleDialogPanel title={dialogPendingTitle} onDismiss={dismissDialog}>
        <AlertBanner tone="error" action={<Button size="xs" variant="outline" onClick={() => void instances.refetch()}><PhArrowsClockwise />{copy.ui.retry}</Button>}>{subjectCopy.loadFailed}</AlertBanner>
      </DismissibleDialogPanel>
    ) : <AlertBanner tone="error" action={<Button size="xs" variant="outline" onClick={() => void instances.refetch()}><PhArrowsClockwise />{copy.ui.retry}</Button>}>{subjectCopy.loadFailed}</AlertBanner>
  ) : surface === 'dialog' ? (
    <DismissibleDialogPanel title={fixedTitle} onDismiss={dismissDialog}>
      <p className="m-0 text-sm leading-6 text-muted-foreground">{fixedDescription}</p>
    </DismissibleDialogPanel>
  ) : <div className="rounded-lg border border-dashed border-amber-200 dark:border-amber-800 bg-amber-50/50 dark:bg-amber-950/40 p-3 text-xs leading-5 text-amber-800"><strong className="mb-1 block text-amber-950">{fixedTitle}</strong>{fixedDescription}</div>
  if (surface === 'dialog') return control
  const compactChrome = density === 'compact'
  return <section data-testid={`${subjectType}-workflow-workbench`} data-subject-type={subjectType} data-layout={layout} data-density={density} className={`pm-workflow-shell overflow-hidden ${compactChrome ? '' : 'rounded-lg border bg-card'} ${className}`} aria-label={title}>
    {compactChrome ? null : (
      <div className={`pm-workflow-shell__header flex min-w-0 items-center justify-between gap-2 border-b ${layout === 'vertical' ? 'px-3 py-2' : 'px-4 py-3.5'}`}>
        <div className={`flex min-w-0 ${layout === 'vertical' ? 'items-center gap-2' : 'items-start gap-3'}`}>
          <PhPath aria-hidden="true" className={layout === 'vertical' ? 'size-4 shrink-0 text-muted-foreground' : 'mt-0.5 size-5 shrink-0 text-muted-foreground'} />
          <div className="min-w-0">
            <h2 className="m-0 truncate text-sm font-semibold text-foreground">{title}</h2>
            {layout === 'vertical' ? null : <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{latest ? subjectCopy.executionDescription : subjectCopy.description}</p>}
          </div>
        </div>
        <StatusPill tone={latest ? 'info' : 'neutral'}>{latest ? subjectCopy.bound : subjectCopy.notStarted}</StatusPill>
      </div>
    )}
    <div className={compactChrome ? 'px-4 py-2' : layout === 'vertical' ? 'p-3' : 'p-3.5 sm:p-4'}>
      {compactChrome && !latest ? (
        <div className="mb-2 flex min-w-0 items-center gap-2">
          <h2 className="m-0 truncate text-xs font-semibold text-foreground">{title}</h2>
          <StatusPill tone="neutral">{subjectCopy.notStarted}</StatusPill>
        </div>
      ) : null}
      {control}
    </div>
  </section>
}