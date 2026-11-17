import { PhPlus } from '@/shared/ui/icons'
import { useState, type RefObject } from 'react'

import type { PMScopeOptions } from '@/shared/api'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { Button, ConfirmDialog, ResponsivePanel } from '@/shared/ui'
import type {
  QuickTaskChildDraft,
  QuickTaskChildField,
  QuickTaskCreateState,
  QuickTaskDraft,
  QuickTaskField,
  WorkItemUserOption,
  WorkItemClientOption,
} from '../../model/types'
import { TaskBlock } from './TaskBlock'

const FORM_ID = 'work-items-quick-create-form'

export function QuickCreatePanel({
  state,
  scopeOptions,
  users,
  clients,
  finalFocus,
  onOpenChange,
  onAddTask,
  onApplyTaskToAll,
  onRemoveTask,
  onUpdateTask,
  onAddChild,
  onRemoveChild,
  onUpdateChild,
  onSubmit,
}: {
  state: QuickTaskCreateState
  scopeOptions?: PMScopeOptions
  users: readonly WorkItemUserOption[]
  clients: readonly WorkItemClientOption[]
  finalFocus: RefObject<HTMLElement | null>
  onOpenChange: (open: boolean) => void
  onAddTask: () => void
  onApplyTaskToAll: (clientId: string) => void
  onRemoveTask: (clientId: string) => void
  onUpdateTask: <TKey extends QuickTaskField>(clientId: string, field: TKey, value: QuickTaskDraft[TKey]) => void
  onAddChild: (clientId: string) => void
  onRemoveChild: (parentId: string, childId: string) => void
  onUpdateChild: <TKey extends QuickTaskChildField>(parentId: string, childId: string, field: TKey, value: QuickTaskChildDraft[TKey]) => void
  onSubmit: () => Promise<boolean>
}) {
  const [confirmClose, setConfirmClose] = useState(false)
  const editing = state.status === 'editing' ? state : undefined
  const pending = editing?.submit === 'pending'
  const hasErrors = Boolean(editing && Object.keys(editing.errors).length)
  return (
    <>
    <ResponsivePanel open={Boolean(editing)} onOpenChange={(open) => {
      if (open) onOpenChange(true)
    }}
      onCloseRequest={() => {
        if (editing && !pending) {
          setConfirmClose(true)
          return false
        }
      }}
      title={WORK_ITEMS_COPY.quickCreate.title}
      closeLabel={WORK_ITEMS_COPY.create.cancel} pending={pending} finalFocus={finalFocus}
      className="h-[80dvh] max-w-[96rem]" contentClassName="bg-muted/20"
      footer={(
        <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex flex-wrap gap-2">
            <Button type="button" variant="outline" disabled={pending || !editing} onClick={onAddTask}>
              <PhPlus aria-hidden="true" />{WORK_ITEMS_COPY.quickCreate.addTask}
            </Button>
          </div>
          <Button type="submit" form={FORM_ID} disabled={pending}>
            {pending ? WORK_ITEMS_COPY.quickCreate.pending : WORK_ITEMS_COPY.quickCreate.submit}
          </Button>
        </div>
      )}>
      {editing && scopeOptions ? (
        <form id={FORM_ID} className="grid gap-3" onSubmit={(event) => { event.preventDefault(); void onSubmit() }}>
          {editing.tasks.map((task, index) => (
            <TaskBlock key={task.clientId} task={task} index={index} errors={editing.errors}
              scopeOptions={scopeOptions} users={users}
              clients={clients} disabled={pending} removable={editing.tasks.length > 1}
              onUpdate={onUpdateTask} onRemove={onRemoveTask} onAddChild={onAddChild}
              onApplyToAll={onApplyTaskToAll} onUpdateChild={onUpdateChild} onRemoveChild={onRemoveChild} />
          ))}
          {hasErrors || editing.submit === 'error' ? (
            <p role="alert" className="m-0 text-sm text-destructive">
              {hasErrors ? WORK_ITEMS_COPY.quickCreate.fieldError : WORK_ITEMS_COPY.quickCreate.failed}
            </p>
          ) : null}
        </form>
      ) : null}
    </ResponsivePanel>
    <ConfirmDialog
      open={confirmClose && Boolean(editing)}
      onOpenChange={setConfirmClose}
      title={WORK_ITEMS_COPY.quickCreate.unsavedTitle}
      description={WORK_ITEMS_COPY.quickCreate.unsavedDescription}
      cancelLabel={WORK_ITEMS_COPY.create.cancel}
      confirmLabel={WORK_ITEMS_COPY.quickCreate.discard}
      onConfirm={() => {
        setConfirmClose(false)
        onOpenChange(false)
      }}
    />
    </>
  )
}
