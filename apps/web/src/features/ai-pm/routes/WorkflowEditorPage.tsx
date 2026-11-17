import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from '@tanstack/react-router'
import dagre from '@dagrejs/dagre'
import {
  addEdge, Background, Controls, Handle, MiniMap, Position, ReactFlow,
  useEdgesState, useNodesState, type Connection, type Edge, type Node, type NodeProps, type ReactFlowInstance,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { PhArrowLeft, PhRobot, PhCheckCircle, PhGitBranch, PhPlus, PhFloppyDisk, PhPaperPlaneTilt, PhShieldCheck, PhChartBar } from '@/shared/ui/icons'
import { useWorkspaceStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import { AlertBanner, Button, EmptyState, LoadingSkeleton, OperationalPage, StatusPill } from '@/shared/ui'
import { useAICapabilities, useCancelAIRun, useRequirementTemplateVersions, useWorkflow, useWorkflowMutations } from '../api/api'
import { useInjectedAIPMEnvironment } from '../components/environment'
import { WorkflowAIAssistant, type WorkflowAIRunState } from '../components/WorkflowAIAssistant'
import { CanvasAILoading } from './WorkflowCanvasAILoading'
import { InspectorPanel } from './WorkflowInspectorPanel'
import { WorkflowDeliveryProgressEditor } from './WorkflowDeliveryProgressEditor'
import type { AICapability, WorkflowDefinition, WorkflowDeliveryProgressConfig, WorkflowNodeConfig } from '../model/types'
import { aiNodeTypeForCapability, isWorkflowCapability, nodeTypeLabel } from '../utils/workflow-node-utils'
import { workflowEdgeDisplayLabel, workflowEdgeRouting } from '../utils/workflow-edge-label'
import { useWorkflowOperationAccess } from '../hooks/useOperationAccess'

const workflowCopy = copy.aiPM.workflow
const commonCopy = copy.aiPM.common

const nodeCatalog = [
  { type: 'start', label: workflowCopy.startNode, singleton: true },
  { type: 'human_task', label: workflowCopy.humanTask }, { type: 'approval', label: workflowCopy.approval },
  { type: 'condition', label: workflowCopy.condition }, { type: 'parallel_split', label: workflowCopy.parallelSplit }, { type: 'parallel_join', label: workflowCopy.parallelJoin },
  { type: 'requirement_gate', label: workflowCopy.requirementGate },
  { type: 'ai', label: workflowCopy.aiNode },
  { type: 'end', label: workflowCopy.endNode, singleton: true },
]

const aiDefaultCapability = {
  requirement: 'requirement.completeness',
  plan_unit: 'delivery.risk_monitor',
} satisfies Record<'requirement' | 'plan_unit', string>

function withoutTemplateAssignees(definition: WorkflowDefinition): WorkflowDefinition {
  const identityKeys = ['assignee_user_id', 'assignee_name', 'project_id', 'project_set_id', 'requirement_id', 'subject_id']
  return {
    ...definition,
    nodes: definition.nodes.map((node) => {
      const config: WorkflowNodeConfig = { ...(node.config ?? {}) }
      for (const key of identityKeys) delete config[key]
      return { ...node, ...(Object.keys(config).length ? { config } : { config: undefined }) }
    }),
  }
}

function layout(definition: WorkflowDefinition): { nodes: Node[]; edges: Edge[] } {
  const graph = new dagre.graphlib.Graph().setDefaultEdgeLabel(() => ({}))
  graph.setGraph({ rankdir: 'LR', nodesep: 46, ranksep: 90, marginx: 30, marginy: 30 })
  definition.nodes.forEach((node) => graph.setNode(node.id, { width: 190, height: 80 }))
  definition.edges.forEach((edge) => {
    if (edge.kind !== 'failure' && edge.kind !== 'reject') graph.setEdge(edge.source, edge.target)
  })
  dagre.layout(graph)
  return {
    nodes: definition.nodes.map((node) => {
      const point = graph.node(node.id)
      return { id: node.id, type: 'workflow', position: { x: point.x - 95, y: point.y - 40 }, data: { ...node } }
    }),
    edges: definition.edges.map((edge) => ({
      id: edge.id, source: edge.source, target: edge.target,
      data: { kind: edge.kind ?? '', condition: edge.condition ?? '', loop: edge.loop ?? false },
      label: workflowEdgeDisplayLabel(edge), animated: edge.kind === 'failure',
      ...workflowEdgeRouting(edge),
    })),
  }
}

function WorkflowNode({ data, selected }: NodeProps) {
  const node = data as { name?: string; type?: string; capability?: string; config?: WorkflowNodeConfig; aiRecentlyApplied?: boolean }
  const ai = String(node.type).startsWith('ai_')
  return <div className={`w-[190px] rounded-lg border bg-card px-3 py-2 transition ${node.aiRecentlyApplied ? 'border-primary ring-2 ring-primary/30' : selected ? 'border-primary ring-2 ring-primary/20' : ai ? 'border-primary/35' : 'border-border'}`}>
    <Handle id="forward-target" type="target" position={Position.Left} />
    <Handle id="recovery-target" type="target" position={Position.Top} isConnectable={false} className="!opacity-0" />
    <div className="flex items-center gap-2">{ai ? <PhRobot className="size-4 text-primary" /> : node.type === 'approval' ? <PhShieldCheck className="size-4 text-amber-600" /> : <PhGitBranch className="size-4 text-muted-foreground" />}<strong className="truncate text-xs">{node.name}</strong></div>
    <div className="mt-1 truncate text-xs text-muted-foreground">{nodeTypeLabel(String(node.type))}</div>
    {node.config?.critical_gate ? <span className="mt-1 inline-block text-xs font-medium text-amber-700 dark:text-amber-400">{workflowCopy.criticalGate}</span> : null}
    <Handle id="forward-source" type="source" position={Position.Right} />
    <Handle id="recovery-source" type="source" position={Position.Bottom} isConnectable={false} className="!opacity-0" />
  </div>
}

const workflowNodeTypes = { workflow: WorkflowNode }

type AICapabilityDirectory = {
  status: 'loading' | 'error' | 'ready'
  items: AICapability[]
  retry: () => void
}

export function WorkflowEditorPage() {
  const injected = useInjectedAIPMEnvironment()
  if (injected) {
    return (
      <WorkflowEditorSession
        onBack={() => void injected.navigate('/workspace/pm/workflows')}
        templateId={injected.templateId ?? 0}
        workspaceId={injected.workspaceId}
      />
    )
  }
  return <RoutedWorkflowEditorPage />
}

function RoutedWorkflowEditorPage() {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const params = useParams({ from: '/workspace/pm/workflows/$templateId' })
  const navigate = useNavigate()
  return (
    <WorkflowEditorSession
      onBack={() => void navigate({ to: '/workspace/pm/workflows' } as never)}
      templateId={Number(params.templateId)}
      workspaceId={workspaceId}
    />
  )
}

function WorkflowEditorSession({
  onBack,
  templateId,
  workspaceId,
}: {
  onBack: () => void
  templateId: number
  workspaceId: number
}) {
  const access = useWorkflowOperationAccess()
  const query = useWorkflow(workspaceId, templateId)
  if (access.status === 'ready' && !access.canViewTemplates()) return null
  if (query.isPending) return <OperationalPage title={workflowCopy.title}><div className="p-5"><LoadingSkeleton rows={10} /></div></OperationalPage>
  if (query.isError || !query.data?.definition) return <OperationalPage title={workflowCopy.title}><div className="p-5"><EmptyState role="alert" title={workflowCopy.loadFailed} /></div></OperationalPage>
  return <WorkflowEditorReady key={`${query.data.id}-${query.data.updated_at}`} onBack={onBack} template={query.data} workspaceId={workspaceId} />
}

function WorkflowEditorReady({
  onBack,
  template,
  workspaceId,
}: {
  onBack: () => void
  template: NonNullable<ReturnType<typeof useWorkflow>['data']>
  workspaceId: number
}) {
  const mutations = useWorkflowMutations(workspaceId)
  const access = useWorkflowOperationAccess()
  const cancelAIRun = useCancelAIRun(workspaceId)
  const capabilities = useAICapabilities(workspaceId)
  const requirementTemplates = useRequirementTemplateVersions(workspaceId)
  const capabilityDirectory: AICapabilityDirectory = capabilities.isPending
    ? { status: 'loading', items: [], retry: () => void capabilities.refetch() }
    : capabilities.isError
      ? { status: 'error', items: [], retry: () => void capabilities.refetch() }
      : { status: 'ready', items: capabilities.data?.capabilities ?? [], retry: () => void capabilities.refetch() }
  const templateDefinition = useMemo(() => withoutTemplateAssignees(template.definition!), [template.definition])
  const initial = useMemo(() => layout(templateDefinition), [templateDefinition])
  const [nodes, setNodes, onNodesChange] = useNodesState(initial.nodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(initial.edges)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [selectedEdgeId, setSelectedEdgeId] = useState<string | null>(null)
  const [rightTab, setRightTab] = useState<'ai' | 'inspector' | 'delivery'>('ai')
  const [deliveryConfig, setDeliveryConfig] = useState<WorkflowDeliveryProgressConfig | null>(
    template.subject_type === 'plan_unit' ? template.definition?.delivery_progress ?? null : null,
  )
  const [aiRunState, setAIRunState] = useState<WorkflowAIRunState>({ busy: false, phase: 'idle' })
  const [applyNotice, setApplyNotice] = useState(false)
  const flowInstance = useRef<ReactFlowInstance<Node, Edge> | null>(null)
  const highlightTimer = useRef<number | null>(null)
  const canvasSection = useRef<HTMLElement | null>(null)
  const previousAIRunBusy = useRef(false)
  const selected = nodes.find((node) => node.id === selectedId)
  const selectedEdge = edges.find((edge) => edge.id === selectedEdgeId)
  const selectedIsAI = String(selected?.data.type ?? '').startsWith('ai_')
  const workflowCapabilities = capabilityDirectory.items
  const visibleNodeCatalog = nodeCatalog.filter((entry) => (
    entry.type !== 'ai' || workflowCapabilities.some((capability) => isWorkflowCapability(capability, template.subject_type))
  ))
  const definition = useCallback((): WorkflowDefinition => ({
    nodes: nodes.map((node) => ({
      id: node.id, type: String(node.data.type), name: String(node.data.name),
      ...(node.data.capability ? { capability: String(node.data.capability) } : {}),
      ...(node.data.config ? { config: node.data.config as Record<string, unknown> } : {}),
    })),
    edges: edges.map((edge) => ({
      id: edge.id, source: edge.source, target: edge.target,
      ...(edge.data?.kind ? { kind: String(edge.data.kind) } : {}),
      ...(edge.data?.condition ? { condition: String(edge.data.condition) } : {}),
      ...(edge.data?.loop ? { loop: true } : {}),
    })),
    ...(deliveryConfig ? { delivery_progress: deliveryConfig } : {}),
  }), [deliveryConfig, edges, nodes])
  const currentDefinition = definition()
  const dirty = JSON.stringify(currentDefinition) !== JSON.stringify(templateDefinition)
  const onConnect = useCallback((connection: Connection) => setEdges((current) => addEdge({ ...connection, id: `e-${connection.source}-${connection.target}-${Date.now()}`, data: { kind: '', condition: '', loop: false } }, current)), [setEdges])
  const addNode = (type: string, label: string) => {
    if ((type === 'start' || type === 'end') && nodes.some((node) => node.data.type === type)) return
    const defaultCapability = type === 'ai'
      ? workflowCapabilities.find((capability) => capability.key === aiDefaultCapability[template.subject_type] && isWorkflowCapability(capability, template.subject_type))
        ?? workflowCapabilities.find((capability) => isWorkflowCapability(capability, template.subject_type))
      : undefined
    if (type === 'ai' && !defaultCapability) return
    const persistedType = defaultCapability ? aiNodeTypeForCapability(defaultCapability) : type
    const id = type === 'start' || type === 'end' ? type : `${type}-${Date.now()}`
    const config: WorkflowNodeConfig = {
      ...(type === 'approval' ? { critical_gate: true } : {}),
    }
    setNodes((current) => [...current, { id, type: 'workflow', position: { x: 120 + current.length * 25, y: 100 + current.length * 22 }, data: { id, type: persistedType, name: label, ...(defaultCapability ? { capability: defaultCapability.key } : {}), ...(Object.keys(config).length ? { config } : {}) } }])
    setSelectedId(id)
    setRightTab('inspector')
  }
  const applyProposal = async (next: WorkflowDefinition) => {
    const assignedProposal = withoutTemplateAssignees(next)
    const previous = new Map(currentDefinition.nodes.map((node) => [node.id, JSON.stringify(node)]))
    const changedIds = new Set(assignedProposal.nodes.filter((node) => previous.get(node.id) !== JSON.stringify(node)).map((node) => node.id))
    const arranged = layout(assignedProposal)
    const highlighted = arranged.nodes.map((node) => ({ ...node, data: { ...node.data, ...(changedIds.has(node.id) ? { aiRecentlyApplied: true } : {}) } }))
    const focusNode = highlighted.find((node) => changedIds.has(node.id) && !['start', 'end'].includes(String((node.data as Record<string, unknown>).type))) ?? highlighted.find((node) => changedIds.has(node.id))
    setNodes(highlighted)
    setEdges(arranged.edges)
    setSelectedId(focusNode?.id ?? null)
    setSelectedEdgeId(null)
    setRightTab(focusNode ? 'inspector' : 'ai')
    setApplyNotice(true)
    if (highlightTimer.current) window.clearTimeout(highlightTimer.current)
    highlightTimer.current = window.setTimeout(() => {
      setNodes((current) => current.map((node) => node.data.aiRecentlyApplied ? { ...node, data: { ...node.data, aiRecentlyApplied: false } } : node))
      setApplyNotice(false)
    }, 3600)
    window.requestAnimationFrame(() => flowInstance.current?.fitView({ padding: 0.22, duration: 450 }))
    await mutations.save.mutateAsync({ id: template.id, name: template.name, description: template.description ?? '', definition: assignedProposal })
  }
  const save = () => mutations.save.mutate({ id: template.id, name: template.name, description: template.description ?? '', definition: currentDefinition })
  const publish = () => mutations.publish.mutate(template.id)
  useEffect(() => () => { if (highlightTimer.current) window.clearTimeout(highlightTimer.current) }, [])
  useEffect(() => {
    if (aiRunState.busy && !previousAIRunBusy.current && window.matchMedia('(max-width: 1279px)').matches) {
      window.requestAnimationFrame(() => canvasSection.current?.scrollIntoView({ behavior: 'smooth', block: 'start' }))
    }
    previousAIRunBusy.current = aiRunState.busy
  }, [aiRunState.busy])

  const selectNode = (id: string) => { setSelectedId(id); setSelectedEdgeId(null); setRightTab('inspector') }
  const selectEdge = (id: string) => { setSelectedEdgeId(id); setSelectedId(null); setRightTab('inspector') }

  return <OperationalPage
    title={template.name}
    description={workflowCopy.subjectAndVersion(template.subject_type === 'requirement' ? workflowCopy.requirement : workflowCopy.planUnit, template.published_version)}
    actions={<><Button variant="outline" onClick={onBack}><PhArrowLeft />{commonCopy.back}</Button>{access.canUpdateTemplate() ? <Button variant="outline" disabled={!access.canUpdateTemplate(dirty) || mutations.save.isPending} onClick={save}><PhFloppyDisk />{workflowCopy.saveDraft}</Button> : null}{access.canPublishTemplate() ? <Button disabled={dirty || aiRunState.busy || mutations.save.isPending || mutations.publish.isPending} onClick={publish}><PhPaperPlaneTilt />{workflowCopy.publish}</Button> : null}</>}
    context={<div className="grid gap-2"><div className="flex flex-wrap items-center gap-2">{dirty ? <StatusPill tone="warning">{workflowCopy.unsaved}</StatusPill> : <StatusPill tone="success"><PhCheckCircle className="mr-1 size-3" />{workflowCopy.saved}</StatusPill>}<span className="text-xs text-muted-foreground">{workflowCopy.immutableHint}</span></div>{mutations.publish.isError ? <p role="alert" className="m-0 text-xs text-red-700">{workflowCopy.publishFailed}</p> : null}</div>}
  >
    <div className="grid min-h-[calc(100dvh-10rem)] grid-cols-1 xl:grid-cols-[14rem_minmax(0,1fr)_24rem]">
      <aside className="order-3 border-t border-border bg-card p-3 xl:order-1 xl:border-t-0 xl:border-r">
        <h2 className="m-0 text-xs font-semibold text-muted-foreground">{workflowCopy.catalog}</h2>
        <p className="mt-1 mb-0 text-xs leading-4 text-muted-foreground">{workflowCopy.catalogHint}</p>
        <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-1">{visibleNodeCatalog.map((entry) => {
          const exists = Boolean(entry.singleton && nodes.some((node) => node.data.type === entry.type))
          return <Button key={entry.type} variant="outline" className="justify-start" disabled={exists} title={exists ? workflowCopy.singletonExists : undefined} onClick={() => addNode(entry.type, entry.label)}><PhPlus className="size-3.5" />{entry.label}</Button>
        })}</div>
        {capabilityDirectory.status === 'error' && !selectedIsAI ? <AlertBanner tone="error" className="mt-3" action={<Button type="button" size="xs" variant="outline" onClick={capabilityDirectory.retry}>{copy.ui.retry}</Button>}>{workflowCopy.aiCapabilityLoadFailed}</AlertBanner> : capabilityDirectory.status === 'loading' && !selectedIsAI ? <p className="mt-3 mb-0 text-xs leading-4 text-muted-foreground">{workflowCopy.aiCapabilityLoading}</p> : null}
        <div className="mt-4 rounded-md bg-muted p-3 text-xs leading-5 text-muted-foreground">{workflowCopy.safetyHint}</div>
      </aside>
      <section ref={canvasSection} aria-label={workflowCopy.canvas} className="relative order-2 min-h-[34rem] bg-muted/30 xl:order-2">
        <ReactFlow className="[&_.react-flow__attribution_a]:!text-foreground" fitView nodes={nodes} edges={edges} nodeTypes={workflowNodeTypes} nodesDraggable={access.canUpdateTemplate()} nodesConnectable={access.canUpdateTemplate()} edgesReconnectable={access.canUpdateTemplate()} elementsSelectable nodesFocusable edgesFocusable onInit={(instance) => { flowInstance.current = instance }} onNodesChange={onNodesChange} onEdgesChange={onEdgesChange} onConnect={access.canUpdateTemplate() ? onConnect : undefined} onNodeClick={(_, node) => selectNode(node.id)} onEdgeClick={(_, edge) => selectEdge(edge.id)} onPaneClick={() => { setSelectedId(null); setSelectedEdgeId(null) }}>
          <Background /><MiniMap pannable zoomable /><Controls />
        </ReactFlow>
        {aiRunState.busy ? <CanvasAILoading state={aiRunState} cancelling={cancelAIRun.isPending} onCancel={() => aiRunState.runId && cancelAIRun.mutate(aiRunState.runId)} /> : null}
        {applyNotice && !aiRunState.busy ? <div role="status" className="pointer-events-none absolute top-4 left-1/2 z-20 flex -translate-x-1/2 items-center gap-2 rounded-full border border-emerald-200 dark:border-emerald-800 bg-card/95 px-4 py-2 text-xs font-medium text-emerald-800 dark:text-emerald-300 shadow-[var(--design-shadow-popover)]"><PhCheckCircle className="size-4" />{workflowCopy.aiAppliedNotice}</div> : null}
      </section>
      <aside className="order-1 border-t border-border bg-card xl:order-3 xl:border-t-0 xl:border-l">
        <div className="sticky top-0 z-10 grid grid-cols-2 border-b border-border bg-card p-2">
          <Button type="button" variant="ghost" aria-pressed={rightTab === 'ai'} className={rightTab === 'ai' ? 'bg-primary/10 text-foreground hover:bg-primary/10' : 'text-muted-foreground'} onClick={() => setRightTab('ai')}><PhRobot className="mr-1 inline size-3.5" />{workflowCopy.aiTab}</Button>
          <Button type="button" variant="ghost" aria-pressed={rightTab === 'inspector'} className={rightTab === 'inspector' ? 'bg-muted text-foreground' : 'text-muted-foreground'} onClick={() => setRightTab('inspector')}><PhGitBranch className="size-3.5" />{workflowCopy.inspectorTab}</Button>
          {template.subject_type === 'plan_unit' ? <Button type="button" variant="ghost" aria-pressed={rightTab === 'delivery'} className={rightTab === 'delivery' ? 'bg-muted text-foreground' : 'text-muted-foreground'} onClick={() => setRightTab('delivery')}><PhChartBar className="size-3.5" />{workflowCopy.deliveryPanel}</Button> : null}
        </div>
        <div className="p-3">
          <div className={rightTab === 'ai' ? 'block' : 'hidden'}><WorkflowAIAssistant workspaceId={workspaceId} templateId={template.id} definition={currentDefinition} dirty={dirty} onApply={applyProposal} onRunStateChange={setAIRunState} /></div>
          {rightTab === 'inspector' ? <InspectorPanel subjectType={template.subject_type} capabilityDirectory={capabilityDirectory} requirementTemplates={requirementTemplates} selected={selected} selectedEdge={selectedEdge} setNodes={setNodes} setEdges={setEdges} clearNode={() => setSelectedId(null)} clearEdge={() => setSelectedEdgeId(null)} /> : null}
          {rightTab === 'delivery' && template.subject_type === 'plan_unit' ? (
            <div className="mt-3 grid gap-3">
              <WorkflowDeliveryProgressEditor
                value={deliveryConfig}
                requirementTemplates={requirementTemplates}
                onChange={setDeliveryConfig}
              />
            </div>
          ) : null}
        </div>
      </aside>
    </div>
  </OperationalPage>
}
