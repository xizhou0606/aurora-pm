import { optionalPMRedirect, safePMRedirect } from '@/shared/config'
import { externalLoginProviders, linkableLoginProviders } from '@/shared/model'
import {
  boundedText,
  contentPageSize,
  contentPriorities,
  csvValue,
  enumValue,
  hasLegacyProjectScope,
  LEGACY_PROJECT_ITEMS_TAB,
  loginCodeValue,
  parsePositiveRouteId,
  planUnitDetailTabs,
  positivePage,
  projectDetailTabs,
  projectScopeSearch,
  projectSearchTabs,
  requirementStatuses,
  searchRecord,
  strictNumericCsv,
  stringValue,
  supportedPageSize,
  trueFlag,
  type ProjectSearchTab,
} from './search-utils'

// Re-export everything from search-utils
export {
  boundedText,
  contentPageSize,
  contentPriorities,
  csvValue,
  enumValue,
  hasLegacyProjectScope,
  LEGACY_PROJECT_ITEMS_TAB,
  loginCodeValue,
  parsePositiveRouteId,
  planUnitDetailTabs,
  positivePage,
  projectDetailTabs,
  projectScopeSearch,
  projectSearchTabs,
  requirementStatuses,
  searchRecord,
  strictNumericCsv,
  stringValue,
  supportedPageSize,
  trueFlag,
} from './search-utils'

export type { ProjectDetailTab, ProjectSearchTab } from './search-utils'

// Re-export quality parse functions
export {
  parseQualityCreateSearch,
  parseQualityDetailSearch,
  parseQualitySearch,
} from './search-quality'

const identityResults = ['bound', 'failed'] as const

export function parseLoginSearch(value: unknown) {
  const search = searchRecord(value)
  const redirect = stringValue(search.redirect)
  return {
    redirect: redirect ? optionalPMRedirect(redirect) : undefined,
    login_code: loginCodeValue(search.login_code),
    external_registration_code: loginCodeValue(search.external_registration_code),
    login_provider: enumValue(search.login_provider, externalLoginProviders),
  }
}

export function parseProfileSearch(value: unknown) {
  const search = searchRecord(value)
  const provider = enumValue(search.login_provider, linkableLoginProviders)
  const result = enumValue(search.identity_result, identityResults)
  return provider && result
    ? { identity_result: result, login_provider: provider }
    : { identity_result: undefined, login_provider: undefined }
}

export function parseWorkspaceSearch(value: unknown) {
  const search = searchRecord(value)
  const redirect = stringValue(search.redirect)
  return {
    redirect: redirect ? safePMRedirect(redirect) : undefined,
    empty: trueFlag(search.empty) ? ('1' as const) : undefined,
    error: search.error === 'load' ? ('load' as const) : undefined,
  }
}

export function parseProjectsSearch(value: unknown) {
  const search = searchRecord(value)
  const projectId = parsePositiveRouteId(search.project_id)
  return {
    ...(projectId ? { project_id: projectId } : {}),
  }
}

function projectTab(
  value: unknown,
  allowed: readonly ProjectSearchTab[],
  fallback: ProjectSearchTab,
) {
  const tab = stringValue(value)
  return allowed.includes(tab as ProjectSearchTab)
    ? (tab as ProjectSearchTab)
    : fallback
}

export function parseProjectSearch(value: unknown) {
  const search = searchRecord(value)
  const tab = stringValue(search.tab) === LEGACY_PROJECT_ITEMS_TAB
    ? 'items'
    : projectTab(search.tab, projectDetailTabs, 'overview')
  return {
    tab,
    from_item: parsePositiveRouteId(search.from_item),
  }
}

export function hasLegacyProjectItemsSearch(search: string): boolean {
  return new URLSearchParams(search).get('tab') === LEGACY_PROJECT_ITEMS_TAB
}

export function parsePlanUnitSearch(value: unknown) {
  const search = searchRecord(value)
  const tab = projectTab(search.tab, planUnitDetailTabs, 'requirements')
  const reqQ = stringValue(search.req_q)?.slice(0, 120)
  const reqStatus = enumValue(search.req_status, requirementStatuses)
  const reqPriority = enumValue(search.req_priority, contentPriorities)
  const reqPage = positivePage(search.req_page)
  const reqPageSize = contentPageSize(search.req_page_size)
  const taskRequirementId = parsePositiveRouteId(search.task_requirement_id)
  const taskId = tab === 'tasks' && taskRequirementId
    ? parsePositiveRouteId(search.task_id)
    : undefined
  const taskPage = positivePage(search.task_page)
  const taskPageSize = contentPageSize(search.task_page_size)
  const workflowNode = boundedText(search.workflow_node)
  return {
    tab,
    from_item: undefined,
    ...(workflowNode ? { workflow_node: workflowNode } : {}),
    ...(reqQ ? { req_q: reqQ } : {}),
    ...(reqStatus ? { req_status: reqStatus } : {}),
    ...(reqPriority ? { req_priority: reqPriority } : {}),
    ...(reqPage ? { req_page: reqPage } : {}),
    ...(reqPageSize ? { req_page_size: reqPageSize } : {}),
    ...(taskRequirementId ? { task_requirement_id: taskRequirementId } : {}),
    ...(taskId ? { task_id: taskId } : {}),
    ...(taskPage ? { task_page: taskPage } : {}),
    ...(taskPageSize ? { task_page_size: taskPageSize } : {}),
  }
}

export function parseRequirementPoolSearch(value: unknown) {
  const search = searchRecord(value)
  const projectSets = strictNumericCsv(search.project_set_ids)
  const projects = strictNumericCsv(search.project_ids)
  const productOwners = strictNumericCsv(search.product_owner_user_id)
  const designOwners = strictNumericCsv(search.design_owner_user_id)
  const frontendOwners = strictNumericCsv(search.frontend_owner_user_id)
  const backendOwners = strictNumericCsv(search.backend_owner_user_id)
  const testers = strictNumericCsv(search.tester_user_id)
  const nodes = csvValue(search.current_node_id)
  return {
    ...(boundedText(search.q) ? { q: boundedText(search.q) } : {}),
    ...(projectSets.value ? { project_set_ids: projectSets.value } : {}),
    ...(projects.value ? { project_ids: projects.value } : {}),
    ...(trueFlag(search.unscheduled) ? { unscheduled: '1' as const } : {}),
    ...(nodes ? { current_node_id: nodes } : {}),
    ...(productOwners.value ? { product_owner_user_id: productOwners.value } : {}),
    ...(designOwners.value ? { design_owner_user_id: designOwners.value } : {}),
    ...(frontendOwners.value ? { frontend_owner_user_id: frontendOwners.value } : {}),
    ...(backendOwners.value ? { backend_owner_user_id: backendOwners.value } : {}),
    ...(testers.value ? { tester_user_id: testers.value } : {}),
    ...(enumValue(search.view, ['cards'] as const) ? { view: 'cards' as const } : {}),
    ...(positivePage(search.page) ? { page: positivePage(search.page) } : {}),
    ...(contentPageSize(search.page_size) ? { page_size: contentPageSize(search.page_size) } : {}),
  }
}

export function parseRequirementDetailSearch(value: unknown) {
  const search = searchRecord(value)
  const workflowNode = boundedText(search.workflow_node)
  return workflowNode ? { workflow_node: workflowNode } : {}
}

export function parseInboxSearch(value: unknown) {
  const search = searchRecord(value)
  const scope = projectScopeSearch(search)
  return {
    ...scope,
    ...(parsePositiveRouteId(search.team_id)
      ? { team_id: parsePositiveRouteId(search.team_id) }
      : {}),
    ...(parsePositiveRouteId(search.project_id)
      ? { project_id: parsePositiveRouteId(search.project_id) }
      : {}),
    ...(boundedText(search.q) ? { q: boundedText(search.q) } : {}),
    ...(positivePage(search.workflow_page)
      ? { workflow_page: positivePage(search.workflow_page) }
      : {}),
    ...(positivePage(search.task_page) ? { task_page: positivePage(search.task_page) } : {}),
    ...(positivePage(search.bug_page) ? { bug_page: positivePage(search.bug_page) } : {}),
    ...(positivePage(search.mention_page)
      ? { mention_page: positivePage(search.mention_page) }
      : {}),
    ...(hasLegacyProjectScope(search) || scope.invalid_scope === '1'
      ? { invalid_scope: '1' as const }
      : {}),
  }
}

export function parseWorkItemsSearch(value: unknown) {
  const search = searchRecord(value)
  const teamIds = strictNumericCsv(search.team_ids)
  const classicProjectIds = strictNumericCsv(search.classic_project_ids)
  const assigneeIds = strictNumericCsv(search.assignee_ids)
  const pmAssigneeIds = strictNumericCsv(search.pm_assignee_ids)
  const one2allAssigneeIds = strictNumericCsv(search.one2all_assignee_ids)
  const invalidNumeric = [teamIds, classicProjectIds, assigneeIds, pmAssigneeIds, one2allAssigneeIds]
    .some((entry) => entry.invalid)
  const view = enumValue(search.view, ['board', 'list'] as const) ?? 'list'
  const layout = view === 'list'
    ? enumValue(search.layout, ['table', 'gantt'] as const) ?? 'table'
    : undefined
  return {
    ...projectScopeSearch(search),
    ...(teamIds.value ? { team_ids: teamIds.value } : {}),
    ...(classicProjectIds.value
      ? { classic_project_ids: classicProjectIds.value }
      : {}),
    view,
    ...(stringValue(search.from) === 'inbox' ? { from: 'inbox' as const } : {}),
    ...(layout ? { layout } : {}),
    ...(boundedText(search.q) ? { q: boundedText(search.q) } : {}),
    ...(trueFlag(search.overdue) ? { overdue: '1' as const } : {}),
    ...(assigneeIds.value ? { assignee_ids: assigneeIds.value } : {}),
    ...(pmAssigneeIds.value
      ? { pm_assignee_ids: pmAssigneeIds.value }
      : {}),
    ...(one2allAssigneeIds.value
      ? { one2all_assignee_ids: one2allAssigneeIds.value }
      : {}),
    ...(csvValue(search.statuses) ? { statuses: csvValue(search.statuses) } : {}),
    ...(csvValue(search.priorities) ? { priorities: csvValue(search.priorities) } : {}),
    ...(csvValue(search.filter_kinds) ? { filter_kinds: csvValue(search.filter_kinds) } : {}),
    ...(enumValue(search.group_by, ['requirement_id', 'assignee_id', 'priority'] as const)
      ? { group_by: enumValue(search.group_by, ['requirement_id', 'assignee_id', 'priority'] as const) }
      : {}),
    ...(positivePage(search.page) ? { page: positivePage(search.page) } : {}),
    ...(supportedPageSize(search.page_size)
      ? { page_size: supportedPageSize(search.page_size) }
      : {}),
    ...(parsePositiveRouteId(search.task_id ?? search.detail_id)
      ? { task_id: parsePositiveRouteId(search.task_id ?? search.detail_id) }
      : {}),
    ...(search.mode === 'edit' ? { mode: 'edit' as const } : {}),
    ...(invalidNumeric || trueFlag(search.invalid_scope) ? { invalid_scope: '1' as const } : {}),
  }
}

export function parseBiScreenSearch(value: unknown) {
  const search = searchRecord(value)
  const legacyScope = hasLegacyProjectScope(search)
  const dimension = enumValue(search.team_dimension, ['version', 'month', 'week'] as const)
  const dimensionKey = boundedText(search.team_dimension_key, 16)
  const validDimensionKey = dimension === 'month'
    ? (/^\d{4}-(0[1-9]|1[0-2])$/.test(dimensionKey || '') ? dimensionKey : undefined)
    : dimension === 'week'
      ? (/^\d{4}-W(0[1-9]|[1-4]\d|5[0-3])$/.test(dimensionKey || '') ? dimensionKey : undefined)
      : (/^[1-9]\d*$/.test(dimensionKey || '') ? dimensionKey : undefined)
  return {
    ...projectScopeSearch(search),
    ...(enumValue(search.period, ['week', 'month'] as const)
      ? { period: enumValue(search.period, ['week', 'month'] as const) }
      : {}),
		...(search.bi_view === 'team' ? { bi_view: 'team' as const } : {}),
		...(search.bi_view === 'team' && search.team_view === 'chart'
			? { team_view: 'chart' as const }
			: {}),
    ...(dimension ? { team_dimension: dimension } : {}),
    ...(validDimensionKey ? { team_dimension_key: validDimensionKey } : {}),
    ...(trueFlag(search.global) ? { global: '1' as const } : {}),
    ...(legacyScope ? { invalid_scope: '1' as const } : {}),
  }
}
