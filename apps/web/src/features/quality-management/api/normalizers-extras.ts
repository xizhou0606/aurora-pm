import {
  isSafeApiError,
  normalizePlatformUsers,
  type PlatformUser,
} from '@/shared/api'
import type {
  QualityActivity,
  QualityActivityPage,
  QualityComment,
  QualityDashboard,
  QualityMaterial,
  QualitySceneOption,
  QualityUser,
} from '../model/types'
import type { QualityRawPage } from './contracts'
import {
  array,
  nonNegative,
  optionalNumber,
  optionalString,
  record,
  safeMaterialUrl,
} from '../model/values'
import { QualityContractError } from './normalizers'

function dashboardSummary(value: unknown) {
  const source = record(value)
  return {
    submitted: nonNegative(source.submitted),
    resolved: nonNegative(source.resolved),
    closed: nonNegative(source.closed),
    backlog: nonNegative(source.backlog),
  }
}

export function normalizeQualityDashboard(value: unknown): QualityDashboard {
  const source = record(value)
  const dimensions = (input: unknown) => array(input).flatMap((entry) => {
    const row = record(entry)
    const key = optionalString(row.key)
    const label = optionalString(row.label)
    return key && label ? [{ key, label, count: nonNegative(row.count) }] : []
  })
  return {
    summary: dashboardSummary(source.summary),
    byPerson: array(source.by_person).map((entry) => {
      const row = record(entry)
      return {
        userId: optionalNumber(row.user_id) || 0,
        userName: optionalString(row.user_name) || '未分配',
        avatarUrl: optionalString(row.avatar_url),
        ...dashboardSummary(row),
      }
    }),
    byStatus: dimensions(source.by_status),
    bySeverity: dimensions(source.by_severity),
  }
}

function normalizeActivity(value: unknown): QualityActivity | null {
  const source = record(value)
  const id = optionalNumber(source.id)
  if (!id) return null
  return {
    id,
    userId: optionalNumber(source.user_id),
    userName: optionalString(source.user_name)
      || optionalString(source.display_name)
      || optionalString(source.username),
    createdAt: optionalString(source.created_at),
    action: optionalString(source.action),
    fieldKey: optionalString(source.field_key),
    oldValue: optionalString(source.old_value),
    newValue: optionalString(source.new_value),
    summary: optionalString(source.summary),
  }
}

export function normalizeActivityPage(
  page: QualityRawPage,
  offset: number,
): QualityActivityPage {
  const seen = new Set<number>()
  const rows = page.items.flatMap((entry) => {
    const activity = normalizeActivity(entry)
    if (!activity || seen.has(activity.id)) return []
    seen.add(activity.id)
    return [activity]
  })
  const consumed = offset + page.items.length
  return {
    rows,
    total: page.total,
    nextOffset: page.items.length > 0 && consumed < page.total ? consumed : undefined,
  }
}

export function normalizeQualityComments(page: QualityRawPage): QualityComment[] {
  return page.items.flatMap((entry) => {
    const source = record(entry)
    const id = optionalNumber(source.id)
    const body = optionalString(source.body)
    if (!id || !body) return []
    return [{
      id,
      body,
      userId: optionalNumber(source.user_id),
      userName: optionalString(source.display_name)
        || optionalString(source.user_name)
        || optionalString(source.username),
      createdAt: optionalString(source.created_at),
      canDelete: source.can_delete !== false,
    }]
  })
}

export function normalizeUsers(page: QualityRawPage | { items: unknown[]; total: number }): QualityUser[] {
  return platformUsersToQualityUsers(normalizePlatformUsers(page.items))
}

export function platformUsersToQualityUsers(users: readonly PlatformUser[]): QualityUser[] {
  return users.map((user) => ({
    id: user.id,
    name: user.display_name || user.username,
    username: user.username || undefined,
    avatarUrl: user.avatar_url,
  }))
}

export function normalizeUpload(value: unknown): QualityMaterial {
  const source = record(value)
  const nested = record(source.data)
  const url = safeMaterialUrl(source.url) || safeMaterialUrl(nested.url)
  if (!url) throw new QualityContractError('workspace_contract')
  return { url }
}

export function normalizeQualitySceneOptions(value: unknown): QualitySceneOption[] {
  const source = record(value)
  const rows = array(source.items)
  const seen = new Set<string>()
  return rows.flatMap((entry) => {
    const item = record(entry)
    const name = (optionalString(item.name) || '').trim()
    const normalized = name.toLocaleLowerCase()
    if (!name || seen.has(normalized)) return []
    seen.add(normalized)
    return [{
      id: optionalNumber(item.id),
      kind: optionalString(item.kind) as QualitySceneOption['kind'],
      name,
      builtIn: item.built_in === true,
      legacy: item.legacy === true,
      usageCount: optionalNumber(item.usage_count) || 0,
    }]
  })
}

export function normalizeQualitySceneOption(value: unknown): QualitySceneOption {
  const item = record(value)
  const name = (optionalString(item.name) || '').trim()
  if (!name) throw new QualityContractError('workspace_contract')
  return {
    id: optionalNumber(item.id),
    kind: optionalString(item.kind) as QualitySceneOption['kind'],
    name,
    builtIn: item.built_in === true,
    legacy: item.legacy === true,
    usageCount: optionalNumber(item.usage_count) || 0,
  }
}

export const normalizeQualityEnvironments = normalizeQualitySceneOptions
export const normalizeQualityEnvironment = normalizeQualitySceneOption