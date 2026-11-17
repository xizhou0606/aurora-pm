import { createPortal } from 'react-dom'
import { useRef, type RefObject } from 'react'

import { AlertBanner, Button } from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { useQuickCreate } from '../../hooks/collection/useQuickCreate'
import { useWorkItemClientOptions } from '../../hooks/collection/useClientOptions'
import { useWorkItemCreateWorkflow } from '../../hooks/collection/useCreate'
import type { WorkItemsEnvironment } from '../../model/environment'
import type {
  ScopeOptionsState,
  UserOptionsState,
  WorkItemsRouteState,
  WorkItemWorkflowStatus,
} from '../../model/types'
import { CreatePanel } from '../create'
import { QuickCreatePanel } from '../quick-create'

export function CreationControls({
  environment,
  route,
  scope,
  users,
  statuses,
  noticeTarget,
}: {
  environment: WorkItemsEnvironment
  route: WorkItemsRouteState
  scope: ScopeOptionsState
  users: UserOptionsState
  statuses: readonly WorkItemWorkflowStatus[]
  noticeTarget: RefObject<HTMLDivElement | null>
}) {
  const clientCatalog = useWorkItemClientOptions(
    environment,
    environment.operationAccess.status === 'ready' && Boolean(environment.operationAccess.viewClientOptions),
  )
  const create = useWorkItemCreateWorkflow({ environment, route, scope, users, statuses, clients: clientCatalog.state })
  const quickCreate = useQuickCreate({ environment, route, scope, users })
  const createTriggerRef = useRef<HTMLButtonElement>(null)
  const quickCreateTriggerRef = useRef<HTMLButtonElement>(null)

  return (
    <>
      <Button
        ref={createTriggerRef}
        type="button"
        disabled={!create.canOpen}
        aria-describedby={!create.canOpen ? 'work-items-create-unavailable' : undefined}
        onClick={create.open}
      >
        {WORK_ITEMS_COPY.create.trigger}
      </Button>
      <Button
        ref={quickCreateTriggerRef}
        type="button"
        variant="outline"
        disabled={!quickCreate.canOpen}
        aria-describedby={!quickCreate.canOpen ? 'work-items-quick-create-unavailable' : undefined}
        onClick={quickCreate.open}
      >
        {WORK_ITEMS_COPY.quickCreate.trigger}
      </Button>
      {!create.canOpen ? (
        <span id="work-items-create-unavailable" className="sr-only">
          {create.unavailable === 'workspace'
            ? WORK_ITEMS_COPY.create.workspaceUnavailable
            : WORK_ITEMS_COPY.create.unavailable}
        </span>
      ) : null}
      {!quickCreate.canOpen ? (
        <span id="work-items-quick-create-unavailable" className="sr-only">
          {WORK_ITEMS_COPY.quickCreate.noUser}
        </span>
      ) : null}
      {create.notice === 'attachment-error' && noticeTarget.current
        ? createPortal(
          <AlertBanner tone="warning">
            {WORK_ITEMS_COPY.create.attachmentPartialFailed}
          </AlertBanner>,
          noticeTarget.current,
        )
        : null}
      <CreatePanel
        state={create.state}
        scopeOptions={create.scopeOptions}
        requirements={create.requirements}
        users={create.users}
        usersState={create.usersState}
        statuses={create.statuses}
        clients={clientCatalog.state}
        clientMutations={clientCatalog.mutations}
        finalFocus={createTriggerRef}
        onOpenChange={(open) => open ? create.open() : create.close()}
        onUpdate={create.update}
        canAssignToMe={create.canAssignToMe}
        canUploadAttachments={create.canUploadAttachments}
        onAssignToMe={create.assignToMe}
        onAddAttachment={create.addAttachment}
        onRemoveAttachment={create.removeAttachment}
        onUploadRichTextImage={create.uploadRichTextImage}
        onSubmit={create.submit}
      />
      <QuickCreatePanel
        state={quickCreate.state}
        scopeOptions={create.scopeOptions}
        users={create.users}
        clients={clientCatalog.state.rows}
        finalFocus={quickCreateTriggerRef}
        onOpenChange={(open) => open ? quickCreate.open() : quickCreate.close()}
        onAddTask={quickCreate.addTask}
        onApplyTaskToAll={quickCreate.applyTaskToAll}
        onRemoveTask={quickCreate.removeTask}
        onUpdateTask={quickCreate.updateTask}
        onAddChild={quickCreate.addChild}
        onRemoveChild={quickCreate.removeChild}
        onUpdateChild={quickCreate.updateChild}
        onSubmit={quickCreate.submit}
      />
    </>
  )
}
