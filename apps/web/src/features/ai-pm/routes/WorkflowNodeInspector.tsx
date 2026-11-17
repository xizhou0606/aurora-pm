import type { Node } from '@xyflow/react'
import { copy } from '@/shared/copy'
import { Button, Checkbox, Input, Label, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui'
import { PhTrash } from '@/shared/ui/icons'
import type { AICapability, WorkflowNodeConfig } from '../model/types'
import type { RequirementTemplateVersionOption } from '../api/workflow-queries'
import { nodeTypeLabel, aiNodeTypeForCapability, isWorkflowCapability, subjectStatusOptions } from '../utils/workflow-node-utils'
import { WorkflowRequirementsGuardEditor } from './WorkflowRequirementsGuardEditor'

const workflowCopy = copy.aiPM.workflow

export function NodeInspector({
  subjectType,
  capabilityDirectory,
  requirementTemplates,
  node,
  update,
  remove,
}: {
  subjectType: 'requirement' | 'plan_unit'
  capabilityDirectory: { status: 'loading' | 'error' | 'ready'; items: AICapability[]; retry: () => void }
  requirementTemplates?: {
    data?: RequirementTemplateVersionOption[]
    isPending: boolean
    isError: boolean
    refetch: () => void
  }
  node: Node
  update: (data: Record<string, unknown>) => void
  remove: () => void
}) {
  const type = String(node.data.type)
  const protectedNode = type === 'start' || type === 'end'
  const config = (node.data.config ?? {}) as WorkflowNodeConfig
  const updateConfig = (patch: Record<string, unknown>) => update({ config: { ...config, ...patch } })
  const statusItems = subjectStatusOptions[subjectType].map(([value, label]) => ({ value: value || '__automatic__', label }))
  const stageItems = [
    { value: '__automatic__', label: workflowCopy.stageAutomatic },
    { value: 'backlog', label: workflowCopy.stageBacklog }, { value: 'grooming', label: workflowCopy.stageGrooming },
    { value: 'development', label: workflowCopy.stageDevelopment }, { value: 'acceptance', label: workflowCopy.stageAcceptance },
    { value: 'done', label: workflowCopy.stageDone }, { value: 'none', label: workflowCopy.stageNone },
  ]
  const supportedCapabilities = capabilityDirectory.items.filter((capability) => isWorkflowCapability(capability, subjectType))
  const currentCapability = capabilityDirectory.items.find((capability) => capability.key === node.data.capability)
  const selectedCapability = supportedCapabilities.find((capability) => capability.key === node.data.capability)
  const selectItems = supportedCapabilities.map((capability) => ({ value: capability.key, label: capability.name }))
  const currentCapabilityLabel = currentCapability?.name || String(node.data.name || workflowCopy.aiCapability)
  const legacyCapability = Boolean(node.data.capability && !selectedCapability)
  const capabilityControl = capabilityDirectory.status === 'error'
    ? <div role="alert" className="grid gap-2 rounded-md bg-red-50 p-2.5 text-xs leading-4 text-red-700"><span>{workflowCopy.aiCapabilityLoadFailed}</span>{node.data.capability ? <span>{workflowCopy.aiCapabilityCurrent(currentCapabilityLabel)}</span> : null}<Button type="button" size="xs" variant="outline" onClick={capabilityDirectory.retry}>{copy.ui.retry}</Button></div>
    : <><Select items={selectItems} value={selectedCapability?.key ?? ''} onValueChange={(value) => { const capability = supportedCapabilities.find((item) => item.key === String(value)); if (capability) update({ capability: capability.key, type: aiNodeTypeForCapability(capability) }) }} disabled={capabilityDirectory.status === 'loading' || !supportedCapabilities.length}><SelectTrigger name="workflow-ai-capability" aria-label={workflowCopy.aiCapability}><SelectValue placeholder={workflowCopy.aiCapabilityOpenedPlaceholder} /></SelectTrigger><SelectContent>{supportedCapabilities.map((capability) => <SelectItem key={capability.key} value={capability.key}>{capability.name}</SelectItem>)}</SelectContent></Select>{capabilityDirectory.status === 'loading' ? <span className="text-xs leading-4 text-muted-foreground">{workflowCopy.aiCapabilityLoading}</span> : selectedCapability ? <span className="rounded-md bg-muted/60 px-2.5 py-2 text-xs leading-4 text-muted-foreground">{selectedCapability.description}</span> : legacyCapability ? <span className="rounded-md bg-amber-50 dark:bg-amber-950/40 px-2.5 py-2 text-xs leading-4 text-amber-800">{currentCapability ? workflowCopy.aiCapabilityLegacy(currentCapability.name) : workflowCopy.aiCapabilityLegacyUnknown}</span> : null}</>
  return <div className="mt-3 grid gap-3"><label className="grid gap-1.5"><Label>{workflowCopy.nodeName}</Label><Input name="workflow-node-name" value={String(node.data.name ?? '')} onChange={(event) => update({ name: event.target.value })} /></label><div><Label>{workflowCopy.nodeType}</Label><p className="mt-1 mb-0 text-xs text-muted-foreground">{nodeTypeLabel(type)}</p></div>{!protectedNode ? <label className="grid gap-1.5"><Label>{workflowCopy.subjectStatus}</Label><Select items={statusItems} value={String(config.subject_status || '__automatic__')} onValueChange={(value) => updateConfig({ subject_status: value === '__automatic__' ? '' : String(value) })}><SelectTrigger name="workflow-subject-status"><SelectValue /></SelectTrigger><SelectContent>{statusItems.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent></Select><span className="text-xs leading-4 text-muted-foreground">{workflowCopy.subjectStatusHint}</span></label> : null}{type.startsWith('ai_') ? <div className="grid gap-1.5"><Label>{workflowCopy.aiCapability}</Label>{capabilityControl}</div> : null}{type === 'approval' ? <label className="flex min-h-10 items-center gap-2 text-xs"><Checkbox name="workflow-critical-gate" checked={Boolean(config.critical_gate)} onCheckedChange={(checked) => updateConfig({ critical_gate: checked })} />{workflowCopy.criticalField}</label> : null}{subjectType === 'requirement' && (type === 'human_task' || type === 'form') ? <label className="grid gap-1 text-xs"><span className="flex min-h-10 items-center gap-2"><Checkbox name="workflow-requirement-input" checked={Boolean(config.requirement_input)} onCheckedChange={(checked) => updateConfig({ requirement_input: checked })} />{workflowCopy.requirementInputNode}</span><span className="text-xs leading-4 text-muted-foreground">{workflowCopy.requirementInputNodeHint}</span></label> : null}{subjectType === 'plan_unit' && (type === 'human_task' || type === 'form') ? <label className="grid gap-1 text-xs"><span className="flex min-h-10 items-center gap-2"><Checkbox name="workflow-requirement-intake-lock" checked={Boolean(config.requirement_intake_lock)} onCheckedChange={(checked) => updateConfig({ requirement_intake_lock: checked })} />{workflowCopy.intakeLockField}</span><span className="text-xs leading-4 text-muted-foreground">{workflowCopy.intakeLockHint}</span></label> : null}{subjectType === 'plan_unit' && (type === 'human_task' || type === 'form' || type === 'approval') ? <label className="grid gap-1.5"><Label>{workflowCopy.stageField}</Label><Select items={stageItems} value={String(config.requirement_stage || '__automatic__')} onValueChange={(value) => updateConfig({ requirement_stage: value === '__automatic__' ? undefined : String(value) })}><SelectTrigger name="workflow-requirement-stage"><SelectValue /></SelectTrigger><SelectContent>{stageItems.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent></Select><span className="text-xs leading-4 text-muted-foreground">{workflowCopy.stageHint}</span></label> : null}{type === 'human_task' ? <label className="grid gap-1 text-xs"><span className="flex min-h-10 items-center gap-2"><Checkbox name="workflow-require-evidence" checked={Boolean(config.require_evidence)} onCheckedChange={(checked) => updateConfig({ require_evidence: checked })} />{workflowCopy.requireEvidence}</span></label> : null}{subjectType === 'plan_unit' && (type === 'human_task' || type === 'approval') && requirementTemplates ? <div className="grid gap-1 text-xs"><Label>{workflowCopy.guardField}</Label><WorkflowRequirementsGuardEditor config={config as Record<string, unknown>} requirementTemplates={requirementTemplates} onChange={updateConfig} /></div> : null}{!protectedNode ? <Button variant="destructive" onClick={remove}><PhTrash aria-hidden="true" />{workflowCopy.removeNode}</Button> : null}</div>
}