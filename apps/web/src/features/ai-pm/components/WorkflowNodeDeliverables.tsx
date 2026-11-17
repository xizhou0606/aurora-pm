import { useState } from 'react'
import { PhArrowSquareOut, PhFileXls, PhFileText, PhImage, PhCircleNotch, PhPackage } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import { Button, EmptyState, IconBadge, Skeleton, buttonVariants, notifyOperationError, operationFeedback } from '@/shared/ui'
import { useRequirementMaterials, useWorkflowMaterialProjections, useWorkflowTestCaseArtifacts } from '../api/api'
import type { WorkflowInstance, WorkflowNodeArtifact } from '../model/types'
import { isRequirementInputWorkflowNode } from './WorkflowRequirementEntry'
import { isTestCaseWorkflowNode } from './WorkflowTestCaseWorkbench'
import { useWorkflowOperationAccess } from '../hooks/useOperationAccess'
import { RequirementMaterialsPanel } from './RequirementMaterialsPanel'
import { previewWorkflowNodeArtifact } from '../utils/workflow-artifact-file'
import {
  designMaterialAttachURL,
  designMaterialDisplayName,
  isDesignDeliverableNode,
  mergeDesignMaterialURLs,
  resolveNodeDesignMaterialURLs,
  selectEffectiveTestCaseArtifacts,
} from '../utils/workflow-node-materials'

export {
  isDesignDeliverableNode,
  resolveNodeDesignMaterialURL,
  resolveNodeDesignMaterialURLs,
  selectEffectiveTestCaseArtifact,
  selectEffectiveTestCaseArtifacts,
} from '../utils/workflow-node-materials'

const detailCopy = copy.aiPM.requirementDetail

type TimelineNode = WorkflowInstance['timeline_nodes'][number]

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${Math.ceil(value / 1024)} KB`
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}

function DesignDeliverable({ urls }: { urls: string[] }) {
  return <div className="grid gap-2 rounded-lg border border-blue-100 dark:border-blue-900 bg-blue-50/50 dark:bg-blue-950/40 p-3">
    <div className="flex items-start gap-2.5">
      <IconBadge icon={PhImage} tone="blue" />
      <div className="min-w-0"><strong className="block text-xs text-foreground">{detailCopy.deliverablesDesign}</strong><span className="mt-0.5 block text-xs leading-4 text-muted-foreground">{urls.length ? detailCopy.deliverablesDesignReady : detailCopy.deliverablesDesignEmpty}</span></div>
    </div>
    {urls.map((url) => {
      const name = designMaterialDisplayName(url)
      return <a
        key={url}
        href={url}
        target="_blank"
        rel="noopener noreferrer"
        aria-label={urls.length > 1 ? detailCopy.deliverablesOpenDesignNamed(name) : detailCopy.deliverablesOpenDesign}
        className={buttonVariants({ variant: 'outline', size: 'sm', className: 'w-full border-blue-200 dark:border-blue-800 bg-card text-blue-700 dark:text-blue-300 hover:bg-blue-50 dark:hover:bg-blue-950/40' })}
      ><PhArrowSquareOut /><span className="min-w-0 truncate">{urls.length > 1 ? name : detailCopy.deliverablesOpenDesign}</span></a>
    })}
  </div>
}

function RequirementDocumentDeliverable({ url }: { url?: string }) {
  return <div className="grid gap-2 rounded-lg border border-cyan-100 bg-cyan-50/50 dark:bg-cyan-950/40 p-3">
    <div className="flex items-start gap-2.5">
      <IconBadge icon={PhFileText} tone="cyan" />
      <div className="min-w-0"><strong className="block text-xs text-foreground">{detailCopy.deliverablesRequirementDocument}</strong><span className="mt-0.5 block text-xs leading-4 text-muted-foreground">{url ? detailCopy.deliverablesRequirementDocumentReady : detailCopy.deliverablesRequirementDocumentEmpty}</span></div>
    </div>
    {url ? <a href={url} target="_blank" rel="noopener noreferrer" className={buttonVariants({ variant: 'outline', size: 'sm', className: 'w-full border-cyan-200 dark:border-cyan-800 bg-card text-cyan-700 dark:text-cyan-300 hover:bg-cyan-50 dark:hover:bg-cyan-950/40' })}><PhArrowSquareOut />{detailCopy.deliverablesOpenRequirementDocument}</a> : null}
  </div>
}

function TestCaseDeliverables({ workspaceId, instanceId, node }: { workspaceId: number; instanceId: number; node: TimelineNode }) {
  const access = useWorkflowOperationAccess()
  const artifacts = useWorkflowTestCaseArtifacts(workspaceId, instanceId, node.id)
  const effectiveArtifacts = selectEffectiveTestCaseArtifacts(artifacts.data ?? [])
  const [openingArtifactId, setOpeningArtifactId] = useState(0)
  const openArtifact = async (artifact: WorkflowNodeArtifact) => {
    if (!access.canDownloadTestCaseMaterial()) return
    setOpeningArtifactId(artifact.id)
    try {
      await previewWorkflowNodeArtifact(artifact)
    } catch (error) {
      notifyOperationError(operationFeedback('', detailCopy.deliverablesTestCaseOpenFailed), error)
    } finally {
      setOpeningArtifactId(0)
    }
  }
  const sourceKind = effectiveArtifacts[0]?.kind
  if (!access.canViewTestCaseMaterials()) return null
  return <div className="grid gap-2 rounded-lg border border-border bg-muted/30 p-3">
    <div className="flex items-start gap-2.5">
      <IconBadge icon={PhFileXls} tone="violet" />
      <div className="min-w-0"><strong className="block text-xs text-foreground">{detailCopy.deliverablesTestCases}</strong><span className="mt-0.5 block text-xs leading-4 text-muted-foreground">{sourceKind ? detailCopy.deliverablesTestCaseSource(sourceKind) : detailCopy.deliverablesTestCaseEmptyStatus}</span></div>
    </div>
    {artifacts.isPending ? <Skeleton className="h-9 w-full" /> : artifacts.isError ? <p role="alert" className="m-0 text-xs text-destructive">{detailCopy.deliverablesTestCaseLoadFailed}</p> : effectiveArtifacts.length && access.canDownloadTestCaseMaterial() ? effectiveArtifacts.map((artifact) => (
      <Button
        key={artifact.id}
        type="button"
        variant="ghost"
        size="sm"
        disabled={openingArtifactId === artifact.id}
        aria-label={detailCopy.deliverablesOpenTestCase(artifact.name)}
        className="h-auto w-full min-w-0 justify-between gap-2 rounded-md border border-border bg-card px-2.5 py-2 text-xs text-foreground hover:bg-muted/60 disabled:cursor-wait disabled:opacity-70"
        onClick={() => void openArtifact(artifact)}
      >
        <span className="min-w-0 truncate font-medium">{artifact.name}</span>
        <span className="flex shrink-0 items-center gap-1 text-xs text-muted-foreground">{openingArtifactId === artifact.id ? <PhCircleNotch aria-hidden="true" className="size-3 animate-spin" /> : null}{formatBytes(artifact.size)}</span>
      </Button>
    )) : <p className="m-0 text-xs leading-5 text-muted-foreground">{detailCopy.deliverablesTestCaseEmpty}</p>}
  </div>
}

function DeliverableNode({ workspaceId, instance, node, designURLs, requirementDocURL }: { workspaceId: number; instance: WorkflowInstance; node: TimelineNode; designURLs: string[]; requirementDocURL?: string }) {
  const collectsRequirement = isRequirementInputWorkflowNode(node)
  const collectsDesign = isDesignDeliverableNode(instance, node)
  const collectsTestCases = isTestCaseWorkflowNode(node)
  return <section className="overflow-hidden rounded-lg border border-border bg-card" aria-labelledby={`workflow-deliverables-${node.id}`}>
    <header className="border-b border-border bg-muted/30 px-3 py-2.5"><h3 id={`workflow-deliverables-${node.id}`} className="m-0 truncate text-sm font-semibold text-foreground">{node.name}</h3></header>
    <div className="grid gap-2.5 p-3">
      {collectsRequirement ? <RequirementDocumentDeliverable url={requirementDocURL} /> : null}
      {collectsDesign ? <DesignDeliverable urls={designURLs} /> : null}
      {collectsTestCases ? <TestCaseDeliverables workspaceId={workspaceId} instanceId={instance.id} node={node} /> : null}
    </div>
  </section>
}

export function WorkflowNodeDeliverables({
  workspaceId,
  requirementId,
  instance,
  loading,
  error,
  onRetry,
  designURL,
  requirementDocURL,
}: {
  workspaceId: number
  requirementId?: number
  instance?: WorkflowInstance
  loading?: boolean
  error?: boolean
  onRetry?: () => void
  designURL?: string
  requirementDocURL?: string
}) {
  'use no memo'
  const designMaterials = useRequirementMaterials(workspaceId, instance ? 0 : requirementId ?? 0, 'design')
  const projectedMaterials = useWorkflowMaterialProjections(workspaceId, instance?.id ?? 0, 'design')
  const libraryURLs = ((instance ? projectedMaterials.data : designMaterials.data) ?? []).map((row) => designMaterialAttachURL(row)).filter(Boolean)
  const nodes = (instance?.timeline_nodes ?? []).filter((node) => (
    node.type !== 'start' && node.type !== 'end' && node.status !== 'skipped' && instance && (
      isRequirementInputWorkflowNode(node) || isDesignDeliverableNode(instance, node) || isTestCaseWorkflowNode(node)
    )
  ))
  return <section className="grid gap-3 xl:sticky xl:top-4" aria-labelledby="workflow-deliverables-title">
    <header className="rounded-lg border border-border bg-card p-4">
      <div className="flex items-start gap-3"><IconBadge icon={PhPackage} tone="blue" size="lg" /><div><h2 id="workflow-deliverables-title" className="m-0 text-sm font-semibold text-foreground">{detailCopy.deliverablesTitle}</h2><p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{detailCopy.deliverablesDescription}</p></div></div>
    </header>
    {loading ? <div className="grid gap-3"><Skeleton className="h-32 w-full" /><Skeleton className="h-32 w-full" /></div> : error ? <EmptyState role="alert" title={detailCopy.deliverablesLoadFailed} action={onRetry ? <Button variant="outline" onClick={onRetry}>{copy.ui.retry}</Button> : undefined} /> : !instance && requirementId ? <RequirementMaterialsPanel workspaceId={workspaceId} requirementId={requirementId} /> : !instance ? <EmptyState title={detailCopy.deliverablesNoWorkflowTitle} /> : nodes.length ? <div className="grid gap-3">{nodes.map((node) => (
      <DeliverableNode key={node.id} workspaceId={workspaceId} instance={instance} node={node} designURLs={mergeDesignMaterialURLs(resolveNodeDesignMaterialURLs(instance, node.id, designURL), libraryURLs)} requirementDocURL={requirementDocURL} />
    ))}</div> : <EmptyState title={detailCopy.deliverablesEmptyTitle} />}
  </section>
}
