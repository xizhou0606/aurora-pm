import { useCallback } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { pmKeys } from '@/shared/query'
import {
  isSafeApiError,
  PM_API_ERROR_CODES,
  platformApi,
  toSafeApiError,
  unwrapApiData,
} from '@/shared/api'
import { PMSafeApiError } from '@/shared/api'
import { OPERATION_FEEDBACK_COPY } from '@/shared/copy'
import { operationFeedback } from '@/shared/ui'
import { useRequirementsOperationAccess, useTestingOperationAccess, useWorkflowOperationAccess } from '../hooks/useOperationAccess'
import type { Requirement, WorkflowDefinition, WorkflowInstance, WorkflowNodeArtifact, WorkflowNodeBatchUpdate, WorkflowNodeOpinion, WorkflowTemplate, WorkflowValidation } from '../model/types'
import { keys as workflowKeys } from './workflow-queries'

async function safe<T>(work: () => Promise<T>, code: string): Promise<T> {
  try { return await work() } catch (error) { throw toSafeApiError(error, code) }
}

function requiredData<T>(response: unknown): T {
  const value = unwrapApiData<T>(response)
  if (value === undefined) throw new Error('PM API returned no data')
  return value
}

function requireOperation(allowed: boolean): void {
  if (!allowed) throw new PMSafeApiError('ACTION_PERMISSION_DENIED', 403)
}

export function useWorkflowMutations(workspaceId: number, options?: { actOverride?: boolean }) {
  const client = useQueryClient()
  const workflowAccess = useWorkflowOperationAccess()
  const requirementAccess = useRequirementsOperationAccess()
  const testingAccess = useTestingOperationAccess()
  // Object-policy mirror: the requirement creator may act on their own
  // requirement's workflow without the global act grant (backend enforces it).
  const canActOnInstance = useCallback(
    (local = true) => workflowAccess.canActOnInstance(local) || options?.actOverride === true,
    [workflowAccess, options?.actOverride],
  )
  const invalidate = () => Promise.all([
    client.invalidateQueries({ queryKey: ['ai-pm', workspaceId] }),
    client.invalidateQueries({ queryKey: pmKeys.workspace(workspaceId) }),
  ])
  return {
    create: useMutation({ meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.create), mutationFn: (body: { name: string; subject_type: string; description: string; definition: WorkflowDefinition }) => safe(async () => { requireOperation(workflowAccess.canCreateTemplate()); return requiredData<WorkflowTemplate>(await platformApi.post('/pm/workflows/templates', body)) }, 'WORKFLOW_CREATE_FAILED'), onSuccess: () => client.invalidateQueries({ queryKey: workflowKeys.workflows(workspaceId) }) }),
    save: useMutation({ meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.save), mutationFn: ({ id, definition, name, description }: { id: number; definition: WorkflowDefinition; name: string; description: string }) => safe(async () => { requireOperation(workflowAccess.canUpdateTemplate()); return requiredData<WorkflowTemplate>(await platformApi.patch(`/pm/workflows/templates/${id}`, { definition, name, description })) }, 'WORKFLOW_SAVE_FAILED'), onSuccess: (data) => { client.setQueryData(workflowKeys.workflow(workspaceId, data.id), data); void invalidate() } }),
    validate: useMutation({ meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.validate), mutationFn: (id: number) => safe(async () => { requireOperation(workflowAccess.canValidateTemplate()); return requiredData<WorkflowValidation>(await platformApi.post(`/pm/workflows/templates/${id}/validate`)) }, 'WORKFLOW_VALIDATE_FAILED') }),
    validateDefinition: useMutation({ meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.validate), mutationFn: ({ id, definition }: { id: number; definition: WorkflowDefinition }) => safe(async () => { requireOperation(workflowAccess.canValidateTemplate()); return requiredData<WorkflowValidation>(await platformApi.post(`/pm/workflows/templates/${id}/validate-definition`, { definition })) }, 'WORKFLOW_VALIDATE_FAILED') }),
    publish: useMutation({ meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.publish), mutationFn: (id: number) => safe(async () => { requireOperation(workflowAccess.canPublishTemplate()); return requiredData<{ template_id: number; version: number; definition_hash: string }>(await platformApi.post(`/pm/workflows/templates/${id}/publish`)) }, 'WORKFLOW_PUBLISH_FAILED'), onSuccess: invalidate }),
    planNodes: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.saveNodePlan),
      mutationFn: ({ id, nodes, confirm_subject_schedule }: { id: number; nodes: WorkflowNodeBatchUpdate[]; confirm_subject_schedule?: true }) => safe(async () => { requireOperation(workflowAccess.canUpdateNodePlans()); return requiredData<WorkflowInstance | { node_plans: NonNullable<WorkflowInstance['node_plans']> }>(await platformApi.patch(`/pm/workflows/instances/${id}/node-plans`, { nodes, ...(confirm_subject_schedule ? { confirm_subject_schedule } : {}) })) }, 'WORKFLOW_NODE_PLAN_FAILED'),
      onSuccess: (data, { id }) => {
        const key = workflowKeys.workflowInstance(workspaceId, id)
        client.setQueryData<WorkflowInstance>(key, (current) => {
          if ('timeline_nodes' in data) return data
          if (!current) return current
          return { ...current, node_plans: data.node_plans, timeline_nodes: current.timeline_nodes.map((node) => ({
            ...node,
            planned_start_at: data.node_plans[node.id]?.planned_start_at ?? null,
            planned_end_at: data.node_plans[node.id]?.planned_end_at ?? null,
          })) }
        })
        void client.invalidateQueries({ queryKey: pmKeys.workspace(workspaceId) })
      },
      onError: async (error, { id }) => {
        if (!isSafeApiError(error) || error.code !== PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE) return
        await Promise.all([
          client.refetchQueries({ queryKey: workflowKeys.workflowInstance(workspaceId, id), type: 'active' }),
          client.invalidateQueries({ queryKey: pmKeys.workspace(workspaceId) }),
        ])
      },
    }),
    uploadDesignImage: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.uploadDesign),
      mutationFn: ({ requirementId, file }: { requirementId: number; file: File }) => safe(async () => {
        requireOperation(requirementAccess.canUploadDesignAsset())
        const formData = new FormData()
        formData.append('file', file)
        return requiredData<{ url: string; name: string; content_type: string; size: number }>(await platformApi.post(`/requirements/${requirementId}/design-assets/upload`, formData))
      }, 'WORKFLOW_DESIGN_UPLOAD_FAILED'),
    }),
    uploadTestCaseArtifact: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.uploadTestCase),
      mutationFn: ({ instanceId, nodeId, file, url }: { instanceId: number; nodeId: string; file?: File; url?: string }) => safe(async () => {
        requireOperation(workflowAccess.canUploadTestCaseMaterial())
        const formData = new FormData()
        if (file) formData.append('file', file)
        if (url) formData.append('url', url)
        return requiredData<WorkflowNodeArtifact>(await platformApi.post(`/pm/workflows/instances/${instanceId}/nodes/${encodeURIComponent(nodeId)}/test-case-artifacts`, formData))
      }, 'WORKFLOW_TEST_CASE_UPLOAD_FAILED'),
      onSuccess: (artifact) => Promise.all([
        client.invalidateQueries({ queryKey: workflowKeys.testCaseArtifacts(workspaceId, artifact.instance_id, artifact.node_id) }),
        client.invalidateQueries({ queryKey: ['ai-pm', workspaceId, 'testing-library'] }),
      ]),
    }),
    uploadTestReportArtifact: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.uploadTestReport),
      mutationFn: ({ instanceId, nodeId, file }: { instanceId: number; nodeId: string; file: File }) => safe(async () => {
        requireOperation(testingAccess.canUploadVersionReport())
        const formData = new FormData()
        formData.append('file', file)
        return requiredData<WorkflowNodeArtifact>(await platformApi.post(`/pm/workflows/instances/${instanceId}/nodes/${encodeURIComponent(nodeId)}/test-report-artifacts`, formData))
      }, 'WORKFLOW_TEST_REPORT_UPLOAD_FAILED'),
      onSuccess: (artifact) => Promise.all([
        client.invalidateQueries({ queryKey: workflowKeys.testReportArtifacts(workspaceId, artifact.instance_id, artifact.node_id) }),
        client.invalidateQueries({ queryKey: ['ai-pm', workspaceId, 'testing-library'] }),
      ]),
    }),
    saveTestReportLink: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.saveTestReportLink),
      mutationFn: ({ instanceId, nodeId, url }: { instanceId: number; nodeId: string; url: string }) => safe(async () => { requireOperation(testingAccess.canLinkVersionReport()); return requiredData<WorkflowNodeArtifact>(await platformApi.post(`/pm/workflows/instances/${instanceId}/nodes/${encodeURIComponent(nodeId)}/test-report-artifacts/link`, { url })) }, 'WORKFLOW_TEST_REPORT_LINK_FAILED'),
      onSuccess: (artifact) => Promise.all([
        client.invalidateQueries({ queryKey: workflowKeys.testReportArtifacts(workspaceId, artifact.instance_id, artifact.node_id) }),
        client.invalidateQueries({ queryKey: ['ai-pm', workspaceId, 'testing-library'] }),
      ]),
    }),
    materializeTestCaseArtifact: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.uploadTestCase),
      mutationFn: ({ instanceId, nodeId, aiRunId }: { instanceId: number; nodeId: string; aiRunId: number }) => safe(async () => {
        requireOperation(workflowAccess.canGenerateTestCaseMaterial())
        const path = `/pm/workflows/instances/${instanceId}/nodes/${encodeURIComponent(nodeId)}/test-case-artifacts/ai-generated`
        const retryDelays = [500, 1_000, 2_000]
        for (let attempt = 0; ; attempt += 1) {
          try {
            return requiredData<WorkflowNodeArtifact>(await platformApi.post(path, { ai_run_id: aiRunId }))
          } catch (error) {
            const safeError = toSafeApiError(error, 'WORKFLOW_TEST_CASE_DELIVERY_FAILED')
            const retryable = !safeError.aborted && (
              safeError.status === undefined
              || [408, 425, 429, 500, 502, 503, 504].includes(safeError.status)
            )
            if (!retryable || attempt >= retryDelays.length) throw safeError
            await new Promise((resolve) => window.setTimeout(resolve, retryDelays[attempt]))
          }
        }
      }, 'WORKFLOW_TEST_CASE_DELIVERY_FAILED'),
      onSuccess: (artifact) => Promise.all([
        client.invalidateQueries({ queryKey: workflowKeys.testCaseArtifacts(workspaceId, artifact.instance_id, artifact.node_id) }),
        client.invalidateQueries({ queryKey: ['ai-pm', workspaceId, 'runs'] }),
      ]),
    }),
    createNodeOpinion: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.opinion),
      mutationFn: ({ instanceId, nodeId, body, invite_user_ids }: { instanceId: number; nodeId: string; body: string; invite_user_ids?: number[] }) => safe(async () => { requireOperation(workflowAccess.canCreateOpinion()); return requiredData<WorkflowNodeOpinion>(await platformApi.post(`/pm/workflows/instances/${instanceId}/nodes/${encodeURIComponent(nodeId)}/opinions`, { body, invite_user_ids })) }, 'WORKFLOW_NODE_OPINION_FAILED'),
      onSuccess: (opinion) => client.invalidateQueries({ queryKey: workflowKeys.nodeOpinions(workspaceId, opinion.instance_id, opinion.node_id) }),
    }),
    invitePreReview: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.opinionInvite),
      mutationFn: ({ instanceId, nodeId, invite_user_ids }: { instanceId: number; nodeId: string; invite_user_ids: number[] }) => safe(async () => { requireOperation(workflowAccess.canCreateOpinion()); return requiredData<{ invited_user_ids: number[] }>(await platformApi.post(`/pm/workflows/instances/${instanceId}/nodes/${encodeURIComponent(nodeId)}/opinions`, { invite_user_ids })) }, 'WORKFLOW_NODE_OPINION_INVITE_FAILED'),
    }),
    updateNodeOpinion: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.opinion),
      mutationFn: ({ instanceId, nodeId, opinionId, body }: { instanceId: number; nodeId: string; opinionId: number; body: string }) => safe(async () => { requireOperation(workflowAccess.canUpdateOpinion()); return requiredData<WorkflowNodeOpinion>(await platformApi.patch(`/pm/workflows/instances/${instanceId}/nodes/${encodeURIComponent(nodeId)}/opinions/${opinionId}`, { body })) }, 'WORKFLOW_NODE_OPINION_FAILED'),
      onSuccess: (opinion) => client.invalidateQueries({ queryKey: workflowKeys.nodeOpinions(workspaceId, opinion.instance_id, opinion.node_id) }),
    }),
    uploadOpinionImage: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.uploadOpinionImage),
      mutationFn: ({ instanceId, nodeId, file }: { instanceId: number; nodeId: string; file: File }) => safe(async () => {
        requireOperation(workflowAccess.canUploadOpinionImage())
        const formData = new FormData()
        formData.append('file', file)
        return requiredData<{ url: string; name: string; content_type: string; size: number }>(
          await platformApi.post(`/pm/workflows/instances/${instanceId}/nodes/${encodeURIComponent(nodeId)}/opinion-images`, formData),
        )
      }, 'WORKFLOW_OPINION_IMAGE_UPLOAD_FAILED'),
    }),
    action: useMutation({
      meta: operationFeedback(OPERATION_FEEDBACK_COPY.workflow.action),
      mutationFn: ({ id, action, node_id, target_node_id, comment, design_url, skip_task_plan, confirm_requirement_lock, next_node_assignees }: { id: number; action: 'advance' | 'complete' | 'approve' | 'reject' | 'resume' | 'rollback' | 'supplement_design' | 'retry_ai' | 'recheck_ai' | 'override_ai' | 'condition_yes' | 'condition_no'; node_id?: string; target_node_id?: string; comment?: string; design_url?: string; skip_task_plan?: boolean; confirm_requirement_lock?: boolean; next_node_assignees?: Array<{ node_id: string; assignee_user_id: number }> }) => safe(async () => { requireOperation(canActOnInstance()); return requiredData<WorkflowInstance>(await platformApi.post(`/pm/workflows/instances/${id}/actions`, { action, node_id, target_node_id, comment, design_url, skip_task_plan, confirm_requirement_lock, next_node_assignees })) }, 'WORKFLOW_ACTION_FAILED'),
      onSuccess: (data, { id }) => {
        client.setQueryData(workflowKeys.workflowInstance(workspaceId, id), data)
        void invalidate()
      },
      onError: (_error, { id }) => client.invalidateQueries({ queryKey: workflowKeys.workflowInstance(workspaceId, id) }),
    }),
  }
}