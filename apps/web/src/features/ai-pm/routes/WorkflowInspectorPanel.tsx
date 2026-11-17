import type { Edge, Node } from '@xyflow/react'
import { copy } from '@/shared/copy'
import type { AICapability } from '../model/types'
import type { RequirementTemplateVersionOption } from '../api/workflow-queries'
import { workflowEdgeDisplayLabel, workflowEdgeRouting } from '../utils/workflow-edge-label'
import { EdgeInspector } from './WorkflowEdgeInspector'
import { NodeInspector } from './WorkflowNodeInspector'

const workflowCopy = copy.aiPM.workflow

export function InspectorPanel({
  subjectType,
  capabilityDirectory,
  requirementTemplates,
  selected,
  selectedEdge,
  setNodes,
  setEdges,
  clearNode,
  clearEdge,
}: {
  subjectType: 'requirement' | 'plan_unit'
  capabilityDirectory: { status: 'loading' | 'error' | 'ready'; items: AICapability[]; retry: () => void }
  requirementTemplates?: {
    data?: RequirementTemplateVersionOption[]
    isPending: boolean
    isError: boolean
    refetch: () => void
  }
  selected?: Node
  selectedEdge?: Edge
  setNodes: (updater: (current: Node[]) => Node[]) => void
  setEdges: (updater: (current: Edge[]) => Edge[]) => void
  clearNode: () => void
  clearEdge: () => void
}) {
  return <>
    <h2 className="m-0 text-xs font-semibold text-muted-foreground">{selectedEdge ? workflowCopy.edgeInspector : workflowCopy.inspector}</h2>
    {selected ? <NodeInspector subjectType={subjectType} capabilityDirectory={capabilityDirectory} requirementTemplates={requirementTemplates} node={selected} update={(data) => setNodes((current) => current.map((node) => node.id === selected.id ? { ...node, data: { ...node.data, ...data } } : node))} remove={() => { setNodes((current) => current.filter((node) => node.id !== selected.id)); setEdges((current) => current.filter((edge) => edge.source !== selected.id && edge.target !== selected.id)); clearNode() }} /> : selectedEdge ? <EdgeInspector edge={selectedEdge} update={(data) => setEdges((current) => current.map((edge) => edge.id === selectedEdge.id ? { ...edge, data: { ...edge.data, ...data }, label: workflowEdgeDisplayLabel(data), animated: data.kind === 'failure', ...workflowEdgeRouting(data) } : edge))} remove={() => { setEdges((current) => current.filter((edge) => edge.id !== selectedEdge.id)); clearEdge() }} /> : <p className="mt-3 text-xs text-muted-foreground">{workflowCopy.selectNodeOrEdge}</p>}
  </>
}