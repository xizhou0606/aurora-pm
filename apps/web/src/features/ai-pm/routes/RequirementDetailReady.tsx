import { useEffect, useRef, useState } from 'react'
import { PhArrowLeft, PhFloppyDisk } from '@/shared/ui/icons'
import { isSafeApiError, PM_API_ERROR_CODES } from '@/shared/api'
import { Button, Input, Label, MarkdownTextField, OperationalPage, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, StatusPill, Textarea } from '@/shared/ui'
import { useRequirementMutations, useRequirementScope, useWorkflowAssignees, useWorkflowInstance } from '../api/api'
import { WorkflowNodeDeliverables } from '../components/WorkflowNodeDeliverables'
import { RequirementDocuments } from '../components/RequirementDocuments'
import { WorkflowSubjectLauncher } from '../components/WorkflowSubjectLauncher'
import { RequirementRoleOwnerFields, sameOwnerIDs } from '../utils/requirement-role-owners'
import { ScheduleMutationNotice } from '../components/WorkflowSchedule'
import { workflowPeopleDirectory } from '../components/WorkflowTaskPlanOwners'
import type { Requirement } from '../model/types'
import { useRequirementsOperationAccess } from '../hooks/useOperationAccess'
import {
  type RequirementEditor,
  type RequirementEditable,
  detailCopy,
  commonCopy,
  poolCopy,
  statusLabels,
  requirementStatusTone,
  priorityOptions,
  sameEditable,
  reconcileRequirement,
  editableFields,
  initialEditor,
} from './RequirementDetailPage'

export function RequirementDetailReady({ initialNodeId, onBack, requirement, workspaceId, reloadRequirement }: {
  initialNodeId?: string
  onBack: () => void
  requirement: Requirement
  workspaceId: number
  reloadRequirement: () => Promise<Requirement | undefined>
}) {
  'use no memo'
  const mutations = useRequirementMutations(workspaceId)
  const access = useRequirementsOperationAccess()
  const scheduleStaleError = isSafeApiError(mutations.schedule.error) &&
    mutations.schedule.error.code === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE
  const scheduleReloadLock = useRef(false)
  const [scheduleReloading, setScheduleReloading] = useState(false)
  const [editor, setEditor] = useState<RequirementEditor>(() => initialEditor(requirement))
  const [schedule, setSchedule] = useState(() => {
    const id = requirement.version_task_new_id ?? 0
    return {
      baseId: id,
      draftId: id,
      baseVersion: requirement.version,
      baseProjectId: requirement.project_new_id,
    }
  })
  const scope = useRequirementScope(workspaceId)
  const units = (scope.data?.projects ?? []).filter((unit) => unit.parentId === requirement.project_new_id)
  const workflowInstance = useWorkflowInstance(workspaceId, requirement.workflow_instance_id ?? 0)
  const peopleQuery = useWorkflowAssignees(workspaceId)
  useEffect(() => {
    setEditor((current) => reconcileRequirement(current, requirement))
    setSchedule((current) => {
      if (scheduleStaleError) return current
      const incomingId = requirement.version_task_new_id ?? 0
      const incomingProjectId = requirement.project_new_id
      if (requirement.version < current.baseVersion) return current
      if (incomingProjectId === current.baseProjectId && incomingId === current.baseId) {
        return { ...current, baseVersion: requirement.version }
      }
      if (
        (incomingProjectId === current.baseProjectId && incomingId === current.draftId) ||
        current.draftId === current.baseId
      ) {
        return {
          baseId: incomingId,
          draftId: incomingId,
          baseVersion: requirement.version,
          baseProjectId: incomingProjectId,
        }
      }
      return current
    })
  }, [requirement, scheduleStaleError])
  const changed = !sameEditable(editor.draft, editor.base.fields)
  const changeDraft = (patch: Partial<RequirementEditable>) => {
    mutations.patch.reset()
    setEditor((current) => ({ ...current, draft: { ...current.draft, ...patch } }))
  }
  const save = () => {
    const submitted = editor.draft
    mutations.patch.mutate({
      id: requirement.id,
      expected_version: editor.base.version,
      ...(submitted.title !== editor.base.fields.title ? { title: submitted.title } : {}),
      ...(submitted.description !== editor.base.fields.description ? { description: submitted.description } : {}),
      ...(submitted.acceptance_criteria !== editor.base.fields.acceptance_criteria ? { acceptance_criteria: submitted.acceptance_criteria } : {}),
      ...(submitted.priority !== editor.base.fields.priority ? { priority: submitted.priority } : {}),
      ...(!sameOwnerIDs(submitted.product_owner_user_ids, editor.base.fields.product_owner_user_ids) ? { product_owner_user_ids: submitted.product_owner_user_ids } : {}),
      ...(!sameOwnerIDs(submitted.design_owner_user_ids, editor.base.fields.design_owner_user_ids) ? { design_owner_user_ids: submitted.design_owner_user_ids } : {}),
      ...(!sameOwnerIDs(submitted.frontend_owner_user_ids, editor.base.fields.frontend_owner_user_ids) ? { frontend_owner_user_ids: submitted.frontend_owner_user_ids } : {}),
      ...(!sameOwnerIDs(submitted.backend_owner_user_ids, editor.base.fields.backend_owner_user_ids) ? { backend_owner_user_ids: submitted.backend_owner_user_ids } : {}),
      ...(!sameOwnerIDs(submitted.tester_user_ids, editor.base.fields.tester_user_ids) ? { tester_user_ids: submitted.tester_user_ids } : {}),
    }, {
      onSuccess: (saved) => setEditor((current) => {
        const fields = editableFields(saved)
        return {
          base: { version: saved.version, fields },
          draft: sameEditable(current.draft, submitted) ? fields : current.draft,
        }
      }),
    })
  }
  const scheduleControlledError = isSafeApiError(mutations.schedule.error) && (
    mutations.schedule.error.code === PM_API_ERROR_CODES.WORKFLOW_SCHEDULE_CONFIRMATION_REQUIRED ||
    mutations.schedule.error.code === PM_API_ERROR_CODES.WORKFLOW_SCHEDULE_CONFLICT ||
    mutations.schedule.error.code === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE
  )
  const reloadLatestSchedule = async () => {
    if (!scheduleStaleError || scheduleReloadLock.current) return
    scheduleReloadLock.current = true
    setScheduleReloading(true)
    try {
      const latest = await reloadRequirement()
      if (!latest || latest.id !== requirement.id || latest.version < schedule.baseVersion) return
      const sameProject = latest.project_new_id === schedule.baseProjectId
      let draftStillAvailable = false
      if (sameProject) {
        const refreshedScope = await scope.refetch({ throwOnError: true })
        if (!refreshedScope.data) return
        draftStillAvailable = refreshedScope.data.projects.some((unit) => (
          unit.id === schedule.draftId && unit.parentId === latest.project_new_id
        ))
      }
      const baseId = latest.version_task_new_id ?? 0
      const factsAdvanced = latest.version > schedule.baseVersion ||
        latest.project_new_id !== schedule.baseProjectId ||
        baseId !== schedule.baseId ||
        !draftStillAvailable
      if (!factsAdvanced) return
      mutations.schedule.reset()
      setSchedule({
        baseId,
        draftId: draftStillAvailable ? schedule.draftId : baseId,
        baseVersion: latest.version,
        baseProjectId: latest.project_new_id,
      })
    } catch {
      return
    } finally {
      scheduleReloadLock.current = false
      setScheduleReloading(false)
    }
  }
  const scheduleRequirement = (confirmSubjectSchedule = false) => mutations.schedule.mutate({
    id: requirement.id,
    version_task_new_id: schedule.draftId,
    expected_version: schedule.baseVersion,
    ...(confirmSubjectSchedule ? { confirm_subject_schedule: true as const } : {}),
  }, {
    onSuccess: (scheduled) => {
      setEditor((current) => reconcileRequirement(current, scheduled))
      const scheduledId = scheduled.version_task_new_id ?? 0
      setSchedule({
        baseId: scheduledId,
        draftId: scheduledId,
        baseVersion: scheduled.version,
        baseProjectId: scheduled.project_new_id,
      })
    },
  })
  return <OperationalPage
    title={requirement.title}
    description={detailCopy.headingDescription(requirement.id, editor.base.version)}
    actions={<><Button variant="outline" onClick={onBack}><PhArrowLeft aria-hidden="true" />{detailCopy.pool}</Button>{access.canUpdateRequirement() ? <Button disabled={!access.canUpdateRequirement(changed) || mutations.patch.isPending} onClick={save}><PhFloppyDisk aria-hidden="true" />{mutations.patch.isPending ? commonCopy.saving : commonCopy.save}</Button> : null}</>}
    context={<div className="flex flex-wrap items-center gap-2"><StatusPill tone={requirement.version_task_new_id ? 'info' : 'warning'}>{requirement.version_task_new_id ? detailCopy.scheduled(requirement.version_task_new_id) : poolCopy.unscheduled}</StatusPill>{requirement.workflow_instance_id ? <StatusPill tone={workflowInstance.data?.status === 'completed' ? 'success' : 'info'}>{workflowInstance.data?.status === 'completed' ? detailCopy.workflowCompletedBadge(requirement.workflow_instance_id) : detailCopy.workflowRunning(requirement.workflow_instance_id)}</StatusPill> : null}</div>}
  >
    <div className="grid gap-4 p-3 sm:p-5">
      <WorkflowSubjectLauncher workspaceId={workspaceId} subjectType="requirement" subjectId={requirement.id} instanceId={requirement.workflow_instance_id} designImageUrl={requirement.design_url} requirement={requirement} initialNodeId={initialNodeId} />
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_25rem]">
      <div className="grid content-start gap-4">
        <section className="grid gap-4 rounded-lg border border-border bg-card p-4" aria-labelledby="requirement-fields">
          <h2 id="requirement-fields" className="m-0 text-sm font-semibold">{detailCopy.fields}</h2>
          <label className="grid gap-1.5"><Label>{detailCopy.titleField}</Label><Input name="requirement-title" value={editor.draft.title} disabled={!access.canUpdateRequirement() || mutations.patch.isPending} onChange={(event) => changeDraft({ title: event.target.value })} /></label>
          <div className="grid gap-3 sm:grid-cols-2"><div className="grid gap-1.5" aria-label={commonCopy.status}><Label>{commonCopy.status}</Label><div className="flex min-h-9 flex-wrap items-center gap-x-3 gap-y-1 rounded-md border border-border bg-muted/30 px-3 py-2"><StatusPill tone={requirementStatusTone(requirement.status)}>{statusLabels[requirement.status] ?? requirement.status ?? detailCopy.statusUnknown}</StatusPill><span className="text-xs text-muted-foreground">{detailCopy.statusWorkflowOwned}</span></div></div><label className="grid gap-1.5"><Label>{commonCopy.priority}</Label><Select items={priorityOptions} value={editor.draft.priority} disabled={mutations.patch.isPending} onValueChange={(value) => changeDraft({ priority: String(value) })}><SelectTrigger name="requirement-priority"><SelectValue /></SelectTrigger><SelectContent>{priorityOptions.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent></Select></label></div>
          <MarkdownTextField label={detailCopy.descriptionField} name="requirement-description" className="min-h-36" value={editor.draft.description} disabled={mutations.patch.isPending} onChange={(description) => changeDraft({ description })} />
          <label className="grid gap-1.5"><Label>{detailCopy.acceptanceField}</Label><Textarea name="requirement-acceptance" className="min-h-36" placeholder={detailCopy.acceptancePlaceholder} value={editor.draft.acceptance_criteria} disabled={mutations.patch.isPending} onChange={(event) => changeDraft({ acceptance_criteria: event.target.value })} /></label>
          <RequirementRoleOwnerFields
            value={{
              product_owner_user_ids: editor.draft.product_owner_user_ids,
              design_owner_user_ids: editor.draft.design_owner_user_ids,
              frontend_owner_user_ids: editor.draft.frontend_owner_user_ids,
              backend_owner_user_ids: editor.draft.backend_owner_user_ids,
              tester_user_ids: editor.draft.tester_user_ids,
            }}
            users={workflowPeopleDirectory(peopleQuery)}
            disabled={!access.canAssignRoleOwners() || !access.canUpdateRequirement() || mutations.patch.isPending}
            onChange={(owners) => changeDraft(owners)}
          />
          {mutations.patch.isError ? <p role="alert" className="m-0 text-sm text-destructive">{detailCopy.saveFailed}</p> : null}
        </section>
        {access.canScheduleRequirement() ? <section className="grid min-w-0 gap-3 rounded-lg border border-border bg-card p-4"><div><h2 className="m-0 text-sm font-semibold">{detailCopy.scheduleTitle}</h2><p className="mt-1 mb-0 text-xs text-muted-foreground">{detailCopy.scheduleDescription}</p></div><div className="grid min-w-0 gap-2 sm:grid-cols-[minmax(0,1fr)_auto]"><Select items={units.map((unit) => ({ value: unit.id, label: unit.label }))} value={schedule.draftId || null} disabled={mutations.schedule.isPending || scheduleStaleError || scheduleReloading} onValueChange={(value) => { const nextId = Number(value) || 0; if (!nextId || scheduleStaleError || scheduleReloading) return; mutations.schedule.reset(); setSchedule((current) => ({ ...current, draftId: nextId })) }}><SelectTrigger name="requirement-plan-unit" aria-label={detailCopy.chooseVersion} className="min-w-0"><SelectValue placeholder={detailCopy.chooseVersion} /></SelectTrigger><SelectContent>{units.map((unit) => <SelectItem key={unit.id} value={unit.id}>{unit.label}</SelectItem>)}</SelectContent></Select><Button variant="outline" className="w-full sm:w-auto" disabled={!access.canScheduleRequirement(Boolean(schedule.draftId && schedule.draftId !== schedule.baseId)) || mutations.schedule.isPending || scheduleStaleError || scheduleReloading} onClick={() => scheduleRequirement(false)}>{detailCopy.confirmSchedule}</Button></div>{mutations.schedule.error ? <ScheduleMutationNotice error={mutations.schedule.error} instance={workflowInstance.data} pending={mutations.schedule.isPending || scheduleReloading} onAdjust={scheduleStaleError ? reloadLatestSchedule : mutations.schedule.reset} onConfirm={() => scheduleRequirement(true)} /> : null}{mutations.schedule.isError && !scheduleControlledError ? <p role="alert" className="m-0 text-sm text-destructive">{detailCopy.scheduleFailed}</p> : null}</section> : null}
      </div>
      <aside className="grid content-start gap-4"><RequirementDocuments workspaceId={workspaceId} requirementId={requirement.id} /><WorkflowNodeDeliverables workspaceId={workspaceId} requirementId={requirement.id} instance={workflowInstance.data} loading={Boolean(requirement.workflow_instance_id) && workflowInstance.isPending} error={workflowInstance.isError} designURL={requirement.design_url} requirementDocURL={requirement.product_doc_url} onRetry={() => void workflowInstance.refetch()} /></aside>
      </div>
    </div>
  </OperationalPage>
}
