import { toSafeApiError, PMSafeApiError, unwrapApiData } from '@/shared/api'

export const aiPmKeys = {
  requirements: (workspaceId: number, filters: Record<string, unknown>) => ['ai-pm', workspaceId, 'requirements', filters] as const,
  requirement: (workspaceId: number, id: number) => ['ai-pm', workspaceId, 'requirement', id] as const,
  requirementScope: (workspaceId: number) => ['ai-pm', workspaceId, 'requirement-scope'] as const,
  capabilities: (workspaceId: number) => ['ai-pm', workspaceId, 'capabilities'] as const,
  runs: (workspaceId: number, subjectType?: string, subjectId?: number) => ['ai-pm', workspaceId, 'runs', subjectType ?? 'all', subjectId ?? 'all'] as const,
  run: (workspaceId: number, id: number) => ['ai-pm', workspaceId, 'run', id] as const,
  workflows: (workspaceId: number) => ['ai-pm', workspaceId, 'workflows'] as const,
  requirementWorkflowNodes: (workspaceId: number) => ['ai-pm', workspaceId, 'requirement-workflow-nodes'] as const,
  workflow: (workspaceId: number, id: number) => ['ai-pm', workspaceId, 'workflow', id] as const,
  workflowInstance: (workspaceId: number, id: number) => ['ai-pm', workspaceId, 'workflow-instance', id] as const,
  workflowInstances: (workspaceId: number, subjectType: string, subjectId: number) => ['ai-pm', workspaceId, 'workflow-instances', subjectType, subjectId] as const,
  workflowAssignees: (workspaceId: number) => ['ai-pm', workspaceId, 'workflow-assignees'] as const,
  testCaseArtifacts: (workspaceId: number, instanceId: number, nodeId: string) => ['ai-pm', workspaceId, 'workflow-instance', instanceId, 'test-case-artifacts', nodeId] as const,
  testReportArtifacts: (workspaceId: number, instanceId: number, nodeId: string) => ['ai-pm', workspaceId, 'workflow-instance', instanceId, 'test-report-artifacts', nodeId] as const,
  testingLibrary: (workspaceId: number, filters: Record<string, unknown>) => ['ai-pm', workspaceId, 'testing-library', filters] as const,
  requirementMaterials: (workspaceId: number, requirementId: number, kind?: string) => ['ai-pm', workspaceId, 'requirement-materials', requirementId, kind ?? 'all'] as const,
  nodeOpinions: (workspaceId: number, instanceId: number, nodeId: string) => ['ai-pm', workspaceId, 'workflow-instance', instanceId, 'node-opinions', nodeId] as const,
  projects: (workspaceId: number) => ['ai-pm', workspaceId, 'projects'] as const,
  units: (workspaceId: number, projectId: number) => ['ai-pm', workspaceId, 'units', projectId] as const,
}

export async function safeApiCall<T>(work: () => Promise<T>, code: string): Promise<T> {
  try { return await work() } catch (error) { throw toSafeApiError(error, code) }
}

export function requiredApiData<T>(response: unknown): T {
  const value = unwrapApiData<T>(response)
  if (value === undefined) throw new Error('PM API returned no data')
  return value
}

export function requireApiOperation(allowed: boolean): void {
  if (!allowed) throw new PMSafeApiError('ACTION_PERMISSION_DENIED', 403)
}

export function requirementPatchAssignsOwners(body: Record<string, unknown>): boolean {
  return Object.keys(body).some((key) => key.endsWith('_owner_user_id') || key.endsWith('_owner_user_ids') || key === 'tester_user_ids' || key === 'developer_user_ids')
}
