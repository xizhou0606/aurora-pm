import { PhTrash } from '@/shared/ui/icons'
import type { Edge } from '@xyflow/react'
import { copy } from '@/shared/copy'
import { Button, Checkbox, Input, Label, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui'

const workflowCopy = copy.aiPM.workflow

export function EdgeInspector({ edge, update, remove }: { edge: Edge; update: (data: Record<string, unknown>) => void; remove: () => void }) {
  const data = edge.data ?? {}
  const kindItems = [{ value: '__default__', label: workflowCopy.edgeDefault }, { value: 'yes', label: workflowCopy.edgeYes }, { value: 'no', label: workflowCopy.edgeNo }, { value: 'failure', label: workflowCopy.edgeFailure }, { value: 'approve', label: workflowCopy.edgeApprove }, { value: 'reject', label: workflowCopy.edgeReject }]
  return <div className="mt-3 grid gap-3"><label className="grid gap-1.5"><Label>{workflowCopy.edgeKind}</Label><Select items={kindItems} value={String(data.kind || '__default__')} onValueChange={(value) => update({ ...data, kind: value === '__default__' ? '' : String(value) })}><SelectTrigger name="workflow-edge-kind"><SelectValue /></SelectTrigger><SelectContent>{kindItems.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent></Select></label><label className="grid gap-1.5"><Label>{workflowCopy.edgeCondition}</Label><Input name="workflow-edge-condition" value={String(data.condition ?? '')} placeholder={workflowCopy.edgeConditionPlaceholder} onChange={(event) => update({ ...data, condition: event.target.value })} /></label><label className="flex min-h-10 items-center gap-2 text-xs"><Checkbox name="workflow-edge-loop" checked={Boolean(data.loop)} onCheckedChange={(checked) => update({ ...data, loop: checked })} />{workflowCopy.edgeLoop}</label><Button variant="destructive" onClick={remove}><PhTrash />{workflowCopy.removeEdge}</Button></div>
}