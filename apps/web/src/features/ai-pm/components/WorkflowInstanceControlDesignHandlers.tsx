import { useCallback } from 'react'
import { PhCheck, PhCircleNotch, PhArrowUUpLeft } from '@/shared/ui/icons'
import { copy } from '@/shared/copy'
import { Button } from '@/shared/ui'
import { appendUniqueDesignImage, attachableDesignURL } from '../utils/workflow-node-materials'
import type { WorkflowInstance, WorkflowNodeAction } from '../model/types'

const subjectCopy = copy.aiPM.workflowSubject

interface DesignHandlersParams {
  instance: WorkflowInstance
  actionNode: WorkflowInstance['timeline_nodes'][number] | undefined
  actionIsActive: boolean
  actionAllows: (action: WorkflowNodeAction) => boolean
  canAttachDesignToNode: boolean
  pendingAttachDesignURLs: string[]
  persistedActionDesignImages: Array<{ url: string; name: string }>
  designImages: Record<string, Array<{ url: string; name: string }>>
  designLinks: Record<string, string>
  rollbackTargets: WorkflowInstance['rollback_targets']
  requirementId: number
  libraryDesignURLs: string[]
  mutations: ReturnType<typeof import('../api/api').useWorkflowMutations>
  materialMutations: ReturnType<typeof import('../api/api').useRequirementMaterialMutations>
  setDesignImages: React.Dispatch<React.SetStateAction<Record<string, Array<{ url: string; name: string }>>>>
  setDesignLinks: React.Dispatch<React.SetStateAction<Record<string, string>>>
  beginRollback: (sourceNodeId?: string) => void
}

export function useWorkflowInstanceControlDesignHandlers(params: DesignHandlersParams) {
  const {
    instance, actionNode, actionIsActive, actionAllows,
    canAttachDesignToNode, pendingAttachDesignURLs,
    persistedActionDesignImages, designImages,
    designLinks, rollbackTargets,
    requirementId, libraryDesignURLs,
    mutations, materialMutations,
    setDesignImages, setDesignLinks,
    beginRollback,
  } = params

  const saveDesignMaterial = useCallback(async (nodeId: string, designURL: string) => {
    if (!actionAllows('supplement_design')) return
    const attachURL = attachableDesignURL(designURL)
    if (!attachURL) return
    await mutations.action.mutateAsync({
      id: instance.id, action: 'supplement_design', node_id: nodeId, design_url: attachURL,
    })
  }, [actionAllows, instance.id, mutations.action])

  const saveDesignLinks = useCallback(async (nodeId: string, links: string[]) => {
    for (const link of links) {
      if (requirementId > 0 && !libraryDesignURLs.includes(link)) {
        await materialMutations.upload.mutateAsync({
          requirementId, kind: 'design', url: link, source: 'workflow',
        })
      }
      await saveDesignMaterial(nodeId, link)
    }
    setDesignLinks((current) => ({ ...current, [nodeId]: '' }))
  }, [libraryDesignURLs, materialMutations.upload, requirementId, saveDesignMaterial,
    setDesignLinks])

  const attachPendingDesignToNode = useCallback(async () => {
    if (!actionNode || !canAttachDesignToNode) return
    for (const url of pendingAttachDesignURLs) {
      await saveDesignMaterial(actionNode.id, url)
    }
  }, [actionNode, canAttachDesignToNode, pendingAttachDesignURLs, saveDesignMaterial])

  const attachDesignToNodeButton = canAttachDesignToNode ? (
    <Button
      data-testid="workflow-attach-design-to-node"
      variant={actionIsActive ? 'outline' : undefined}
      size={actionIsActive ? 'sm' : undefined}
      className={actionIsActive ? 'bg-card' : undefined}
      disabled={mutations.action.isPending}
      onClick={() => { void attachPendingDesignToNode() }}
    >
      {mutations.action.isPending ? <PhCircleNotch className="animate-spin" /> : <PhCheck />}
      {subjectCopy.attachToNode}
    </Button>
  ) : null

  const rollbackActionButton = actionNode && actionIsActive && (rollbackTargets?.length ?? 0) > 0
    && actionAllows('rollback') ? (
    <Button
      variant="warning-soft"
      disabled={mutations.action.isPending}
      onClick={() => beginRollback(actionNode.id)}
    >
      <PhArrowUUpLeft />{subjectCopy.rollback}
    </Button>
  ) : null

  return {
    saveDesignMaterial, saveDesignLinks,
    attachPendingDesignToNode,
    attachDesignToNodeButton, rollbackActionButton,
  }
}