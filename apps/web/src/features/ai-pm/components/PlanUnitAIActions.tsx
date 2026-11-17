import { useMemo, useState } from 'react'
import { PhSparkle, PhMagicWand } from '@/shared/ui/icons'
import { useWorkspaceStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import { AiLoading, Button, ResponsivePanel, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, StatusPill, Textarea } from '@/shared/ui'
import { useAICapabilities, useAIMutations } from '../api/api'
import { useAIRunsOperationAccess } from '../hooks/useOperationAccess'

const planAICopy = copy.aiPM.planAI

export function PlanUnitAIActions({ planUnitId }: { planUnitId: number }) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const [open, setOpen] = useState(false)
  const [capability, setCapability] = useState('version.scope_proposal')
  const [prompt, setPrompt] = useState('')
  const [createdRun, setCreatedRun] = useState<{ id: number; capability: string; status: string } | null>(null)
  const capabilities = useAICapabilities(workspaceId)
  const access = useAIRunsOperationAccess()
  const mutations = useAIMutations(workspaceId)
  const supported = useMemo(() => (capabilities.data?.capabilities ?? []).filter((item) => item.subject_types.includes('plan_unit')), [capabilities.data])
  const selected = supported.find((item) => item.key === capability) ?? supported[0]
  const run = () => access.canCreateRun(Boolean(selected)) && selected && mutations.create.mutate({ capability: selected.key, subject_type: 'plan_unit', subject_id: planUnitId, user_prompt: prompt.trim() || undefined }, { onSuccess: setCreatedRun })

  return <>
    {access.canCreateRun() ? <Button type="button" size="sm" variant="outline" onClick={() => setOpen(true)}><PhSparkle />{planAICopy.combinedButton}</Button> : null}
    <ResponsivePanel open={open} onOpenChange={setOpen} title={planAICopy.combinedTitle} description={planAICopy.description}>
      <div className="grid gap-4">
        <div className="rounded-lg border border-primary/25 bg-primary/5 p-4"><div className="flex items-center justify-between gap-2"><strong className="flex items-center gap-2 text-sm"><PhSparkle className="size-4 text-primary" />{planAICopy.goal}</strong><StatusPill tone={capabilities.data?.enabled ? 'success' : 'danger'}>{capabilities.data?.enabled ? copy.aiPM.workflow.aiEnabled : copy.aiPM.workflow.aiDisabled}</StatusPill></div><p className="mt-1 mb-0 text-xs text-muted-foreground">{planAICopy.modelHint(capabilities.data?.model ?? copy.aiPM.workflow.modelLoading)}</p></div>
        <label className="grid gap-1.5 text-xs font-semibold">{planAICopy.capability}<Select items={supported.map((item) => ({ value: item.key, label: item.name }))} value={selected?.key ?? ''} onValueChange={(value) => setCapability(String(value))}><SelectTrigger name="plan-unit-ai-capability" className="h-9 font-normal"><SelectValue /></SelectTrigger><SelectContent>{supported.map((item) => <SelectItem key={item.key} value={item.key}>{item.name}</SelectItem>)}</SelectContent></Select></label>
        {selected ? <p className="m-0 rounded-md bg-muted/60 p-3 text-xs leading-5 text-muted-foreground">{selected.description}</p> : null}
        <label className="grid gap-1.5 text-xs font-semibold">{planAICopy.prompt} <span className="font-normal text-muted-foreground">{planAICopy.optional}</span><Textarea name="plan-unit-ai-prompt" className="min-h-24 font-normal" maxLength={4000} value={prompt} placeholder={planAICopy.promptPlaceholder} onChange={(event) => setPrompt(event.target.value)} /></label>
        <Button disabled={!capabilities.data?.enabled || !selected || mutations.create.isPending} onClick={run}>{mutations.create.isPending ? <AiLoading size={16} label={null} /> : <PhMagicWand />}{planAICopy.generate}</Button>
        {createdRun ? <div role="status" className="flex items-center justify-between rounded-md border border-border bg-muted p-3 text-xs"><span>{planAICopy.created(createdRun.capability, createdRun.id)}</span><StatusPill tone="warning">{createdRun.status}</StatusPill></div> : null}
        {mutations.create.isError ? <p role="alert" className="m-0 text-sm text-destructive">{planAICopy.failed}</p> : null}
      </div>
    </ResponsivePanel>
  </>
}
