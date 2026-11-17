import { useEffect, useRef, useState } from 'react'
import { useSessionStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import { REQUIREMENT_DOCUMENTS_COPY } from '@/shared/copy/requirement-documents'
import { PM_API_ERROR_CODES, isSafeApiError } from '@/shared/api'
import { RequirementDocumentField } from './RequirementDocumentField'
import { useCreateRequirementWithDocument } from '../hooks/useCreateRequirementWithDocument'
import type { RequirementDocumentChoice } from '../model/requirement-document-choice'
import { platformUserFactId } from '@/shared/runtime'
import type { PMRequirementPriority } from '@/features/project-management/api'
import { REQUIREMENT_PRIORITY_OPTIONS } from '@/features/project-management/model'
import {
  Button,
  Input,
  Label,
  MarkdownTextField,
  ResponsivePanel,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/shared/ui'
import { useRequirementScope, useWorkflowAssignees, useWorkflows } from '../api/api'
import { defaultRequirementRoleOwners, RequirementRoleOwnerFields } from '../utils/requirement-role-owners'
import { workflowPeopleDirectory } from './WorkflowTaskPlanOwners'
import { useRequirementsOperationAccess, useWorkflowOperationAccess } from '../hooks/useOperationAccess'

const poolCopy = copy.aiPM.requirementPool
const commonCopy = copy.aiPM.common

// 提交时项目才被他人锁版（409 PROJECT_REQUIREMENTS_LOCKED）：保留草稿并刷新选项。
const isLockedConflictError = (error: unknown) =>
  isSafeApiError(error) && (error as { code?: string }).code === PM_API_ERROR_CODES.PROJECT_REQUIREMENTS_LOCKED

export function CreateRequirementPanel({
  open,
  onOpenChange,
  workspaceId,
  onCreated,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  workspaceId: number
  onCreated: (id: number) => void
}) {
  'use no memo'
  const currentWorkspaceId = useRef(workspaceId)
  currentWorkspaceId.current = workspaceId
  const scope = useRequirementScope(workspaceId)
  const workflows = useWorkflows(workspaceId)
  const create = useCreateRequirementWithDocument(workspaceId)
  const mutations = { create }
  const access = useRequirementsOperationAccess()
  const workflowAccess = useWorkflowOperationAccess()
  const creatorId = platformUserFactId(useSessionStore((state) => state.user))
  const [projectId, setProjectId] = useState(0)
  const [unitId, setUnitId] = useState(0)
  const [workflowTemplateId, setWorkflowTemplateId] = useState(0)
  const [priority, setPriority] = useState<PMRequirementPriority>('p1')
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [docURL, setDocURL] = useState('')
  const [documentChoice, setDocumentChoice] = useState<RequirementDocumentChoice>()
  const [owners, setOwners] = useState(() => defaultRequirementRoleOwners(creatorId))
  const people = useWorkflowAssignees(workspaceId)
  const resetForm = () => {
    setProjectId(0)
    setUnitId(0)
    setWorkflowTemplateId(0)
    setPriority('p1')
    setTitle('')
    setDescription('')
    setDocURL('')
    setDocumentChoice(undefined)
    setOwners(defaultRequirementRoleOwners(creatorId))
    mutations.create.reset()
  }
  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) resetForm()
    onOpenChange(nextOpen)
  }
  useEffect(() => {
    setProjectId(0)
    setUnitId(0)
    setWorkflowTemplateId(0)
    setPriority('p1')
    setTitle('')
    setDescription('')
    setDocURL('')
    setDocumentChoice(undefined)
    setOwners(defaultRequirementRoleOwners(creatorId))
    mutations.create.reset()
  }, [workspaceId])
  useEffect(() => {
    if (creatorId <= 0) return
    setOwners((current) => (
      current.product_owner_user_ids.length > 0
        ? current
        : { ...current, product_owner_user_ids: [creatorId] }
    ))
  }, [creatorId])
  useEffect(() => {
    if (isLockedConflictError(create.error)) void scope.refetch()
  }, [create.error, scope])
  const units = (scope.data?.projects ?? []).filter((unit) => unit.parentId === projectId)
  const projectItems = [
    { value: 0, label: poolCopy.projectPlaceholder },
    ...(scope.data?.projectSets ?? []).map((project) => ({
      value: project.id,
      label: project.label,
    })),
  ]
  // 锁版只禁止新建需求：创建模式下锁版版本不可选；移动 / 排期模式不受影响。
  const unitItems = units.map((unit) => ({
    value: unit.id,
    label: unit.requirementsLockedAt ? poolCopy.scheduleLockedOption(unit.label) : unit.label,
    locked: Boolean(unit.requirementsLockedAt),
  }))
  const workflowItems = (workflows.data ?? [])
    .filter((workflow) => workflow.subject_type === 'requirement' && workflow.published_version > 0)
    .map((workflow) => ({
      value: workflow.id,
      label: poolCopy.workflowTemplateOption(workflow.name, workflow.published_version),
    }))
  const workflowCatalogReady = workflows.isSuccess && workflowItems.length > 0
  const workflowTemplateSelected = workflowItems.some((item) => item.value === workflowTemplateId)
  const scheduleSelected = unitItems.some((item) => item.value === unitId)
  const canSubmit = access.canCreateRequirement(workflowAccess.canViewTemplates(Boolean(projectId && scheduleSelected && workflowTemplateSelected && title.trim() && workflowCatalogReady)))
  const submit = () => {
    if (!canSubmit || (documentChoice?.type === 'internal' && !documentChoice.document)) return
    const submittedWorkspaceId = workspaceId
    mutations.create.mutate({ document: documentChoice, body: {
      project_new_id: projectId,
      version_task_new_id: unitId,
      workflow_template_id: workflowTemplateId,
      title: title.trim(),
      priority,
      description: description.trim(),
      product_doc_url: documentChoice?.type === 'internal' ? '' : docURL.trim(),
      product_owner_user_ids: owners.product_owner_user_ids,
      design_owner_user_ids: owners.design_owner_user_ids,
      frontend_owner_user_ids: owners.frontend_owner_user_ids,
      backend_owner_user_ids: owners.backend_owner_user_ids,
      tester_user_ids: owners.tester_user_ids,
    } }, {
      onSuccess: (result) => {
        if (currentWorkspaceId.current !== submittedWorkspaceId) return
        handleOpenChange(false)
        onCreated(result.id)
      },
    })
  }
  return (
    <ResponsivePanel
      open={open && access.canCreateRequirement(workflowAccess.canViewTemplates())}
      onOpenChange={handleOpenChange}
      title={poolCopy.create}
      pending={mutations.create.isPending}
      footer={(
        <>
          <Button variant="outline" onClick={() => handleOpenChange(false)}>{commonCopy.cancel}</Button>
          <Button disabled={!canSubmit || mutations.create.isPending || (documentChoice?.type === 'internal' && !documentChoice.document)} onClick={submit}>
            {mutations.create.isPending ? poolCopy.createPending : poolCopy.createSubmit}
          </Button>
        </>
      )}
    >
      <div className="grid gap-4">
        <fieldset disabled={Boolean(create.createdId) || create.isPending} className="m-0 grid min-w-0 gap-4 border-0 p-0">
        <div className="grid gap-1.5">
          <Label htmlFor="requirement-project">{poolCopy.project}</Label>
          <Select<number>
            items={projectItems}
            value={projectId}
            disabled={scope.isPending}
            onValueChange={(value) => { setProjectId(value ?? 0); setUnitId(0) }}
          >
            <SelectTrigger id="requirement-project"><SelectValue /></SelectTrigger>
            <SelectContent>
              {projectItems.map((item) => (
                <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="requirement-workflow-template">
            {poolCopy.workflowTemplate}
            <span className="ml-1 text-xs text-destructive">{copy.projectManagement.write.actions.required}</span>
          </Label>
          {workflowCatalogReady ? (
            <Select<number>
              items={workflowItems}
              value={workflowTemplateId || null}
              onValueChange={(value) => setWorkflowTemplateId(value ?? 0)}
            >
              <SelectTrigger id="requirement-workflow-template" aria-required="true">
                <SelectValue placeholder={poolCopy.workflowTemplatePlaceholder} />
              </SelectTrigger>
              <SelectContent>
                {workflowItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : workflows.isPending ? (
            <div className="rounded-md border border-dashed border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">{poolCopy.workflowTemplateLoading}</div>
          ) : workflows.isError ? (
            <p role="alert" className="m-0 flex items-center gap-2 text-xs text-destructive"><span>{poolCopy.workflowTemplateLoadFailed}</span><Button type="button" size="xs" variant="outline" onClick={() => void workflows.refetch()}>{commonCopy.retry}</Button></p>
          ) : (
            <div className="rounded-md border border-dashed border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">{poolCopy.workflowTemplateEmpty}</div>
          )}
          <span className="text-xs text-muted-foreground">{poolCopy.workflowTemplateHint}</span>
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="requirement-plan-unit">
            {poolCopy.requiredSchedule}
            <span className="ml-1 text-xs text-destructive">{copy.projectManagement.write.actions.required}</span>
          </Label>
          {projectId && unitItems.length === 0 ? (
            <div className="rounded-md border border-dashed border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">{poolCopy.scheduleEmpty}</div>
          ) : (
            <Select<number>
              items={unitItems}
              disabled={!projectId}
              value={unitId || null}
              onValueChange={(value) => setUnitId(value ?? 0)}
            >
              <SelectTrigger id="requirement-plan-unit" aria-required="true">
                <SelectValue placeholder={poolCopy.schedulePlaceholder} />
              </SelectTrigger>
              <SelectContent>
                {unitItems.map((item) => (
                  <SelectItem key={item.value} value={item.value} disabled={item.locked}>{item.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
          <span className="text-xs text-muted-foreground">{poolCopy.scheduleHint}</span>
        </div>
        <label className="grid gap-1.5">
          <Label>{poolCopy.requirementTitle}</Label>
          <Input name="requirement-title" autoFocus value={title} maxLength={220} onChange={(event) => setTitle(event.target.value)} />
        </label>
        <div className="grid gap-1.5">
          <Label htmlFor="requirement-priority">
            {commonCopy.priority}
            <span className="ml-1 text-xs text-destructive">{copy.projectManagement.write.actions.required}</span>
          </Label>
          <Select<PMRequirementPriority>
            items={REQUIREMENT_PRIORITY_OPTIONS}
            value={priority}
            onValueChange={(value) => setPriority((value ?? 'p1') as PMRequirementPriority)}
          >
            <SelectTrigger id="requirement-priority" aria-required="true"><SelectValue /></SelectTrigger>
            <SelectContent>
              {REQUIREMENT_PRIORITY_OPTIONS.map((item) => (
                <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <MarkdownTextField
          label={poolCopy.requirementDescription}
          name="requirement-description"
          className="min-h-36"
          value={description}
          disabled={mutations.create.isPending}
          onChange={setDescription}
        />
        </fieldset>
        <RequirementDocumentField projectId={projectId} projectName={scope.data?.projectSets.find(project => project.id === projectId)?.label} value={documentChoice} url={docURL} disabled={create.isPending} onChange={value => { setDocumentChoice(value); if (value.type === 'feishu') setDocURL(value.url) }} />
        <RequirementRoleOwnerFields
          value={owners}
          users={workflowPeopleDirectory(people)}
          disabled={mutations.create.isPending || Boolean(create.createdId)}
          onChange={setOwners}
        />
        {mutations.create.isError ? (
          <p role="alert" className="m-0 text-sm text-destructive">
            {create.createdId
              ? REQUIREMENT_DOCUMENTS_COPY.createLinkFailed
              : isLockedConflictError(create.error)
                ? poolCopy.createLockedFailed
                : poolCopy.createFailed}
          </p>
        ) : null}
      </div>
    </ResponsivePanel>
  )
}
