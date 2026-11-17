import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useMemo, useRef } from 'react'

import { nativeModuleKeys } from '@/shared/query'

import {
  normalizeDirectory,
  normalizePreview,
  normalizeRun,
} from '../api/normalizers'
import type { ReportingDirectory, ReportPreview, ReportingRun, ReportTargetKind } from '../model/types'
import type { ReportingEnvironment } from '../components/environment'

const DIRECTORY_STALE_TIME = 30_000
const RUN_POLL_FAST_INTERVAL = 1_000
const RUN_POLL_INTERVAL = 2_500
const RUN_POLL_SLOW_INTERVAL = 5_000
const RUN_FAST_POLL_WINDOW_MS = 15_000
const RUN_MEDIUM_POLL_WINDOW_MS = 60_000
const RUN_POLL_JITTER_MS = 300
const RUN_TERMINAL_STATUSES = ['completed', 'failed']

// Query keys require a positive workspace ID; disabled queries get a stable
// placeholder key that never reads the cache.
function reportingQueryKey(
  workspaceId: number,
  ...scope: readonly unknown[]
): readonly unknown[] {
  return workspaceId > 0
    ? nativeModuleKeys.resource(workspaceId, 'reporting', ...scope)
    : ['pm', 'reporting', 'disabled', ...scope]
}

export type ReportingListState<TData> =
  | { status: 'loading' }
  | { status: 'unavailable' }
  | { status: 'error'; retry: () => void }
  | { status: 'ready'; data: TData }

function toQueryState<TData>(
  environment: ReportingEnvironment,
  enabled: boolean,
  query: { isPending: boolean; isError: boolean; refetch: () => unknown; data: TData | undefined },
): ReportingListState<TData> {
  if (!enabled || !environment.allowed || environment.workspaceId <= 0) {
    return { status: 'unavailable' }
  }
  if (query.isPending) return { status: 'loading' }
  if (query.isError) return { status: 'error', retry: () => { void query.refetch() } }
  return { status: 'ready', data: query.data as TData }
}

export function useReportingDirectoryQuery(
  environment: ReportingEnvironment,
  enabled = true,
): ReportingListState<ReportingDirectory> {
  const query = useQuery({
    queryKey: reportingQueryKey(environment.workspaceId, 'directory'),
    queryFn: async ({ signal }) => normalizeDirectory(await environment.client.directory(signal)),
    enabled: enabled && environment.allowed && environment.workspaceId > 0,
    staleTime: DIRECTORY_STALE_TIME,
    placeholderData: keepPreviousData,
    retry: false,
  })
  return useMemo(
    () => toQueryState(environment, enabled, query),
    [enabled, environment, query.data, query.isError, query.isPending, query.refetch],
  )
}

function useTargetReportPreviewQuery(
  environment: ReportingEnvironment,
  scope: ReportTargetKind,
  targetId: number,
  period: { type: string; anchorDate: string },
  enabled = true,
): ReportingListState<ReportPreview> {
  const enabledQuery = enabled && targetId > 0
  const query = useQuery({
    queryKey: reportingQueryKey(environment.workspaceId, 'preview', scope, targetId, period.type, period.anchorDate),
    queryFn: async ({ signal }) => {
      const query = { type: period.type, date: period.anchorDate || undefined }
      if (scope === 'group') return normalizePreview(await environment.client.groupPreview(targetId, query, signal))
      if (scope === 'department') return normalizePreview(await environment.client.departmentPreview(targetId, query, signal))
      return normalizePreview(await environment.client.memberPreview(targetId, query, signal))
    },
    enabled: enabledQuery && environment.allowed && environment.workspaceId > 0,
    staleTime: DIRECTORY_STALE_TIME,
    placeholderData: keepPreviousData,
    retry: false,
  })
  return useMemo(
    () => toQueryState(environment, enabledQuery, query),
    [enabledQuery, environment, query.data, query.isError, query.isPending, query.refetch],
  )
}

export function useMemberReportPreviewQuery(
  environment: ReportingEnvironment,
  memberId: number,
  period: { type: string; anchorDate: string },
  enabled = true,
): ReportingListState<ReportPreview> {
  return useTargetReportPreviewQuery(environment, 'member', memberId, period, enabled)
}

export function useGroupReportPreviewQuery(
  environment: ReportingEnvironment,
  groupId: number,
  period: { type: string; anchorDate: string },
  enabled = true,
): ReportingListState<ReportPreview> {
  return useTargetReportPreviewQuery(environment, 'group', groupId, period, enabled)
}

export function useDepartmentReportPreviewQuery(
  environment: ReportingEnvironment,
  departmentId: number,
  period: { type: string; anchorDate: string },
  enabled = true,
): ReportingListState<ReportPreview> {
  return useTargetReportPreviewQuery(environment, 'department', departmentId, period, enabled)
}

// The generated narrative arrives asynchronously through the AI run pipeline;
// polling stops as soon as the run reaches a terminal status.
export function useReportRunQuery(
  environment: ReportingEnvironment,
  runId: number,
  startedAtEpoch: number,
  enabled = true,
): ReportingListState<ReportingRun> {
  const enabledQuery = enabled && runId > 0
  // The backoff anchor is the submission time — anchoring on the last fetch
  // time resets after every success and would poll at 1s forever.
  const startedAtRef = useRef(startedAtEpoch)
  startedAtRef.current = startedAtEpoch || Date.now()
  const query = useQuery({
    queryKey: reportingQueryKey(environment.workspaceId, 'run', runId),
    queryFn: async ({ signal }) => normalizeRun(await environment.client.runStatus(runId, signal)),
    enabled: enabledQuery && environment.allowed && environment.workspaceId > 0,
    retry: false,
    refetchInterval: (current) => {
      const status = current.state.data?.status
      if (!status || RUN_TERMINAL_STATUSES.includes(status)) return false
      const elapsed = Date.now() - startedAtRef.current
      const base = elapsed < RUN_FAST_POLL_WINDOW_MS
        ? RUN_POLL_FAST_INTERVAL
        : elapsed < RUN_MEDIUM_POLL_WINDOW_MS
          ? RUN_POLL_INTERVAL
          : RUN_POLL_SLOW_INTERVAL
      return base + Math.floor(Math.random() * RUN_POLL_JITTER_MS)
    },
  })
  return useMemo(
    () => toQueryState(environment, enabledQuery, query),
    [enabledQuery, environment, query.data, query.isError, query.isPending, query.refetch],
  )
}
