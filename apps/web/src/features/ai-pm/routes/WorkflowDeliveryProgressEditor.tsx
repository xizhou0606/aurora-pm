import { copy } from '@/shared/copy'
import { Button, Checkbox, Input, Label, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui'
import { PhPlus, PhTrash } from '@/shared/ui/icons'
import type { RequirementTemplateVersionOption } from '../api/workflow-queries'
import type { WorkflowDeliveryProgressConfig, WorkflowDeliveryProgressMilestone } from '../model/types'

const workflowCopy = copy.aiPM.workflow

function defaultMilestone(): WorkflowDeliveryProgressMilestone {
  return { key: '', name: '', weight: 20, bindings: [] }
}

// WorkflowDeliveryProgressEditor edits the template-level 需求交付进度
// config: weighted milestones whose bindings reuse the requirements_guard
// template-node mapping semantics (阶段 B).
export function WorkflowDeliveryProgressEditor({
  value,
  requirementTemplates,
  onChange,
}: {
  value: WorkflowDeliveryProgressConfig | null
  requirementTemplates: {
    data?: RequirementTemplateVersionOption[]
    isPending: boolean
    isError: boolean
    refetch: () => void
  }
  onChange: (config: WorkflowDeliveryProgressConfig | null) => void
}) {
  const enabled = Boolean(value)
  const config = value ?? { schema_version: 1, milestones: [] }
  const templates = requirementTemplates.data ?? []
  const patchConfig = (patch: Partial<WorkflowDeliveryProgressConfig>) => {
    onChange({ ...config, ...patch })
  }
  const patchMilestone = (index: number, patch: Partial<WorkflowDeliveryProgressMilestone>) => {
    patchConfig({ milestones: config.milestones.map((milestone, i) => i === index ? { ...milestone, ...patch } : milestone) })
  }
  const toggleTarget = (index: number, node: { id: string; type: string }) => {
    const milestone = config.milestones[index]
    const binding = milestone.bindings[0] ?? { requirement_template_version_id: 0, condition: {} }
    const successAction = node.type === 'approval' ? 'approve' as const : 'complete' as const
    const allOf = [...(binding.condition.all_of ?? [])]
    const anyOf = [...(binding.condition.any_of ?? [])]
    const allIndex = allOf.findIndex((ref) => ref.node_id === node.id)
    const anyIndex = anyOf.findIndex((ref) => ref.node_id === node.id)
    if (allIndex >= 0) allOf.splice(allIndex, 1)
    else if (anyIndex >= 0) anyOf.splice(anyIndex, 1)
    else allOf.push({ node_id: node.id, success_action: successAction })
    const condition = allOf.length ? { all_of: allOf } : anyOf.length ? { any_of: anyOf } : {}
    patchMilestone(index, { bindings: [{ ...binding, condition }] })
  }
  return (
    <div className="grid gap-3">
      <label className="grid gap-1 text-xs">
        <span className="flex min-h-10 items-center gap-2">
          <Checkbox
            name="workflow-delivery-progress"
            checked={enabled}
            onCheckedChange={(checked) => onChange(checked ? { schema_version: 1, milestones: [] } : null)}
          />
          {workflowCopy.deliveryEnable}
        </span>
        <span className="text-xs leading-4 text-muted-foreground">{workflowCopy.deliveryPanelHint}</span>
      </label>
      {enabled ? (
        <div className="grid gap-2">
          {config.milestones.length === 0 ? (
            <p className="m-0 text-xs text-muted-foreground">{workflowCopy.deliveryMilestoneEmpty}</p>
          ) : null}
          {config.milestones.map((milestone, index) => {
            const template = templates.find((item) => item.versionId === milestone.bindings[0]?.requirement_template_version_id)
            const refs = [
              ...(milestone.bindings[0]?.condition.all_of ?? []),
              ...(milestone.bindings[0]?.condition.any_of ?? []),
            ]
            return (
              <div key={index} className="grid gap-2 rounded-md border border-border bg-card p-2.5">
                <div className="grid gap-2 sm:grid-cols-2">
                  <label className="grid gap-1 text-xs">
                    <Label>{workflowCopy.deliveryMilestoneKey}</Label>
                    <Input
                      name={`workflow-delivery-key-${index}`}
                      value={milestone.key}
                      onChange={(event) => patchMilestone(index, { key: event.target.value.trim() })}
                    />
                  </label>
                  <label className="grid gap-1 text-xs">
                    <Label>{workflowCopy.deliveryMilestoneName}</Label>
                    <Input
                      name={`workflow-delivery-name-${index}`}
                      value={milestone.name}
                      onChange={(event) => patchMilestone(index, { name: event.target.value })}
                    />
                  </label>
                  <label className="grid gap-1 text-xs">
                    <Label>{workflowCopy.deliveryMilestoneWeight}</Label>
                    <Input
                      name={`workflow-delivery-weight-${index}`}
                      type="number"
                      min={0}
                      value={milestone.weight}
                      onChange={(event) => patchMilestone(index, { weight: Math.max(0, Number(event.target.value) || 0) })}
                    />
                  </label>
                  <label className="grid content-end gap-1 text-xs">
                    <span className="flex min-h-8 items-center gap-2">
                      <Checkbox
                        name={`workflow-delivery-acceptance-${index}`}
                        checked={Boolean(milestone.is_acceptance)}
                        onCheckedChange={(checked) => {
                          const milestones = config.milestones.map((item, i) => ({
                            ...item,
                            is_acceptance: i === index ? checked : false,
                          }))
                          patchConfig({ milestones })
                        }}
                      />
                      {workflowCopy.deliveryMilestoneAcceptance}
                    </span>
                    <span className="text-xs leading-4 text-muted-foreground">{workflowCopy.deliveryMilestoneAcceptanceHint}</span>
                  </label>
                </div>
                <label className="grid gap-1 text-xs">
                  <Label>{workflowCopy.guardBindingVersion}</Label>
                  <Select
                    items={templates.map((item) => ({ value: item.versionId, label: item.label }))}
                    value={milestone.bindings[0]?.requirement_template_version_id || null}
                    onValueChange={(value) => patchMilestone(index, { bindings: [{ requirement_template_version_id: Number(value), condition: {} }] })}
                  >
                    <SelectTrigger name={`workflow-delivery-version-${index}`}><SelectValue placeholder={workflowCopy.guardBindingVersionPlaceholder} /></SelectTrigger>
                    <SelectContent>
                      {templates.map((item) => (
                        <SelectItem key={item.versionId} value={item.versionId}>{item.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </label>
                {template ? (
                  <div className="grid gap-1">
                    <span className="text-xs font-medium text-foreground">{workflowCopy.guardBindingNodes}</span>
                    <div className="grid gap-1 sm:grid-cols-2">
                      {template.nodes.map((node) => {
                        const checked = refs.some((ref) => ref.node_id === node.id)
                        return (
                          <label key={node.id} className="flex min-h-8 items-center gap-2 text-xs">
                            <Checkbox
                              name={`workflow-delivery-${index}-node-${node.id}`}
                              checked={checked}
                              onCheckedChange={() => toggleTarget(index, node)}
                            />
                            {node.name}
                          </label>
                        )
                      })}
                    </div>
                  </div>
                ) : null}
                <Button
                  type="button"
                  variant="outline"
                  size="xs"
                  className="justify-self-start bg-card"
                  onClick={() => patchConfig({ milestones: config.milestones.filter((_, i) => i !== index) })}
                >
                  <PhTrash aria-hidden="true" />{workflowCopy.deliveryMilestoneRemove}
                </Button>
              </div>
            )
          })}
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="justify-self-start bg-card"
            onClick={() => patchConfig({ milestones: [...config.milestones, defaultMilestone()] })}
          >
            <PhPlus aria-hidden="true" />{workflowCopy.deliveryAddMilestone}
          </Button>
        </div>
      ) : null}
    </div>
  )
}
