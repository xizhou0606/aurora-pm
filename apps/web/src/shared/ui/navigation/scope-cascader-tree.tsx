import { useState } from 'react'
import { PhCheck, PhCaretRight, PhMinus } from '@/shared/ui/icons'

import type { PMScopeOption } from '@/shared/api'
import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { SearchInput } from './search-input'
import { textMatchesQuery } from '../user/user-select-option'
import type { PMScopeLevel } from './scope-cascader-model'
import type { PMScopeCheckboxState } from './scope-cascader-model'

type Column = {
  level: PMScopeLevel
  label: string
  options: readonly PMScopeOption[]
  selectedIds: readonly number[]
  indeterminateIds?: readonly number[]
  unavailable?: string
  emptyLabel: string
  hasChildren: boolean
  activeId?: number
}

function Option({
  option,
  selected,
  indeterminate,
  hasChildren,
  active,
  onSelect,
  onNavigate,
}: {
  option: PMScopeOption
  selected: boolean
  indeterminate: boolean
  hasChildren: boolean
  active: boolean
  onSelect: (id: number) => void
  onNavigate: (id: number) => void
}) {
  return (
    <div
      data-slot="scope-cascader-option"
      className={cn(
        'grid min-h-9 w-full grid-cols-[1rem_minmax(0,1fr)_1.5rem] items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none',
        'hover:bg-muted focus-within:bg-muted',
        (selected || indeterminate) && 'bg-muted text-foreground',
      )}
    >
      <button
        type="button"
        role="checkbox"
        aria-label={option.label}
        aria-checked={indeterminate ? 'mixed' : selected}
        className={cn(
          'flex size-4 items-center justify-center rounded border border-input outline-none',
          'focus-visible:ring-2 focus-visible:ring-ring/50',
          (selected || indeterminate) && 'border-primary bg-primary text-primary-foreground',
        )}
        onClick={() => onSelect(option.id)}
      >
        <span aria-hidden="true">
          {indeterminate
            ? <PhMinus className="size-3" />
            : selected ? <PhCheck className="size-3" /> : null}
        </span>
      </button>
      <button
        type="button"
        aria-label={hasChildren ? copy.ui.scope.navigateToChildren(option.label) : undefined}
        aria-expanded={hasChildren ? active : undefined}
        className="col-span-2 grid min-w-0 grid-cols-[minmax(0,1fr)_1.5rem] items-center text-left outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
        onClick={() => hasChildren ? onNavigate(option.id) : onSelect(option.id)}
      >
        <span className="truncate">{option.label}</span>
        {hasChildren ? (
          <span className="grid size-6 place-items-center rounded-sm text-muted-foreground">
          <PhCaretRight aria-hidden="true" className="size-4" />
          </span>
        ) : null}
      </button>
    </div>
  )
}

function Column({ column, mobileActive, onSelect, onNavigate }: {
  column: Column
  mobileActive: boolean
  onSelect: (level: PMScopeLevel, id: number) => void
  onNavigate: (level: PMScopeLevel, id: number) => void
}) {
  const [query, setQuery] = useState('')
  const options = column.options.filter((option) => textMatchesQuery(option.label, query))
  return (
    <section
      aria-label={column.label}
      className={cn(
        'min-h-0 content-start gap-1 border-b border-border p-2 last:border-b-0 md:grid md:border-b-0 md:border-r md:last:border-r-0',
        mobileActive ? 'grid' : 'hidden',
      )}
    >
      <h3 className="px-2 py-1 text-xs font-medium text-muted-foreground">
        {column.label}
      </h3>
      {column.unavailable ? (
        <p className="m-0 px-2 py-3 text-xs text-muted-foreground">
          {column.unavailable}
        </p>
      ) : (
        <>
          {column.options.length > 0 ? (
            <SearchInput
              aria-label={copy.ui.scope.search(column.label)}
              placeholder={copy.ui.filter.searchPlaceholder}
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
          ) : null}
          {options.length ? (
            <div role="group" aria-label={column.label} className="grid max-h-44 gap-0.5 overflow-y-auto">
              {options.map((option) => (
                <Option
                  key={option.id}
                  option={option}
                  selected={column.selectedIds.includes(option.id)}
                  indeterminate={column.indeterminateIds?.includes(option.id) ?? false}
                  hasChildren={column.hasChildren}
                  active={column.activeId === option.id}
                  onSelect={(id) => onSelect(column.level, id)}
                  onNavigate={(id) => onNavigate(column.level, id)}
                />
              ))}
            </div>
          ) : (
            <p className="m-0 px-2 py-3 text-xs text-muted-foreground">
              {query.trim() ? copy.ui.scope.noMatches : (column.emptyLabel || copy.ui.scope.noOptions)}
            </p>
          )}
        </>
      )}
    </section>
  )
}

export function PMScopeCascaderTree({
  columns,
  onSelect,
  onNavigate,
}: {
  columns: readonly Column[]
  onSelect: (level: PMScopeLevel, id: number) => void
  onNavigate: (level: PMScopeLevel, id: number) => void
}) {
  const mobileActiveIndex = columns.length <= 2
    ? columns[0]?.activeId ? 1 : 0
    : !columns[0]?.activeId ? 0 : !columns[1]?.activeId ? 1 : 2
  return (
    <div
      data-slot="scope-cascader-tree"
      className={cn(
        'grid min-h-0 overflow-y-auto md:overflow-hidden',
        columns.length === 2 ? 'md:grid-cols-2' : 'md:grid-cols-3',
      )}
    >
      {columns.map((column, index) => (
        <Column
          key={column.level}
          column={column}
          mobileActive={index === mobileActiveIndex}
          onSelect={onSelect}
          onNavigate={onNavigate}
        />
      ))}
    </div>
  )
}

export type { Column as PMScopeCascaderColumn }
