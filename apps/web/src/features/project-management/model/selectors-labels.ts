import { copy } from '@/shared/copy'
import type { ScrumHealthStatus, PlatformUser } from '../api'
import type { UserPreviewModel } from './selectors-types'

export function clampProgress(value: unknown): number {
  const number = Number(value || 0)
  if (!Number.isFinite(number)) return 0
  return Math.min(100, Math.max(0, Math.round(number)))
}

export function projectItemTypeLabel(value?: string): string {
  if (value === 'project') return copy.projectManagement.planUnitList.unitProject
  if (value === 'version') return copy.projectManagement.planUnitList.unitVersion
  return copy.projectManagement.planUnits
}

export function statusLabel(value?: string): string {
  const labels = copy.projectManagement.statusLabels
  return labels[value as keyof typeof labels] ?? labels.unknown
}

export function healthLabel(value?: ScrumHealthStatus | string): string {
  const labels = copy.projectManagement.healthLabels
  return labels[value as keyof typeof labels] ?? labels.unknown
}

export function riskLabel(value?: string): string {
  const labels = copy.projectManagement.planUnitList.riskLabels
  return labels[value as keyof typeof labels] ?? labels.unknown
}

export function ownerLabel(
  value?: number | null,
  users: readonly PlatformUser[] = [],
 resolvedName?: string,
): string {
  if (resolvedName?.trim()) return resolvedName.trim()
  const id = Number(value || 0)
  const user = users.find((candidate) => candidate.id === id)
  if (user) return user.display_name || user.username
  return copy.projectManagement.projectList.owner(Number(value || 0))
}

export function ownerPreview(
  value?: number | null,
  users: readonly PlatformUser[] = [],
 resolvedName?: string,
): UserPreviewModel {
  const userId = Number(value || 0)
  const user = users.find((candidate) => candidate.id === userId)
  const name = resolvedName?.trim() || user?.display_name?.trim() || user?.username?.trim()
    || ownerLabel(value, users)
  return {
    ...(userId > 0 ? { userId } : {}),
    name,
    ...(user?.username && user.username !== name
      ? { secondary: user.username }
      : {}),
    ...(user?.avatar_url ? { avatarUrl: user.avatar_url } : {}),
  }
}

export function formatPMDate(value?: string | null): string {
  if (!value) return copy.projectManagement.common.unknownDate
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return copy.projectManagement.common.unknownDate
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(date)
}

export function formatPMDateTime(value?: string | null): string {
  if (!value) return copy.projectManagement.common.unknownDate
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return copy.projectManagement.common.unknownDate
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

export function formatPMDateRange(start?: string | null, end?: string | null): string {
  return `${formatPMDate(start)} — ${formatPMDate(end)}`
}