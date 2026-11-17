import { useNavigate } from '@tanstack/react-router'

import { copy } from '@/shared/copy'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/shared/ui'
import type { ScrumVersionTaskNew } from '../api'
import { projectItemTypeLabel, type PlanUnitTab } from '../model'

export function PlanUnitSwitcher({
  projectId,
  currentId,
  units,
  tab,
  variant = 'title',
}: {
  projectId: number
  currentId?: number
  units: readonly ScrumVersionTaskNew[]
  tab: PlanUnitTab
  variant?: 'title' | 'compact' | 'header'
}) {
  const navigate = useNavigate()
  const detailCopy = copy.projectManagement.planUnitDetail
  const items = units.map((unit) => ({
    value: String(unit.id),
    label: unit.title,
  }))
  const current = units.find((unit) => unit.id === currentId)
  const selected = currentId && units.some((unit) => unit.id === currentId)
    ? String(currentId)
    : undefined

  const goToUnit = (value: string | null) => {
    if (!value) return
    const nextId = Number(value)
    if (!Number.isInteger(nextId) || nextId <= 0 || nextId === currentId) return
    void navigate({
      to: '/workspace/projects/$projectId/items/$projectItemId',
      params: {
        projectId: String(projectId),
        projectItemId: String(nextId),
      },
      search: { tab, from_item: undefined },
    })
  }

  if (units.length === 0) return null
  if (variant === 'title' && units.length <= 1) {
    return (
      <h2 className="mt-0.5 mb-0 truncate text-xl font-semibold tracking-tight text-foreground">
        {current?.title ?? units[0]?.title ?? ''}
      </h2>
    )
  }
  if (variant === 'compact' && units.length <= 1) return null
  if (variant === 'header' && units.length === 1) {
    return (
      <span
        data-test="plan-unit-switcher"
        className="inline-flex h-8 max-w-56 items-center truncate rounded-md border border-border bg-muted/40 px-2.5 text-sm font-medium text-foreground"
      >
        {units[0].title}
      </span>
    )
  }

  return (
    <div className={variant === 'title' ? 'min-w-0' : 'min-w-0 shrink-0'}>
      {variant === 'title' ? <h2 className="sr-only">{current?.title ?? ''}</h2> : null}
      <Select
        items={items}
        value={selected}
        onValueChange={goToUnit}
      >
        <SelectTrigger
          aria-label={detailCopy.switcherLabel}
          data-test="plan-unit-switcher"
          className={variant === 'header'
            ? 'h-8 w-auto min-w-[8.5rem] max-w-56 text-sm font-medium'
            : variant === 'compact'
              ? 'h-8 w-52 max-w-[min(100%,16rem)]'
              : 'h-auto min-h-10 w-full max-w-lg px-3 py-1.5 text-left text-xl font-semibold'}
        >
          <SelectValue placeholder={detailCopy.switcherLabel} />
        </SelectTrigger>
        <SelectContent className="min-w-64">
          {units.map((unit) => (
            <SelectItem key={unit.id} value={String(unit.id)}>
              {detailCopy.switcherOption(projectItemTypeLabel(unit.type), unit.title)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
