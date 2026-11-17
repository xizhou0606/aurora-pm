import { useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { platformApi, toSafeApiError, unwrapApiData, unwrapApiRows } from '@/shared/api'
import { usePlatformUsersQuery } from '@/shared/query'
import { aipmClient } from '../api/client'
import { useAIPMClient } from '../components/environment'
import { requirementPoolWorkflowNodeOptions } from '../utils/requirement-pool-workflow'
import { useTaskTableViewAccess, useTestingOperationAccess, useWorkflowOperationAccess } from '../hooks/useOperationAccess'
import type { TestingLibraryFilters, TestingLibraryItem, WorkflowAssignee, WorkflowInstance, WorkflowNodeArtifact, WorkflowNodeOpinion, WorkflowTemplate } from '../model/types'

export type RequirementTaskRow = {
  id: number
  taskKey: string
  title: string
  status: string
}

export const keys = {
  workflows: (workspaceId: number) => ['ai-pm', workspaceId, 'workflows'] as const,
  requirementWorkflowNodes: (workspaceId: number) => ['ai-pm', workspaceId, 'requirement-workflow-nodes'] as const,
  requirementTemplateVersions: (workspaceId: number) => ['ai-pm', workspaceId, 'requirement-template-versions'] as const,
  workflow: (workspaceId: number, id: number) => ['ai-pm', workspaceId, 'workflow', id] as const,
  workflowInstance: (workspaceId: number, id: number) => ['ai-pm', workspaceId, 'workflow-instance', id] as const,
  workflowInstances: (workspaceId: number, subjectType: string, subjectId: number) => ['ai-pm', workspaceId, 'workflow-instances', subjectType, subjectId] as const,
  workflowAssignees: (workspaceId: number) => ['ai-pm', workspaceId, 'workflow-assignees'] as const,
  testCaseArtifacts: (workspaceId: number, instanceId: number, nodeId: string) => ['ai-pm', workspaceId, 'workflow-instance', instanceId, 'test-case-artifacts', nodeId] as const,
  testReportArtifacts: (workspaceId: number, instanceId: number, nodeId: string) => ['ai-pm', workspaceId, 'workflow-instance', instanceId, 'test-report-artifacts', nodeId] as const,
  testingLibrary: (workspaceId: number, filters: Record<string, unknown>) => ['ai-pm', workspaceId, 'testing-library', filters] as const,
  nodeOpinions: (workspaceId: number, instanceId: number, nodeId: string) => ['ai-pm', workspaceId, 'workflow-instance', instanceId, 'node-opinions', nodeId] as const,
  requirementTasks: (workspaceId: number, requirementId: number) => ['ai-pm', workspaceId, 'requirement-tasks', requirementId] as const,
}

async function safe<T>(work: () => Promise<T>, code: string): Promise<T> {
  try { return await work() } catch (error) { throw toSafeApiError(error, code) }
}

function requiredData<T>(response: unknown): T {
  const value = unwrapApiData<T>(response)
  if (value === undefined) throw new Error('PM API returned no data')
  return value
}

export function useWorkflows(workspaceId: number) {
  const access = useWorkflowOperationAccess()
  return useQuery({ queryKey: keys.workflows(workspaceId), enabled: workspaceId > 0 && access.canViewTemplates(), queryFn: ({ signal }) => safe(async () => unwrapApiRows<WorkflowTemplate>(await platformApi.get('/pm/workflows/templates', { signal })), 'WORKFLOWS_FAILED') })
}

export function useRequirementWorkflowFilterNodes(workspaceId: number) {
  const access = useWorkflowOperationAccess()
  return useQuery({
    queryKey: keys.requirementWorkflowNodes(workspaceId),
    enabled: workspaceId > 0 && access.canViewTemplates(),
    staleTime: 30_000,
    queryFn: ({ signal }) => safe(async () => {
      const rows = unwrapApiRows<WorkflowTemplate>(await platformApi.get('/pm/workflows/templates', {
        params: { subject_type: 'requirement', include_nodes: '1' },
        signal,
      }))
      const fromList = requirementPoolWorkflowNodeOptions(rows)
      if (fromList.length > 0) return fromList
      const published = rows.filter((row) => row.subject_type === 'requirement' && row.published_version)
      if (published.length === 0) return fromList
      const details = await Promise.all(published.map(async (row) => requiredData<WorkflowTemplate>(
        await platformApi.get(`/pm/workflows/templates/${row.id}`, { signal }),
      )))
      return requirementPoolWorkflowNodeOptions(details)
    }, 'WORKFLOWS_FAILED'),
  })
}

export type RequirementTemplateVersionOption = {
  versionId: number
  templateId: number
  label: string
  nodes: Array<{ id: string; name: string; type: string }>
}

// 已发布需求模板版本目录：requirements_guard 绑定使用不可变的
// workflow_template_versions.id 作为映射键，目标节点来自该版本的已发布定义。
export function useRequirementTemplateVersions(workspaceId: number) {
  const access = useWorkflowOperationAccess()
  return useQuery({
    queryKey: keys.requirementTemplateVersions(workspaceId),
    enabled: workspaceId > 0 && access.canViewTemplates(),
    staleTime: 30_000,
    queryFn: ({ signal }) => safe(async () => {
      const rows = unwrapApiRows<WorkflowTemplate>(await platformApi.get('/pm/workflows/templates', {
        params: { subject_type: 'requirement', include_nodes: '1' },
        signal,
      }))
      return rows
        .filter((row) => row.subject_type === 'requirement' && row.published_version > 0 && row.published_version_id)
        .map((row) => ({
          versionId: row.published_version_id as number,
          templateId: row.id,
          label: `${row.name} · v${row.published_version}`,
          nodes: (row.nodes ?? []).filter((node) => node.type !== 'start' && node.type !== 'end'),
        }))
    }, 'WORKFLOWS_FAILED'),
  })
}

export type PlanUnitIntakeRequirementRow = {
  id: number
  title: string
  status: string
  priority: string
}

// 项目「需求录入」节点的已录入需求清单：当前版本下的全部需求（锁版分母）。
export function usePlanUnitIntakeRequirements(workspaceId: number, planUnitId: number, enabled = true) {
  return useQuery({
    queryKey: ['ai-pm', workspaceId, 'plan-unit-intake-requirements', planUnitId] as const,
    enabled: enabled && workspaceId > 0 && planUnitId > 0,
    staleTime: 15_000,
    queryFn: ({ signal }) => safe(async () => {
      const rows = unwrapApiRows<Record<string, unknown>>(await platformApi.get('/requirements', {
        params: { version_task_new_id: planUnitId, limit: 100 },
        signal,
      }))
      return rows
        .map((row) => ({
          id: Number(row.id) || 0,
          title: String(row.title ?? ''),
          status: String(row.status ?? ''),
          priority: String(row.priority ?? ''),
        }))
        .filter((row) => row.id > 0 && row.title)
    }, 'REQUIREMENTS_FAILED'),
  })
}

export function useWorkflow(workspaceId: number, id: number) {
  const api = useAIPMClient()
  const access = useWorkflowOperationAccess()
  return useQuery<WorkflowTemplate>({ queryKey: keys.workflow(workspaceId, id), enabled: workspaceId > 0 && id > 0 && access.canViewTemplates(), queryFn: ({ signal }) => api.workflowTemplate(id, signal) })
}

// Reuse the shared workspace user snapshot (one directory fetch per session,
// filtered locally in the selects — same pattern as the project-management
// owner pickers and quality-management user pickers); typing never issues a
// request. Data shape matches WorkflowAssignee structurally.
export function useWorkflowAssignees(workspaceId: number) {
  const query = usePlatformUsersQuery(workspaceId)
  return {
    ...query,
    data: (query.data ?? []) as WorkflowAssignee[],
  }
}

export function prefetchWorkflowInstance(client: QueryClient, workspaceId: number, id: number) {
  if (workspaceId <= 0 || id <= 0) return
  void client.prefetchQuery({
    queryKey: keys.workflowInstance(workspaceId, id),
    queryFn: ({ signal }) => aipmClient.workflowInstance(id, signal),
  })
}

export function useWorkflowInstance(workspaceId: number, id: number) {
  const api = useAIPMClient()
  const access = useWorkflowOperationAccess()
  return useQuery<WorkflowInstance>({
    queryKey: keys.workflowInstance(workspaceId, id),
    enabled: workspaceId > 0 && id > 0 && access.canViewInstances(),
    refetchInterval: (query) => query.state.data?.timeline_nodes?.some((node) => ['queued', 'running'].includes(node.ai_execution?.status ?? '')) ? 1500 : false,
    queryFn: ({ signal }) => api.workflowInstance(id, signal),
  })
}

export function useWorkflowInstances(workspaceId: number, subjectType: 'requirement' | 'plan_unit', subjectId: number) {
  const api = useAIPMClient()
  const access = useWorkflowOperationAccess()
  return useQuery<WorkflowInstance[]>({ queryKey: keys.workflowInstances(workspaceId, subjectType, subjectId), enabled: workspaceId > 0 && subjectId > 0 && access.canViewInstances(), queryFn: ({ signal }) => api.workflowInstances(subjectType, subjectId, signal) })
}

export function useWorkflowTestCaseArtifacts(workspaceId: number, instanceId: number, nodeId: string, enabled = true) {
  const api = useAIPMClient()
  const access = useWorkflowOperationAccess()
  return useQuery<WorkflowNodeArtifact[]>({
    queryKey: keys.testCaseArtifacts(workspaceId, instanceId, nodeId),
    enabled: enabled && workspaceId > 0 && instanceId > 0 && Boolean(nodeId) && access.canViewTestCaseMaterials(),
    queryFn: ({ signal }) => api.testCaseArtifacts(instanceId, nodeId, signal),
  })
}

export function useWorkflowTestReportArtifacts(workspaceId: number, instanceId: number, nodeId: string, enabled = true) {
  const api = useAIPMClient()
  const access = useTestingOperationAccess()
  return useQuery<WorkflowNodeArtifact[]>({
    queryKey: keys.testReportArtifacts(workspaceId, instanceId, nodeId),
    enabled: enabled && workspaceId > 0 && instanceId > 0 && Boolean(nodeId) && access.canViewVersionReports(),
    queryFn: ({ signal }) => api.testReportArtifacts(instanceId, nodeId, signal),
  })
}

export function useTestingLibrary(workspaceId: number, filters: TestingLibraryFilters) {
  const access = useTestingOperationAccess()
  const canView = filters.kind === 'test_report'
    ? access.canViewVersionReports()
    : access.canViewLibrary()
  const endpoint = filters.kind === 'test_report'
    ? '/pm/testing/version-reports'
    : '/pm/testing/items'
  return useQuery<TestingLibraryItem[]>({
    queryKey: keys.testingLibrary(workspaceId, filters),
    enabled: workspaceId > 0 && canView,
    queryFn: ({ signal }) => safe(async () => unwrapApiRows<TestingLibraryItem>(await platformApi.get(endpoint, {
      params: {
        kind: filters.kind,
        q: filters.q || undefined,
        project_id: filters.projectId || undefined,
        plan_unit_id: filters.planUnitId || undefined,
      },
      signal,
    })), 'TESTING_LIBRARY_FAILED'),
  })
}

export function useWorkflowNodeOpinions(workspaceId: number, instanceId: number, nodeId: string, enabled = true) {
  const api = useAIPMClient()
  const access = useWorkflowOperationAccess()
  return useQuery<WorkflowNodeOpinion[]>({
    queryKey: keys.nodeOpinions(workspaceId, instanceId, nodeId),
    enabled: enabled && workspaceId > 0 && instanceId > 0 && Boolean(nodeId) && access.canViewOpinions(),
    queryFn: ({ signal }) => api.nodeOpinions(instanceId, nodeId, signal),
  })
}

export function useRequirementTasks(workspaceId: number, requirementId: number, enabled = true) {
  const access = useTaskTableViewAccess()
  return useQuery<RequirementTaskRow[]>({
    queryKey: keys.requirementTasks(workspaceId, requirementId),
    enabled: enabled && workspaceId > 0 && requirementId > 0 && access.canViewTaskTable(),
    staleTime: 30_000,
    queryFn: ({ signal }) => safe(async () => {
      const rows = unwrapApiRows<Record<string, unknown>>(await platformApi.get('/tasks', {
        params: { requirement_ids: requirementId, limit: 100 },
        signal,
      }))
      return rows
        .map((row) => ({
          id: Number(row.id) || 0,
          taskKey: String(row.task_key ?? ''),
          title: String(row.title ?? ''),
          status: String(row.status ?? ''),
        }))
        .filter((row) => row.id > 0 && row.title)
    }, 'REQUIREMENT_TASKS_FAILED'),
  })
}
