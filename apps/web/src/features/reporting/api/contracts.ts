export type ReportingRaw = unknown

// Facts context for the runId=0 facts-only Feishu export; the backend
// re-aggregates from it inside the workspace scope.
export type ReportingFeishuFactsContext = {
  targetKind: 'member' | 'group' | 'department'
  targetId: number
  type: string
  anchorDate: string
}

export type ReportingClient = {
  directory(signal?: AbortSignal): Promise<ReportingRaw>
  memberPreview(
    memberId: number,
    query: { type: string; date?: string },
    signal?: AbortSignal,
  ): Promise<ReportingRaw>
  memberGenerate(memberId: number, body: { type: string; date?: string }): Promise<ReportingRaw>
  groupPreview(
    groupId: number,
    query: { type: string; date?: string },
    signal?: AbortSignal,
  ): Promise<ReportingRaw>
  groupGenerate(groupId: number, body: { type: string; date?: string }): Promise<ReportingRaw>
  departmentPreview(
    departmentId: number,
    query: { type: string; date?: string },
    signal?: AbortSignal,
  ): Promise<ReportingRaw>
  departmentGenerate(departmentId: number, body: { type: string; date?: string }): Promise<ReportingRaw>
  runStatus(runId: number, signal?: AbortSignal): Promise<ReportingRaw>
  exportFeishuDoc(runId: number, facts?: ReportingFeishuFactsContext): Promise<ReportingRaw>
}
