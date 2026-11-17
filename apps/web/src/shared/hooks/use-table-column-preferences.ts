import { useCallback, useMemo } from 'react'
import { useStore } from 'zustand'
import { createStore } from 'zustand/vanilla'

import {
  normalizeTableColumnPreference,
  type TableColumnOption,
  type TableColumnPreference,
} from '@/shared/model/table-column-preferences'

const STORAGE_KEY = 'pm_table_column_preferences_v1'

type StoredPreferences = Record<string, TableColumnPreference<string>>
type PreferenceState = { values: StoredPreferences }

export type TableColumnPreferenceControl<TId extends string> = {
  preference: TableColumnPreference<TId>
  setVisible: (id: TId, visible: boolean) => void
  move: (id: TId, direction: -1 | 1) => void
  moveTo: (id: TId, targetId: TId) => void
  reset: () => void
}

function readPreferences(): StoredPreferences {
  try {
    const parsed = JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}')
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? parsed as StoredPreferences
      : {}
  } catch {
    return {}
  }
}

const preferenceStore = createStore<PreferenceState>(() => ({ values: readPreferences() }))

function writePreference(scope: string, preference?: TableColumnPreference<string>) {
  const current = preferenceStore.getState().values
  const values = { ...current }
  if (preference) values[scope] = preference
  else delete values[scope]
  preferenceStore.setState({ values })
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(values))
  } catch {
    // The current session still receives the preference when browser storage is unavailable.
  }
}

export function useTableColumnPreferences<TId extends string>({
  namespace,
  workspaceId,
  userId,
  columns,
}: {
  namespace: string
  workspaceId: number
  userId: number
  columns: readonly TableColumnOption<TId>[]
}): TableColumnPreferenceControl<TId> {
  const scope = `${namespace}:workspace-${Math.max(0, workspaceId)}:user-${Math.max(0, userId)}`
  const stored = useStore(preferenceStore, (state) => state.values[scope])
  const preference = useMemo(
    () => normalizeTableColumnPreference(columns, stored),
    [columns, stored],
  )
  const commit = useCallback((next: TableColumnPreference<TId>) => {
    writePreference(scope, next)
  }, [scope])
  const setVisible = useCallback((id: TId, visible: boolean) => {
    const column = columns.find((entry) => entry.id === id)
    if (!column || (column.required && !visible)) return
    const hidden = visible
      ? preference.hidden.filter((entry) => entry !== id)
      : [...preference.hidden.filter((entry) => entry !== id), id]
    commit({ ...preference, hidden })
  }, [columns, commit, preference])
  const move = useCallback((id: TId, direction: -1 | 1) => {
    const index = preference.order.indexOf(id)
    const target = index + direction
    if (index < 0 || target < 0 || target >= preference.order.length) return
    const sourceColumn = columns.find((entry) => entry.id === id)
    const targetColumn = columns.find((entry) => entry.id === preference.order[target])
    if (sourceColumn?.fixed || targetColumn?.fixed) return
    const order = [...preference.order]
    ;[order[index], order[target]] = [order[target], order[index]]
    commit({ ...preference, order })
  }, [columns, commit, preference])
  const moveTo = useCallback((id: TId, targetId: TId) => {
    const sourceIndex = preference.order.indexOf(id)
    const targetIndex = preference.order.indexOf(targetId)
    if (sourceIndex < 0 || targetIndex < 0 || sourceIndex === targetIndex) return
    const sourceColumn = columns.find((entry) => entry.id === id)
    const targetColumn = columns.find((entry) => entry.id === targetId)
    if (sourceColumn?.fixed || targetColumn?.fixed) return
    const order = [...preference.order]
    const [source] = order.splice(sourceIndex, 1)
    order.splice(targetIndex, 0, source)
    commit({ ...preference, order })
  }, [columns, commit, preference])
  const reset = useCallback(() => writePreference(scope), [scope])
  return { preference, setVisible, move, moveTo, reset }
}

export { STORAGE_KEY as TABLE_COLUMN_PREFERENCES_STORAGE_KEY }
