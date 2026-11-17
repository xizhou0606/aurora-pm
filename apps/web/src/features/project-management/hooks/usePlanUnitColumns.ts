import { useCallback, useMemo, useState } from 'react'

import { copy } from '@/shared/copy'

export const PLAN_UNIT_COLUMN_KEYS = [
  'owner',
  'status',
  'progress',
  'due',
  'participants',
  'type',
  'plannedStart',
  'deliveryNodes',
  'actualDates',
  'requirements',
  'tasks',
  'risk',
] as const

export type PlanUnitColumnKey = (typeof PLAN_UNIT_COLUMN_KEYS)[number]

export const DEFAULT_PLAN_UNIT_COLUMNS: readonly PlanUnitColumnKey[] = [
  'owner',
  'status',
  'progress',
  'due',
  'participants',
]

const columnCopy = copy.projectManagement.planUnitList

const COLUMN_LABELS: Record<PlanUnitColumnKey, string> = {
  owner: columnCopy.columnOwner,
  status: columnCopy.columnStatus,
  progress: columnCopy.columnProgress,
  due: columnCopy.columnDue,
  participants: columnCopy.columnParticipants,
  type: columnCopy.columnTypeOption,
  plannedStart: columnCopy.columnPlannedStart,
  deliveryNodes: columnCopy.columnDeliveryNodes,
  actualDates: columnCopy.columnActualDates,
  requirements: columnCopy.columnRequirements,
  tasks: columnCopy.columnTasks,
  risk: columnCopy.columnRisk,
}

export function usePlanUnitColumns() {
  const [visible, setVisible] = useState<ReadonlySet<PlanUnitColumnKey>>(
    () => new Set(DEFAULT_PLAN_UNIT_COLUMNS),
  )
  const options = useMemo(() => PLAN_UNIT_COLUMN_KEYS.map((key) => ({
    key,
    label: COLUMN_LABELS[key],
  })), [])
  const isVisible = useCallback((key: PlanUnitColumnKey) => visible.has(key), [visible])
  const toggle = useCallback((key: PlanUnitColumnKey) => {
    setVisible((current) => {
      const next = new Set(current)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }, [])

  return { options, isVisible, toggle }
}
