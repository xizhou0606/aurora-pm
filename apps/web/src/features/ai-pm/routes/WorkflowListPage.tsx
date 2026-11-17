import { useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { PhGitBranch, PhPlus } from '@/shared/ui/icons'
import { useWorkspaceStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import { Button, DataTable, Input, Label, OperationalPage, ResponsivePanel, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, StatusPill, Textarea } from '@/shared/ui'
import { useWorkflowMutations, useWorkflows } from '../api/api'
import type { WorkflowDefinition, WorkflowTemplate } from '../model/types'
import { useWorkflowOperationAccess } from '../hooks/useOperationAccess'

const workflowCopy = copy.aiPM.workflow
const commonCopy = copy.aiPM.common

function defaultDefinition(): WorkflowDefinition {
  return {
    nodes: [
      { id: 'start', type: 'start', name: '开始' },
      { id: 'review', type: 'approval', name: '人工评审', config: { critical_gate: true } },
      { id: 'test-acceptance', type: 'human_task', name: '测试验收' },
      { id: 'end', type: 'end', name: '结束' },
    ],
    edges: [
      { id: 'e-start-review', source: 'start', target: 'review' },
      { id: 'e-review-acceptance', source: 'review', target: 'test-acceptance', kind: 'approve' },
      { id: 'e-acceptance-end', source: 'test-acceptance', target: 'end' },
    ],
  }
}

export function WorkflowListPage() {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const query = useWorkflows(workspaceId)
  const access = useWorkflowOperationAccess()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const columns = useMemo<ColumnDef<WorkflowTemplate>[]>(() => [
    { accessorKey: 'name', header: workflowCopy.template, cell: ({ row }) => <div><div className="font-medium">{row.original.name}</div><div className="mt-0.5 text-xs text-muted-foreground">{row.original.description || workflowCopy.noDescription}</div></div> },
    { accessorKey: 'subject_type', header: workflowCopy.subjectType, cell: ({ getValue }) => String(getValue()) === 'requirement' ? workflowCopy.requirement : workflowCopy.planUnit },
    { accessorKey: 'published_version', header: workflowCopy.publishStatus, cell: ({ row }) => row.original.published_version > 0 ? <StatusPill tone="success">{commonCopy.version(row.original.published_version)}</StatusPill> : <StatusPill tone="warning">{commonCopy.draft}</StatusPill> },
    { accessorKey: 'updated_at', header: commonCopy.updatedAt, cell: ({ getValue }) => <span className="text-xs text-muted-foreground">{new Date(String(getValue())).toLocaleString('zh-CN')}</span> },
  ], [])
  const rows = query.data ?? []
  if (access.status === 'ready' && !access.canViewTemplates()) return null
  return <OperationalPage title={workflowCopy.title} actions={access.canCreateTemplate() ? <Button onClick={() => setOpen(true)}><PhPlus />{workflowCopy.create}</Button> : undefined} context={<div className="flex items-center gap-2 text-xs text-muted-foreground"><PhGitBranch className="size-4 text-primary" />{workflowCopy.guidance}</div>}>
    <div className="p-3 sm:p-5"><DataTable aria-label={workflowCopy.table} columns={columns} state={query.isPending ? { status: 'loading', skeletonRows: 6 } : query.isError ? { status: 'error', onRetry: () => void query.refetch() } : rows.length ? { status: 'ready', rows, refreshing: query.isFetching } : { status: 'empty', title: workflowCopy.emptyTitle }} rowAction={{ label: (row) => workflowCopy.openRow(row.name), onActivate: (row) => void navigate({ to: '/workspace/pm/workflows/$templateId', params: { templateId: String(row.id) } } as never) }} /></div>
    <CreateWorkflowPanel open={open} onOpenChange={setOpen} workspaceId={workspaceId} onCreated={(id) => void navigate({ to: '/workspace/pm/workflows/$templateId', params: { templateId: String(id) } } as never)} />
  </OperationalPage>
}

function CreateWorkflowPanel({ open, onOpenChange, workspaceId, onCreated }: { open: boolean; onOpenChange: (open: boolean) => void; workspaceId: number; onCreated: (id: number) => void }) {
  const mutations = useWorkflowMutations(workspaceId)
  const access = useWorkflowOperationAccess()
  const [name, setName] = useState('')
  const [subjectType, setSubjectType] = useState<'requirement' | 'plan_unit'>('requirement')
  const [description, setDescription] = useState('')
  const create = () => mutations.create.mutate({ name: name.trim(), subject_type: subjectType, description: description.trim(), definition: defaultDefinition() }, { onSuccess: (result) => { onOpenChange(false); onCreated(result.id) } })
  const subjectItems = [{ value: 'requirement', label: workflowCopy.requirement }, { value: 'plan_unit', label: workflowCopy.planUnit }]
  return <ResponsivePanel open={open} onOpenChange={onOpenChange} title={workflowCopy.createTitle} description={workflowCopy.createDescription} pending={mutations.create.isPending} footer={<><Button variant="outline" onClick={() => onOpenChange(false)}>{commonCopy.cancel}</Button><Button disabled={!name.trim() || mutations.create.isPending} onClick={create}>{workflowCopy.createSubmit}</Button></>}><div className="grid gap-4"><label className="grid gap-1.5"><Label>{workflowCopy.name}</Label><Input name="workflow-name" autoFocus value={name} onChange={(event) => setName(event.target.value)} /></label><label className="grid gap-1.5"><Label>{workflowCopy.subjectType}</Label><Select items={subjectItems} value={subjectType} onValueChange={(value) => setSubjectType(String(value) as 'requirement' | 'plan_unit')}><SelectTrigger name="workflow-subject-type"><SelectValue /></SelectTrigger><SelectContent>{subjectItems.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent></Select></label><label className="grid gap-1.5"><Label>{workflowCopy.descriptionField}</Label><Textarea name="workflow-description" value={description} onChange={(event) => setDescription(event.target.value)} /></label>{mutations.create.isError ? <p role="alert" className="m-0 text-sm text-destructive">{workflowCopy.createFailed}</p> : null}</div></ResponsivePanel>
}
