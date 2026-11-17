import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import type { WorkItemsEnvironment } from '../../model/environment'
import type {
  ScopeOptionsState,
  UserOptionsState,
  WorkItemCreateDraft,
  WorkItemCreateState,
  WorkItemCreateValidation,
  WorkItemsRouteState,
  WorkItemWorkflowStatus,
  WorkItemClientOptionsState,
} from '../../model/types'
import { appendPendingAttachments } from '../../model/attachments'
import { isWorkItemInlineImage } from '../../model/attachments'
import { uploadWorkItemRichTextImage } from '../../api/write-batch'
import {
  buildRequirementOptions,
  createPayload,
  initialCreateDraft,
  selectedRequirementOption,
  validateCreateDraft,
} from '../../model/create'
import { workItemsPath } from '../../model/utils'
import {
  useWorkItemCreateAttachmentsMutation,
  useWorkItemCreateMutation,
} from './useMutations'
import { requireWorkItemsAction } from '../useOperationAccess'

export function useWorkItemCreateWorkflow(input: {
  environment: WorkItemsEnvironment
  route: WorkItemsRouteState
  scope: ScopeOptionsState
  users: UserOptionsState
  statuses: readonly WorkItemWorkflowStatus[]
  clients: WorkItemClientOptionsState
}) {
  const { environment, route, scope, statuses, users, clients } = input
  const mutation = useWorkItemCreateMutation(environment)
  const attachmentMutation = useWorkItemCreateAttachmentsMutation(environment)
  const [draft, setDraft] = useState<WorkItemCreateDraft | null>(null)
  const [validation, setValidation] = useState<WorkItemCreateValidation>()
  const [notice, setNotice] = useState<'attachment-error'>()
  const ticket = `${environment.workspaceId}:${environment.sessionRevision}`
  const liveTicket = useRef(ticket)
  liveTicket.current = ticket
  const options = useMemo(
    () => scope.status === 'ready' ? buildRequirementOptions(scope.data) : [],
    [scope],
  )
  const canOpen = environment.allowed && environment.createAllowed && environment.workspaceId > 0
    && scope.status === 'ready'
  const canUploadAttachments = Boolean(
    environment.operationAccess.status === 'ready'
    && environment.operationAccess.uploadAttachment
    && environment.client.upload,
  )

  useEffect(() => {
    setDraft(null)
    setValidation(undefined)
    setNotice(undefined)
    mutation.reset()
    attachmentMutation.reset()
  }, [ticket])

  const open = useCallback(() => {
    if (!canOpen || scope.status !== 'ready') return
    mutation.reset()
    attachmentMutation.reset()
    setValidation(undefined)
    setNotice(undefined)
    const directory = users.status === 'ready' ? users.data : []
    const currentUser = directory.some((user) => user.id === environment.userId)
      ? environment.userId
      : undefined
    const defaultStatus = statuses.some((status) => status.key === 'draft')
      ? 'draft'
      : statuses[0]?.key
    setDraft(initialCreateDraft(route.scope, scope.data, {
      assigneeId: currentUser,
      status: defaultStatus,
    }))
  }, [attachmentMutation.reset, canOpen, environment.userId, mutation.reset, route.scope, scope, statuses, users])
  const close = useCallback(() => {
    if (mutation.isPending || attachmentMutation.isPending) return
    setDraft(null)
    setValidation(undefined)
    mutation.reset()
    attachmentMutation.reset()
  }, [attachmentMutation.isPending, attachmentMutation.reset, mutation.isPending, mutation.reset])
  const update = useCallback(<TKey extends keyof WorkItemCreateDraft>(
    key: TKey,
    value: WorkItemCreateDraft[TKey],
  ) => {
    setValidation(undefined)
    mutation.reset()
    setDraft((current) => current ? { ...current, [key]: value } : current)
  }, [mutation.reset])
  const canAssignToMe = users.status === 'ready'
    && users.data.some((user) => user.id === environment.userId)
  useEffect(() => {
    if (!canAssignToMe) return
    setDraft((current) => current && current.assigneeId === undefined
      ? { ...current, assigneeId: environment.userId }
      : current)
  }, [canAssignToMe, environment.userId])
  const assignToMe = useCallback(() => {
    if (!canAssignToMe) return
    update('assigneeId', environment.userId)
  }, [canAssignToMe, environment.userId, update])
  const addAttachment = useCallback((file: File) => {
    if (!canUploadAttachments) return
    setDraft((current) => current ? {
      ...current,
      attachments: appendPendingAttachments(current.attachments, [file]),
    } : current)
  }, [canUploadAttachments])
  const removeAttachment = useCallback((file: File) => {
    setDraft((current) => current ? {
      ...current,
      attachments: current.attachments.filter((entry) => entry !== file),
    } : current)
  }, [])
  const uploadRichTextImage = useCallback(async (file: File) => {
    if (!draft || !environment.client.upload || !isWorkItemInlineImage(file)) return null
    requireWorkItemsAction(canUploadAttachments)
    const requirement = selectedRequirementOption(draft.requirementScope, options)
    return uploadWorkItemRichTextImage(
      environment.client,
      requirement ? Number(requirement.value) : undefined,
      file,
    )
  }, [canUploadAttachments, draft, environment.client, options])
  const submit = useCallback(async () => {
    if (!draft || scope.status !== 'ready') return false
    if (draft.attachments.length && !canUploadAttachments) return false
    const directory = users.status === 'ready' ? users.data : []
    const issue = validateCreateDraft({
      workspaceId: environment.workspaceId,
      draft,
      scope: scope.data,
      users: directory,
      clientOptions: clients.rows,
      statuses,
    })
    if (issue) {
      setValidation(issue)
      return false
    }
    const submittedTicket = liveTicket.current
    try {
      const item = await mutation.mutateAsync(createPayload(draft, environment.userId))
      if (submittedTicket !== liveTicket.current) return false
      const attachments = draft.attachments
      if (attachments.length) {
        const result = await attachmentMutation.mutateAsync({ taskId: item.id, files: attachments })
        if (submittedTicket !== liveTicket.current) return false
        setNotice(result.failed.length ? 'attachment-error' : undefined)
      }
      setDraft(null)
      setValidation(undefined)
      await environment.navigate(workItemsPath({
        ...route,
        detailId: undefined,
        mode: 'view',
        invalidLegacyScope: false,
        needsRouteNormalization: false,
      }, { detailId: item.id }))
      return true
    } catch {
      return false
    }
  }, [attachmentMutation.mutateAsync, canUploadAttachments, clients.rows, draft, environment, mutation.mutateAsync, options, route, scope, statuses, users])

  const state: WorkItemCreateState = draft
    ? {
        status: 'editing',
        draft,
        submit: mutation.isPending || attachmentMutation.isPending
          ? 'pending'
          : mutation.isError ? 'error' : 'idle',
        ...(validation ? { validation } : {}),
      }
    : { status: 'closed' }

  return {
    state,
    canOpen,
    unavailable: environment.createAllowed ? 'scope' as const : 'workspace' as const,
    scopeOptions: scope.status === 'ready' ? scope.data : undefined,
    requirements: options,
    users: users.status === 'ready' ? users.data : [],
    usersState: users.status,
    statuses,
    notice,
    open,
    close,
    update,
    canAssignToMe,
    canUploadAttachments,
    assignToMe,
    addAttachment,
    removeAttachment,
    uploadRichTextImage,
    submit,
  }
}
