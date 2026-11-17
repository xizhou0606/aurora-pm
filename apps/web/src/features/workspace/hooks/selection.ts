import type { WorkspaceOption, WorkspaceSelection } from '../model/types'

export function normalizeWorkspaceOptions(
  options: readonly WorkspaceOption[],
): WorkspaceOption[] {
  const normalized: WorkspaceOption[] = []
  const seen = new Set<number>()
  for (const option of options) {
    const id = Number(option?.id || 0)
    const name = String(option?.name || option?.slug || '').trim()
    if (
      !Number.isInteger(id) ||
      id <= 0 ||
      !name ||
      option?.enabled === false ||
      seen.has(id)
    ) {
      continue
    }
    seen.add(id)
    normalized.push({ ...option, id, name })
  }
  return normalized
}

export function resolveWorkspaceSelection(
  options: readonly WorkspaceOption[],
  currentWorkspaceId: number,
): WorkspaceSelection {
  const selectable = normalizeWorkspaceOptions(options)
  const current = selectable.find(
    (workspace) => workspace.id === currentWorkspaceId,
  )
  if (current) {
    return { status: 'ok', workspaceId: current.id, reason: 'current' }
  }
  if (selectable.length === 0) return { status: 'empty' }
  if (selectable.length === 1) {
    return { status: 'ok', workspaceId: selectable[0].id, reason: 'single' }
  }
  const preferred = selectable.find((workspace) => workspace.is_default)
  if (preferred) {
    return { status: 'ok', workspaceId: preferred.id, reason: 'default' }
  }
  return { status: 'pick' }
}
