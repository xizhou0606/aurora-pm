import { WORK_ITEMS_COPY } from '@/shared/copy'
import { normalizeWorkItemStatus } from '@/shared/model'
import { isValidPMScopeSelection, type PMScopeOptions } from '@/shared/api'
import type {
  WorkItem,
  WorkItemBoardGroup,
  WorkItemBoardLane,
  WorkItemGroup,
  WorkItemListGroup,
  WorkItemTreeRow,
  WorkItemUserOption,
  WorkItemsGroupBy,
  WorkItemWorkflowStatus,
  WorkItemsScope,
} from './types'
import {
  resolveWorkItemAssignee,
  priorityLabel,
  statusLabel,
} from './presenters'

export function groupWorkItems(
  rows: readonly WorkItem[],
  groupBy: WorkItemsGroupBy,
  users: readonly WorkItemUserOption[] = [],
): WorkItemGroup[] {
  if (groupBy === 'none') return [{ id: 'all', label: '', rows: [...rows] }]
  const groups = new Map<string, WorkItemGroup>()
  for (const row of rows) {
    const label = groupBy === 'requirement_id'
      ? row.requirementTitle || WORK_ITEMS_COPY.list.noRequirement
      : groupBy === 'assignee_id'
        ? resolveWorkItemAssignee(row, users).name
        : priorityLabel(row.priority)
    const identity = groupBy === 'requirement_id'
      ? row.requirementId || 'none'
      : groupBy === 'assignee_id'
        ? row.assigneeId || 'none'
        : String(row.priority || 'none').toLowerCase()
    const id = `${groupBy}:${identity}`
    const group = groups.get(id) || { id, label, rows: [] }
    group.rows.push(row)
    groups.set(id, group)
  }
  return [...groups.values()]
}

function flattenWorkItemTree(rows: readonly WorkItem[]): Omit<WorkItemTreeRow, 'treeGroupId'>[] {
  const byParent = new Map<string, WorkItem[]>()
  const rowIds = new Set(rows.map((row) => row.rowId))
  for (const row of rows) {
    const parentRowId = row.parentId ? `task:${row.parentId}` : ''
    const parentKey = parentRowId && rowIds.has(parentRowId) ? parentRowId : 'root'
    byParent.set(parentKey, [...(byParent.get(parentKey) || []), row])
  }
  const result: Omit<WorkItemTreeRow, 'treeGroupId'>[] = []
  const visited = new Set<string>()
  const append = (row: WorkItem, depth: number) => {
    if (visited.has(row.rowId)) return
    visited.add(row.rowId)
    const children = byParent.get(row.rowId) || []
    result.push({ ...row, depth, hasChildren: children.length > 0 })
    children.forEach((child) => append(child, Math.min(depth + 1, 8)))
  }
  ;(byParent.get('root') || []).forEach((row) => append(row, 0))
  rows.forEach((row) => append(row, 0))
  return result
}

export function buildListGroups(
  rows: readonly WorkItem[],
  groupBy: WorkItemsGroupBy,
  users: readonly WorkItemUserOption[] = [],
): WorkItemListGroup[] {
  const preserveTree = groupBy === 'none' || groupBy === 'requirement_id'
  return groupWorkItems(rows, groupBy, users).map((group) => {
    const treeGroupId = `work-items-tree-${group.id.replace(/[^a-zA-Z0-9_-]/g, '-')}`
    const displayRows = preserveTree
      ? flattenWorkItemTree(group.rows)
      : group.rows.map((row) => ({ ...row, depth: 0, hasChildren: false }))
    return {
      id: group.id,
      label: group.label,
      rows: displayRows.map((row) => ({ ...row, treeGroupId })),
    }
  })
}

export function visibleTreeRows(
  rows: readonly WorkItemTreeRow[],
  expandedRowIds: ReadonlySet<string>,
): WorkItemTreeRow[] {
  const availableRowIds = new Set(rows.map((row) => row.rowId))
  const visible = new Set<string>()
  return rows.filter((row) => {
    const parentRowId = row.parentId ? `task:${row.parentId}` : undefined
    const parentIsInPage = parentRowId ? availableRowIds.has(parentRowId) : false
    const shown = !parentRowId
      || !parentIsInPage
      || (visible.has(parentRowId) && expandedRowIds.has(parentRowId))
    if (shown) visible.add(row.rowId)
    return shown
  })
}

export function buildBoardLanes(
  rows: readonly WorkItem[],
  statuses: readonly WorkItemWorkflowStatus[] = [],
): WorkItemBoardLane[] {
  const lanes = new Map<string, WorkItemBoardLane>()
  for (const status of statuses) {
    const key = normalizeWorkItemStatus(status.key)
    if (!key || lanes.has(key)) continue
    lanes.set(key, { id: key, label: status.label || statusLabel(key), rows: [] })
  }
  for (const row of rows) {
    const key = normalizeWorkItemStatus(row.status) || 'unknown'
    const lane = lanes.get(key) || {
      id: key,
      label: key === 'unknown' ? WORK_ITEMS_COPY.board.noStatus : statusLabel(key),
      rows: [],
    }
    lane.rows.push(row)
    lanes.set(key, lane)
  }
  return [...lanes.values()]
}

export function buildBoardGroups(
  rows: readonly WorkItem[],
  groupBy: WorkItemsGroupBy,
  statuses: readonly WorkItemWorkflowStatus[] = [],
  users: readonly WorkItemUserOption[] = [],
): WorkItemBoardGroup[] {
  return groupWorkItems(rows, groupBy, users).map((group) => ({
    id: group.id,
    label: group.label,
    lanes: buildBoardLanes(group.rows, statuses),
  }))
}

function canonicalScopeIds(ids: readonly number[]): boolean {
  return ids.every((id) => Number.isSafeInteger(id) && id > 0)
    && new Set(ids).size === ids.length
}

export function normalizeWorkItemsBoardScope(scope: WorkItemsScope): WorkItemsScope {
  return {
    projectSetIds: [],
    projectIds: [...scope.projectIds],
    requirementIds: [...scope.requirementIds],
  }
}

export function hasWorkItemsBoardScope(scope: WorkItemsScope): boolean {
  return scope.projectIds.length > 0 || scope.requirementIds.length > 0
}

/**
 * Board scope deliberately starts at project/version. Project-set ids are not
 * accepted by this contract, while requirements must still belong to an
 * explicitly selected project so a malformed child-only URL fails closed.
 */
export function isValidWorkItemsBoardScope(
  scope: WorkItemsScope,
  options: PMScopeOptions,
): boolean {
  if (
    scope.projectSetIds.length > 0
    || !canonicalScopeIds(scope.projectSetIds)
    || !canonicalScopeIds(scope.projectIds)
    || !canonicalScopeIds(scope.requirementIds)
    || !hasWorkItemsBoardScope(scope)
  ) return false
  const projects = new Set(options.projects.map(({ id }) => id))
  const selectedProjects = new Set(scope.projectIds)
  const requirements = new Map(options.requirements.map((option) => [option.id, option]))
  if (scope.projectIds.some((id) => !projects.has(id))) return false
  return scope.requirementIds.every((id) => {
    const requirement = requirements.get(id)
    return Boolean(requirement?.parentId && selectedProjects.has(requirement.parentId))
  })
}

/**
 * List routes accept both the full three-level hierarchy and the board's
 * project/requirement scope. This keeps a board selection valid when the user
 * switches to the list without weakening either hierarchy contract.
 */
export function isValidWorkItemsListScope(
  scope: WorkItemsScope,
  options: PMScopeOptions,
): boolean {
  return isValidPMScopeSelection(scope, options)
    || isValidWorkItemsBoardScope(scope, options)
}