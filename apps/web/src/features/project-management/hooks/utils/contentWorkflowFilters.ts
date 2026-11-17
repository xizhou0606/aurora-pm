import { useMemo } from 'react'

import type { PlanUnitContentSearch } from '../../model'
import { optionalPageSize, type ProjectContentSearchPatch } from './contentWorkflowHelpers'

export interface RequirementFiltersInput {
  search: PlanUnitContentSearch['requirement']
  navigateSearch: (patch: ProjectContentSearchPatch, replace?: boolean) => void
}

export function useRequirementFilters(input: RequirementFiltersInput) {
  return useMemo(() => ({
    q: input.search.q,
    status: input.search.status,
    priority: input.search.priority,
    page: input.search.page,
    pageSize: input.search.pageSize,
    setQ: (value: string) => input.navigateSearch({ req_q: value || undefined, req_page: undefined }),
    setStatus: (value: PlanUnitContentSearch['requirement']['status']) => input.navigateSearch({ req_status: value, req_page: undefined }),
    setPriority: (value: PlanUnitContentSearch['requirement']['priority']) => input.navigateSearch({ req_priority: value, req_page: undefined }),
    reset: () => input.navigateSearch({ req_q: undefined, req_status: undefined, req_priority: undefined, req_page: undefined }),
    setPage: (page: number) => input.navigateSearch({ req_page: page > 1 ? page : undefined }),
    setPageSize: (pageSize: number) => input.navigateSearch({ req_page_size: optionalPageSize(pageSize), req_page: undefined }),
  }), [input.navigateSearch, input.search])
}

export interface TaskFiltersInput {
  search: PlanUnitContentSearch['task']
  taskRequirementRows: Array<{ id: number; title: string }>
  navigateSearch: (patch: ProjectContentSearchPatch, replace?: boolean) => void
}

export function useTaskFilters(input: TaskFiltersInput) {
  return useMemo(() => ({
    requirementId: input.search.requirementId,
    requirements: input.taskRequirementRows.map((row) => ({ id: row.id, label: row.title })),
    page: input.search.page,
    pageSize: input.search.pageSize,
    setRequirementId: (value?: number) => input.navigateSearch({ task_requirement_id: value, task_id: undefined, task_page: undefined }),
    setPage: (page: number) => input.navigateSearch({ task_id: undefined, task_page: page > 1 ? page : undefined }),
    setPageSize: (pageSize: number) => input.navigateSearch({ task_id: undefined, task_page_size: optionalPageSize(pageSize), task_page: undefined }),
  }), [input.navigateSearch, input.search, input.taskRequirementRows])
}