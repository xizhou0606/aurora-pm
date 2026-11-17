import { useCallback, useMemo, useState } from 'react'

import { copy } from '@/shared/copy'
import type { DataTableState } from '@/shared/ui'
import type { PlatformUser, ScrumVersionTaskNew } from '../api'
import {
  filterPlanUnits,
  healthLabel,
  ownerLabel,
  statusLabel,
  uniquePlanUnitOptions,
} from '../model'

type Option = { value: string; label: string }

type Control = {
  value: string
  options?: Option[]
  onChange: (value: string) => void
}

type OwnerOption = {
  id: number
  name: string
  username?: string
  avatarUrl?: string
}

export function usePlanUnitFilters(
  rows: ScrumVersionTaskNew[],
  users: PlatformUser[],
) {
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState('all')
  const [health, setHealth] = useState('all')
  const [owner, setOwner] = useState<number | undefined>()
  const statusOptions = useMemo<Option[]>(() => (
    uniquePlanUnitOptions(rows, 'status').map((value) => ({
      value,
      label: statusLabel(value),
    }))
  ), [rows])
  const healthOptions = useMemo<Option[]>(() => (
    uniquePlanUnitOptions(rows, 'health_status').map((value) => ({
      value,
      label: healthLabel(value),
    }))
  ), [rows])
  const ownerOptions = useMemo<OwnerOption[]>(() => {
    return users.map((user) => ({
      id: user.id,
      name: ownerLabel(user.id, users),
      username: user.username,
      avatarUrl: user.avatar_url,
    }))
  }, [users])
  const visibleRows = useMemo(() => filterPlanUnits(
    rows.filter((row) => owner === undefined || Number(row.owner_user_id || 0) === owner),
    { search, status, health },
    users,
  ), [health, owner, rows, search, status, users])
  const filtered = Boolean(search.trim() || status !== 'all' || health !== 'all' || owner !== undefined)
  const reset = useCallback(() => {
    setSearch('')
    setStatus('all')
    setHealth('all')
    setOwner(undefined)
  }, [])
  const tableState: DataTableState<ScrumVersionTaskNew> = visibleRows.length > 0
    ? { status: 'ready', rows: visibleRows }
    : {
        status: 'empty',
        title: filtered
          ? copy.projectManagement.planUnitList.filteredEmptyTitle
          : copy.projectManagement.planUnitList.emptyTitle
      }

  return {
    search: { value: search, onChange: setSearch } satisfies Control,
    status: { value: status, options: statusOptions, onChange: setStatus } satisfies Control,
    health: { value: health, options: healthOptions, onChange: setHealth } satisfies Control,
    owner: { value: owner, options: ownerOptions, onChange: setOwner },
    visibleRows,
    filtered,
    reset,
    tableState,
  }
}
