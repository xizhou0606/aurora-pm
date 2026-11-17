import {
  boundedText,
  enumValue,
  hasLegacyProjectScope,
  parsePositiveRouteId,
  positivePage,
  projectScopeSearch,
  searchRecord,
  strictNumericCsv,
  supportedPageSize,
  trueFlag,
} from './search-utils'

export function parseQualitySearch(value: unknown) {
  const search = searchRecord(value)
  const numericFilter = (raw: unknown): number | string | undefined => {
    const value = strictNumericCsv(raw).value
    if (!value) return undefined
    return value.includes(',') ? value : Number(value)
  }
  const assigneeId = numericFilter(search.assignee_user_id)
  const ownerId = numericFilter(search.current_owner_user_id ?? search.owner_id)
  const reporterId = numericFilter(search.reporter_user_id ?? search.reporter_id)
  const relation = enumValue(search.relation, ['reported', 'assigned', 'responsible', 'resolved'] as const)
    || (trueFlag(search.responsible_by_me)
      ? 'responsible'
      : trueFlag(search.resolved_by_me)
      ? 'resolved'
      : trueFlag(search.assigned_to_me)
        ? 'assigned'
        : trueFlag(search.reported_by_me)
          ? 'reported'
          : undefined)
  return {
    ...projectScopeSearch(search),
    ...(boundedText(search.q) ? { q: boundedText(search.q) } : {}),
    ...(boundedText(search.status, 64) ? { status: boundedText(search.status, 64) } : {}),
    ...(boundedText(search.severity_key, 64)
      ? { severity_key: boundedText(search.severity_key, 64) }
      : {}),
    ...(boundedText(search.found_environment, 64)
      ? { found_environment: boundedText(search.found_environment, 64) }
      : {}),
    ...(assigneeId
      ? { assignee_user_id: assigneeId }
      : {}),
    ...(ownerId
      ? { current_owner_user_id: ownerId }
      : {}),
    ...(reporterId
      ? { reporter_user_id: reporterId }
      : {}),
    ...(relation ? { relation } : {}),
    ...(boundedText(search.created_from, 10)
      ? { created_from: boundedText(search.created_from, 10) }
      : {}),
    ...(boundedText(search.created_to, 10)
      ? { created_to: boundedText(search.created_to, 10) }
      : {}),
    ...(enumValue(search.sort_by, ['priority', 'severity', 'status', 'created_at', 'status_changed_at'] as const)
      ? { sort_by: enumValue(search.sort_by, ['priority', 'severity', 'status', 'created_at', 'status_changed_at'] as const) }
      : {}),
    ...(search.sort_order === 'asc' ? { sort_order: 'asc' as const } : {}),
    ...(positivePage(search.page) ? { page: positivePage(search.page) } : {}),
    ...(supportedPageSize(search.page_size)
      ? { page_size: supportedPageSize(search.page_size) }
      : {}),
    ...(parsePositiveRouteId(search.bug_id)
      ? { bug_id: parsePositiveRouteId(search.bug_id) }
      : {}),
    ...(trueFlag(search.create) ? { create: '1' as const } : {}),
    ...(search.mode === 'edit' || trueFlag(search.edit) ? { mode: 'edit' as const } : {}),
    ...(parsePositiveRouteId(search.source_execution_id)
      ? { source_execution_id: parsePositiveRouteId(search.source_execution_id) }
      : {}),
    ...(parsePositiveRouteId(search.source_test_id)
      ? { source_test_id: parsePositiveRouteId(search.source_test_id) }
      : {}),
    ...(boundedText(search.actual_result, 500)
      ? { actual_result: boundedText(search.actual_result, 500) }
      : {}),
    ...(search.from === 'inbox' ? { from: 'inbox' as const } : {}),
    ...(hasLegacyProjectScope(search) ? { invalid_scope: '1' as const } : {}),
  }
}

export function parseQualityCreateSearch(value: unknown) {
  const search = searchRecord(value)
  return {
    ...projectScopeSearch(search),
    ...(parsePositiveRouteId(search.source_execution_id)
      ? { source_execution_id: parsePositiveRouteId(search.source_execution_id) }
      : {}),
    ...(parsePositiveRouteId(search.source_test_id)
      ? { source_test_id: parsePositiveRouteId(search.source_test_id) }
      : {}),
    ...(boundedText(search.actual_result, 500)
      ? { actual_result: boundedText(search.actual_result, 500) }
      : {}),
    ...(search.from === 'inbox' ? { from: 'inbox' as const } : {}),
    ...(hasLegacyProjectScope(search) ? { invalid_scope: '1' as const } : {}),
  }
}

export function parseQualityDetailSearch(value: unknown) {
  const search = searchRecord(value)
  return {
    ...projectScopeSearch(search),
    ...(search.mode === 'edit' || trueFlag(search.edit) ? { mode: 'edit' as const } : {}),
    ...(search.from === 'inbox' ? { from: 'inbox' as const } : {}),
    ...(hasLegacyProjectScope(search) ? { invalid_scope: '1' as const } : {}),
  }
}
