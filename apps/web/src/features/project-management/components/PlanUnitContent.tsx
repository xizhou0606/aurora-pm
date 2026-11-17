import { useMemo, useState } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'

import type { RequirementTableRow } from '@/features/ai-pm'
import { RequirementWorkflowNodeDialog, type RequirementWorkflowNodeDialogTarget } from '@/features/ai-pm'
import { copy } from '@/shared/copy'
import {
  Button,
  EmptyState,
  LoadingSkeleton,
  ResponsivePanel,
  buttonVariants,
} from '@/shared/ui'
import type {
  PlatformUser,
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'
import { useProjectContentWorkflow } from '../hooks'
import type { ProjectOperationAccess } from '../hooks'
import type { ProjectWorkspaceRoute, TaskDeepLinkState } from '../model'
import {
  RequirementList,
  type RequirementListActions,
  TaskList,
  type TaskListActions,
} from './ContentLists'
import {
  ContentDeleteConfirm,
  RequirementEditorPanel,
  RequirementMovePanel,
  TaskEditorPanel,
} from './ContentPanels'

type PlanUnitRoute = Extract<ProjectWorkspaceRoute, { kind: 'plan-unit' }>

function Feedback({ state }: {
  state:
    | { status: 'idle' }
    | { status: 'success'; message: string }
    | { status: 'error'; message: string }
}) {
  if (state.status === 'idle') return null
  return (
    <p
      role={state.status === 'error' ? 'alert' : 'status'}
      aria-live="polite"
      className={state.status === 'error'
        ? 'rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive'
        : 'rounded-md border border-primary/30 bg-primary/8 px-3 py-2 text-sm text-foreground'}
    >
      {state.message}
    </p>
  )
}

function EmbeddedTaskBoundary({ route }: { route: PlanUnitRoute }) {
  return (
    <EmptyState
      title={copy.projectManagement.content.host.unavailableTitle}
      action={(
        <Link
          to="/workspace/projects/$projectId/items/$projectItemId"
          params={{
            projectId: String(route.projectId),
            projectItemId: String(route.planUnitId),
          }}
          search={{ tab: 'requirements', from_item: undefined }}
          className={buttonVariants({ variant: 'outline', size: 'sm' })}
        >
          {copy.projectManagement.tabs.requirements}
        </Link>
      )}
    />
  )
}

function TaskDeepLinkBoundary({
  state,
  onClose,
}: {
  state: TaskDeepLinkState
  onClose: () => void
}) {
  const open = state.status !== 'idle' && state.status !== 'ready'
  return (
    <ResponsivePanel
      open={open}
      onOpenChange={(next) => { if (!next) onClose() }}
      title={copy.projectManagement.content.task.viewTitle}
      description={copy.projectManagement.content.task.viewDescription}
      closeLabel={copy.projectManagement.content.common.cancel}
      pending={state.status === 'loading'}
      className="max-w-lg"
    >
      {state.status === 'loading' ? (
        <LoadingSkeleton
          rows={5}
          label={copy.projectManagement.content.task.deepLinkLoading}
        />
      ) : state.status === 'error' ? (
        <EmptyState
          role="alert"
          title={copy.projectManagement.content.task.deepLinkLoadFailedTitle}
          action={(
            <Button type="button" variant="outline" onClick={state.retry}>
              {copy.projectManagement.content.common.retry}
            </Button>
          )}
        />
      ) : state.status === 'not_found' ? (
        <EmptyState
          title={copy.projectManagement.content.task.deepLinkUnavailableTitle}
        />
      ) : null}
    </ResponsivePanel>
  )
}

export function PlanUnitContent({
  route,
  project,
  unit,
  users,
  operationAccess,
}: {
  route: PlanUnitRoute
  project: ScrumProjectNew
  unit: ScrumVersionTaskNew
  users: PlatformUser[]
  operationAccess?: ProjectOperationAccess
}) {
  const navigate = useNavigate()
  const workflow = useProjectContentWorkflow({
    route, project, unit, users, operationAccess,
  })
  const [workflowTarget, setWorkflowTarget] = useState<RequirementWorkflowNodeDialogTarget | null>(null)
  const requirementActions = useMemo<RequirementListActions>(() => {
    const items = new Map<number, {
      edit?: () => void
      move?: () => void
      delete?: () => void
    }>()
    for (const requirement of workflow.requirements.rows) {
      const availability = workflow.requirements.actions(requirement)
      if (!availability.update && !availability.move && !availability.delete) continue
      items.set(requirement.id, {
        edit: availability.update
          ? () => workflow.requirements.openEdit(requirement)
          : undefined,
        move: availability.move
          ? () => workflow.requirements.openMove(requirement)
          : undefined,
        delete: availability.delete
          ? () => workflow.requirements.requestDelete(requirement)
          : undefined,
      })
    }
    return {
      // 锁版只禁止新建需求；移动 / 排期等已有需求操作不受影响。
      create: workflow.requirements.createAllowed && !unit.requirements_locked
        ? workflow.requirements.openCreate
        : undefined,
      openRequirement: (requirementId) => void navigate({
        to: '/workspace/requirements/$requirementId',
        params: { requirementId: String(requirementId) },
      } as never),
      openWorkflow: (requirement, node) => {
        setWorkflowTarget({ requirement, nodeId: node.id, nodeName: node.name })
      },
      openTasks: workflow.requirements.openTasks,
      items,
    }
  }, [navigate, unit.requirements_locked, workflow.requirements])
  const taskActions = useMemo<TaskListActions>(() => {
    const items = new Map<number, { edit?: () => void; delete?: () => void }>()
    for (const task of workflow.tasks.rows) {
      const availability = workflow.tasks.actions(task)
      if (!availability.update && !availability.delete) continue
      items.set(task.id, {
        edit: availability.update ? () => workflow.tasks.openEdit(task) : undefined,
        delete: availability.delete ? () => workflow.tasks.requestDelete(task) : undefined,
      })
    }
    return {
      create: workflow.tasks.createAllowed ? workflow.tasks.openCreate : undefined,
      items,
    }
  }, [workflow.tasks])

  return (
    <div className="grid min-w-0 gap-3">
      <Feedback state={workflow.feedback as any} />
      <RequirementWorkflowNodeDialog target={workflowTarget} onDismiss={() => setWorkflowTarget(null)} />
      {route.tab === 'requirements' ? (
        <RequirementList
          state={workflow.requirements.state}
          rows={workflow.requirements.rows}
          project={project}
          unit={unit}
          people={workflow.requirements.users}
          filters={workflow.requirements.filters}
          actions={requirementActions}
        />
      ) : workflow.embedded ? (
        <EmbeddedTaskBoundary route={route} />
      ) : (
        <TaskList
          state={workflow.tasks.state}
          filters={workflow.tasks.filters}
          actions={taskActions}
        />
      )}
      <RequirementEditorPanel
        state={workflow.requirementEditor}
        projects={workflow.projectsState}
        planUnits={workflow.planUnitsState}
        workflowTemplates={workflow.workflowTemplatesState}
        users={workflow.userState}
        fields={workflow.fields}
        onChange={workflow.changeRequirementDraft}
        onClose={workflow.closeRequirementEditor}
        onSubmit={workflow.submitRequirement}
      />
      <TaskEditorPanel
        state={workflow.taskEditor}
        users={workflow.userState}
        onChange={workflow.changeTaskDraft}
        onClose={workflow.closeTaskEditor}
        onSubmit={workflow.submitTask}
      />
      <TaskDeepLinkBoundary
        state={workflow.taskDeepLink}
        onClose={workflow.closeTaskEditor}
      />
      <RequirementMovePanel
        state={workflow.move}
        projects={workflow.projectsState}
        planUnits={workflow.planUnitsState}
        onChange={workflow.changeMoveDraft}
        onClose={workflow.closeMove}
        onSubmit={() => workflow.submitMove(false)}
        onReloadLatest={workflow.reloadLatestMove}
        onConfirmSchedule={() => workflow.submitMove(true)}
      />
      <ContentDeleteConfirm
        state={workflow.deleting}
        onClose={workflow.closeDelete}
        onConfirm={workflow.confirmDelete}
      />
    </div>
  )
}
