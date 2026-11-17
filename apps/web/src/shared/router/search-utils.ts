export const projectDetailTabs = [
  'overview',
  'items',
  'activity',
] as const

export const LEGACY_PROJECT_ITEMS_TAB = 'plan_units'

export type ProjectDetailTab = (typeof projectDetailTabs)[number]

export const projectSearchTabs = [
  ...projectDetailTabs,
  'requirements',
  'tasks',
  'delivery',
] as const

export type ProjectSearchTab = (typeof projectSearchTabs)[number]

export const planUnitDetailTabs = [
  'overview',
  'requirements',
  'tasks',
  'delivery',
  'activity',
] as const

export const requirementStatuses = [
  'backlog',
  'grooming',
  'development',
  'acceptance',
  'done',
] as const

export const contentPriorities = ['low', 'normal', 'high', 'urgent'] as const

export const contentPageSizes = [20, 50, 100] as const

export const identityResults = ['bound', 'failed'] as const

type SearchRecord = Record<string, unknown>

export function searchRecord(value: unknown): SearchRecord {
  return value && typeof value === 'object' ? (value as SearchRecord) : {}
}

export function stringValue(value: unknown): string | undefined {
  const text = typeof value === 'string'
    ? value.trim()
    : typeof value === 'number' || typeof value === 'boolean'
      ? String(value)
      : ''
  return text || undefined
}

export function trueFlag(value: unknown): boolean {
  return value === true || value === 1 || value === '1' || value === 'true'
}

export function csvValue(value: unknown, numeric = false): string | undefined {
  const source = Array.isArray(value) ? value.join(',') : stringValue(value)
  if (!source) return undefined
  const values = [...new Set(
    source
      .split(',')
      .map((item) => item.trim())
      .filter((item) => item && (!numeric || /^[1-9]\d*$/.test(item))),
  )]
  return values.length ? values.join(',') : undefined
}

export function strictNumericCsv(value: unknown): { value?: string; invalid: boolean } {
  if (value === undefined) return { invalid: false }
  if (Array.isArray(value) && value.length !== 1) return { invalid: true }
  const source = String(Array.isArray(value) ? value[0] : value).trim()
  const parts = source.split(',').map((item) => item.trim())
  if (
    !source
    || parts.some((item) => !/^[1-9]\d*$/.test(item))
    || new Set(parts).size !== parts.length
    || parts.some((item) => !Number.isSafeInteger(Number(item)))
  ) {
    return { invalid: true }
  }
  return { value: parts.join(','), invalid: false }
}

export function boundedText(value: unknown, maxLength = 120): string | undefined {
  return stringValue(value)?.slice(0, maxLength)
}

export function loginCodeValue(value: unknown): string | undefined {
  const code = stringValue(value)
  return code && code.length <= 256 ? code : undefined
}

export function supportedPageSize(value: unknown): 10 | 20 | 50 | 100 | undefined {
  const pageSize = Number(value)
  return [10, 20, 50, 100].includes(pageSize)
    ? (pageSize as 10 | 20 | 50 | 100)
    : undefined
}

export function parsePositiveRouteId(value: unknown): number | undefined {
  const id = Number(value || 0)
  return Number.isInteger(id) && id > 0 ? id : undefined
}

export function positivePage(value: unknown): number | undefined {
  const page = parsePositiveRouteId(value)
  return page && page > 1 ? page : undefined
}

export function contentPageSize(value: unknown): (typeof contentPageSizes)[number] | undefined {
  const pageSize = Number(value)
  return contentPageSizes.includes(pageSize as (typeof contentPageSizes)[number])
    ? (pageSize as (typeof contentPageSizes)[number])
    : undefined
}

export function enumValue<const TValues extends readonly string[]>(
  value: unknown,
  values: TValues,
): TValues[number] | undefined {
  const normalized = stringValue(value)
  return values.includes(normalized as TValues[number])
    ? (normalized as TValues[number])
    : undefined
}

export function projectScopeSearch(search: SearchRecord) {
  const projectSets = strictNumericCsv(search.project_set_ids)
  const projects = strictNumericCsv(search.project_ids)
  const projectNews = strictNumericCsv(search.project_new_ids)
  const requirements = strictNumericCsv(search.requirement_ids)
  const invalid = projectSets.invalid || projects.invalid || projectNews.invalid || requirements.invalid
  return {
    ...(projectSets.value
      ? { project_set_ids: projectSets.value }
      : {}),
    ...(projects.value
      ? { project_ids: projects.value }
      : {}),
    ...(projectNews.value
      ? { project_new_ids: projectNews.value }
      : {}),
    ...(requirements.value
      ? { requirement_ids: requirements.value }
      : {}),
    ...(parsePositiveRouteId(search.team_id)
      ? { team_id: parsePositiveRouteId(search.team_id) }
      : {}),
    ...(parsePositiveRouteId(search.project_id)
      ? { project_id: parsePositiveRouteId(search.project_id) }
      : {}),
    ...(invalid || trueFlag(search.invalid_scope) ? { invalid_scope: '1' as const } : {}),
  }
}

export function hasLegacyProjectScope(search: SearchRecord): boolean {
  return [
    'project_set_id', 'project_id', 'project_new_ids', 'requirement_id', 'team_id',
  ].some((key) => search[key] !== undefined)
}