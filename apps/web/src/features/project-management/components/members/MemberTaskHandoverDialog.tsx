import { useMemo } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { copy } from '@/shared/copy'
import { workItemStatusLabel, workItemStatusTone } from '@/shared/model/work-item-status'
import { AlertBanner, Button, DataTable, ResponsivePanel, StatusPill, UserDirectorySelect, UserIdentity } from '@/shared/ui'
import type { MemberTask, PlatformUser, ProjectMember } from '../../api'
import type { MemberRemovalPreview } from '../../model/memberRemoval'
import { useMemberHandoverModel } from '../../hooks/useMemberHandoverModel'

type TaskRow = MemberTask & { assigneeName: string }

export function MemberTaskHandoverDialog({ preview, workspaceUsers, members, pending, error, onCancel, onRefresh, onConfirm }: {
  preview: MemberRemovalPreview
  workspaceUsers: readonly PlatformUser[]
  members: readonly ProjectMember[]
  pending: boolean
  error: string | null
  onCancel: () => void
  onRefresh: () => void
  onConfirm: (assigneeId: number | null) => void
}) {
  const text = copy.projectManagement.members
  const model = useMemberHandoverModel(preview, workspaceUsers, members)
  const columns = useMemo<ColumnDef<TaskRow>[]>(() => [
    { accessorKey: 'task_key', header: text.taskKey },
    { accessorKey: 'title', header: text.taskTitle, cell: ({ row }) => <span className="block min-w-40 max-w-80 whitespace-normal break-words">{row.original.title}</span> },
    { accessorKey: 'assigneeName', header: text.taskAssignee, cell: ({ row }) => <UserIdentity name={row.original.assigneeName} /> },
    { accessorKey: 'status', header: text.taskStatus, cell: ({ row }) => <StatusPill tone={workItemStatusTone(row.original.status)}>{workItemStatusLabel(row.original.status, text.unknownStatus)}</StatusPill> },
  ], [text])
  return (
    <ResponsivePanel open onOpenChange={(open) => { if (!open) onCancel() }} title={text.removalDialogTitle}
      description={text.removalDialogDescription} pending={pending} className="max-w-4xl"
      footer={(
        <div className="flex w-full flex-wrap justify-end gap-2">
          <Button variant="outline" disabled={pending} onClick={onCancel}>{text.cancelRemoval}</Button>
          <Button variant="outline" disabled={pending} onClick={onRefresh}>{text.refreshTasks}</Button>
          <Button variant="danger-soft" disabled={pending || !model.canClear} onClick={() => onConfirm(null)}>{text.clearAndRemove}</Button>
          <Button disabled={pending || !model.canAssign} onClick={() => onConfirm(model.assigneeId!)}>{text.assignAndRemove}</Button>
        </div>
      )}
    >
      <div className="space-y-4">
        <div className="space-y-1 text-sm">
          {model.summaries.map((summary) => <p key={summary.id}><span className="font-medium">{summary.text}</span>{summary.bugs ? <span className="ml-2 text-destructive">{summary.bugs}</span> : null}</p>)}
        </div>
        {preview.blockedReason ? <AlertBanner tone="warning">{preview.blockedReason}</AlertBanner> : null}
        {error ? <AlertBanner tone="error">{error}</AlertBanner> : null}
        <DataTable columns={columns} aria-label={text.taskList} scrollAreaClassName="max-h-80"
          state={model.rows.length ? { status: 'ready', rows: model.rows } : { status: 'empty', title: text.noTasks }} />
        <UserDirectorySelect id="pm-member-task-assignee" label={text.newAssignee}
          value={model.assigneeId} state={{ status: 'ready', items: model.options }} disabled={pending || Boolean(preview.blockedReason)} onValueChange={model.setAssigneeId} />
      </div>
    </ResponsivePanel>
  )
}
