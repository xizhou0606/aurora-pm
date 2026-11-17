import { copy } from '@/shared/copy'
import { Button, Checkbox, Input, Label, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui'
import { PhPlus, PhTrash } from '@/shared/ui/icons'
import type { RequirementTemplateVersionOption } from '../api/workflow-queries'
import type { WorkflowRequirementsGuardConfig } from '../model/types'

const workflowCopy = copy.aiPM.workflow

function defaultGuard(): WorkflowRequirementsGuardConfig {
  return {
    schema_version: 1,
    source: 'template_nodes',
    scope: 'current_plan_requirements',
    bindings: [],
    aggregate: { mode: 'all' },
    on_satisfied: 'enable_action',
    empty_scope: 'block',
    missing_binding: 'block',
  }
}

function guardFrom(config: Record<string, unknown>): WorkflowRequirementsGuardConfig | null {
  const value = config.requirements_guard
  if (!value || typeof value !== 'object') return null
  return value as WorkflowRequirementsGuardConfig
}

// WorkflowRequirementsGuardEditor edits the requirements_guard config of a
// project-node: bind published requirement template versions to their target
// nodes, choose the aggregate threshold, and keep the fail-closed defaults.
export function WorkflowRequirementsGuardEditor({
  config,
  requirementTemplates,
  onChange,
}: {
  config: Record<string, unknown>
  requirementTemplates: {
    data?: RequirementTemplateVersionOption[]
    isPending: boolean
    isError: boolean
    refetch: () => void
  }
  onChange: (patch: Record<string, unknown>) => void
}) {
  const guard = guardFrom(config)
  const enabled = Boolean(guard)
  const current = guard ?? defaultGuard()
  const templates = requirementTemplates.data ?? []
  const patchGuard = (patch: Partial<WorkflowRequirementsGuardConfig>) => {
    onChange({ requirements_guard: { ...current, ...patch } })
  }
  const patchBinding = (index: number, patch: Partial<WorkflowRequirementsGuardConfig['bindings'][number]>) => {
    patchGuard({ bindings: current.bindings.map((binding, i) => i === index ? { ...binding, ...patch } : binding) })
  }
  const toggleTarget = (index: number, node: { id: string; type: string }) => {
    const binding = current.bindings[index]
    const successAction = node.type === 'approval' ? 'approve' as const : 'complete' as const
    const allOf = [...(binding.condition.all_of ?? [])]
    const anyOf = [...(binding.condition.any_of ?? [])]
    const allIndex = allOf.findIndex((ref) => ref.node_id === node.id)
    const anyIndex = anyOf.findIndex((ref) => ref.node_id === node.id)
    if (allIndex >= 0) allOf.splice(allIndex, 1)
    else if (anyIndex >= 0) anyOf.splice(anyIndex, 1)
    else allOf.push({ node_id: node.id, success_action: successAction })
    const condition = allOf.length ? { all_of: allOf } : anyOf.length ? { any_of: anyOf } : {}
    patchBinding(index, { condition })
  }
  const aggregate = current.aggregate
  return (
    <div className="grid gap-2">
      <label className="grid gap-1 text-xs">
        <span className="flex min-h-10 items-center gap-2">
          <Checkbox
            name="workflow-requirements-guard"
            checked={enabled}
            onCheckedChange={(checked) => onChange({ requirements_guard: checked ? defaultGuard() : undefined })}
          />
          {workflowCopy.guardEnable}
        </span>
        <span className="text-xs leading-4 text-muted-foreground">{workflowCopy.guardFieldHint}</span>
      </label>
      {enabled ? (
        <div className="grid gap-3 rounded-md border border-border bg-muted/40 p-2.5">
          <label className="grid gap-1 text-xs">
            <Label>{workflowCopy.guardAggregateMode}</Label>
            <Select
              items={[
                { value: 'all', label: workflowCopy.guardAggregateAll },
                { value: 'any', label: workflowCopy.guardAggregateAny },
                { value: 'count', label: workflowCopy.guardAggregateCount },
                { value: 'ratio', label: workflowCopy.guardAggregateRatio },
              ]}
              value={aggregate.mode}
              onValueChange={(value) => patchGuard({ aggregate: { mode: String(value) as WorkflowRequirementsGuardConfig['aggregate']['mode'] } })}
            >
              <SelectTrigger name="workflow-guard-aggregate"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{workflowCopy.guardAggregateAll}</SelectItem>
                <SelectItem value="any">{workflowCopy.guardAggregateAny}</SelectItem>
                <SelectItem value="count">{workflowCopy.guardAggregateCount}</SelectItem>
                <SelectItem value="ratio">{workflowCopy.guardAggregateRatio}</SelectItem>
              </SelectContent>
            </Select>
          </label>
          {aggregate.mode === 'count' ? (
            <label className="grid gap-1 text-xs">
              <Label>{workflowCopy.guardMinCount}</Label>
              <Input
                name="workflow-guard-min-count"
                type="number"
                min={1}
                value={aggregate.min_count ?? 1}
                onChange={(event) => patchGuard({ aggregate: { ...aggregate, min_count: Math.max(1, Number(event.target.value) || 1) } })}
              />
            </label>
          ) : null}
          {aggregate.mode === 'ratio' ? (
            <label className="grid gap-1 text-xs">
              <Label>{workflowCopy.guardMinRatio}</Label>
              <Input
                name="workflow-guard-min-ratio"
                type="number"
                min={0.01}
                max={1}
                step={0.05}
                value={aggregate.min_ratio ?? 1}
                onChange={(event) => {
                  const parsed = Number(event.target.value)
                  patchGuard({ aggregate: { ...aggregate, min_ratio: Number.isFinite(parsed) && parsed > 0 ? Math.min(1, parsed) : 1 } })
                }}
              />
            </label>
          ) : null}
          {requirementTemplates.isPending ? null : requirementTemplates.isError ? (
            <p role="alert" className="m-0 flex items-center gap-2 text-xs text-destructive">
              <span>{workflowCopy.guardNoRequirementTemplates}</span>
              <Button type="button" size="xs" variant="outline" onClick={requirementTemplates.refetch}>{copy.ui.retry}</Button>
            </p>
          ) : templates.length === 0 ? (
            <p className="m-0 text-xs text-muted-foreground">{workflowCopy.guardNoRequirementTemplates}</p>
          ) : null}
          <div className="grid gap-2">
            {current.bindings.map((binding, index) => {
              const template = templates.find((item) => item.versionId === binding.requirement_template_version_id)
              const refs = [...(binding.condition.all_of ?? []), ...(binding.condition.any_of ?? [])]
              return (
                <div key={index} className="grid gap-2 rounded-md border border-border bg-card p-2.5">
                  <div className="flex items-center justify-between gap-2">
                    <label className="grid flex-1 gap-1 text-xs">
                      <Label>{workflowCopy.guardBindingVersion}</Label>
                      <Select
                        items={templates.map((item) => ({ value: item.versionId, label: item.label }))}
                        value={binding.requirement_template_version_id || null}
                        onValueChange={(value) => patchBinding(index, { requirement_template_version_id: Number(value) })}
                      >
                        <SelectTrigger name={`workflow-guard-binding-${index}`}><SelectValue placeholder={workflowCopy.guardBindingVersionPlaceholder} /></SelectTrigger>
                        <SelectContent>
                          {templates.map((item) => (
                            <SelectItem key={item.versionId} value={item.versionId}>{item.label}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </label>
                    <Button
                      type="button"
                      variant="outline"
                      size="xs"
                      className="mt-4"
                      aria-label={workflowCopy.removeNode}
                      onClick={() => patchGuard({ bindings: current.bindings.filter((_, i) => i !== index) })}
                    >
                      <PhTrash aria-hidden="true" />
                    </Button>
                  </div>
                  {template ? (
                    <div className="grid gap-1">
                      <span className="text-xs font-medium text-foreground">{workflowCopy.guardBindingNodes}</span>
                      <span className="text-xs leading-4 text-muted-foreground">{workflowCopy.guardBindingNodesHint}</span>
                      {template.nodes.length === 0 ? (
                        <span className="text-xs text-muted-foreground">{workflowCopy.guardBindingEmpty}</span>
                      ) : (
                        <div className="grid gap-1 sm:grid-cols-2">
                          {template.nodes.map((node) => {
                            const checked = refs.some((ref) => ref.node_id === node.id)
                            return (
                              <label key={node.id} className="flex min-h-8 items-center gap-2 text-xs">
                                <Checkbox
                                  name={`workflow-guard-binding-${index}-node-${node.id}`}
                                  checked={checked}
                                  onCheckedChange={() => toggleTarget(index, node)}
                                />
                                {node.name}
                              </label>
                            )
                          })}
                        </div>
                      )}
                    </div>
                  ) : null}
                </div>
              )
            })}
            {templates.length > 0 ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="justify-self-start bg-card"
                onClick={() => patchGuard({ bindings: [...current.bindings, { requirement_template_version_id: 0, condition: {} }] })}
              >
                <PhPlus aria-hidden="true" />{workflowCopy.guardAddBinding}
              </Button>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  )
}
