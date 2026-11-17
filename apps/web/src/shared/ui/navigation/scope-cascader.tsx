import { useEffect, useState, type ReactNode } from 'react'
import { PhCaretDown } from '@/shared/ui/icons'

import type { PMScopeSelection } from '@/shared/api'
import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { Button, buttonVariants } from '../primitives/button'
import {
  Popover,
  PopoverClose,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
} from '../primitives/popover'
import { LoadingSkeleton } from '../feedback/loading-skeleton'
import { Label } from '../primitives/label'
import {
  resolvePMScopeCascaderModel,
  selectSinglePMScopePath,
  togglePMScopeSelection,
  togglePMScopeRequirementPath,
  type PMScopeLevel,
  type PMScopeSelectionMode,
} from './scope-cascader-model'
import {
  PMScopeCascaderTree,
} from './scope-cascader-tree'
import {
  columns,
  type PMScopeCascaderLabels,
  type PMScopeCascaderLevels,
  type PMScopeCascaderState,
} from './scope-cascader-columns'

// Import from new files for local use
import { ScopeCascader } from './scope-cascader-shell'

// Re-export from new files
export { ScopeCascader, type ScopeCascaderState } from './scope-cascader-shell'
export { type PMScopeCascaderLabels, type PMScopeCascaderLevels, type PMScopeCascaderState } from './scope-cascader-columns'
export type { PMScopeSelectionMode } from './scope-cascader-model'

export type PMScopeCascaderProps = {
  idPrefix: string
  state: PMScopeCascaderState
  value: PMScopeSelection
  labels: PMScopeCascaderLabels
  summary?: ReactNode
  className?: string
  selectionMode?: PMScopeSelectionMode
  levels?: PMScopeCascaderLevels
  presentation?: 'group' | 'control' | 'embedded'
  'aria-label'?: string
  'aria-describedby'?: string
  'aria-invalid'?: boolean
  onChange: (selection: PMScopeSelection) => void
}

/**
 * Canonical PM project-set -> project -> requirement control. Parent filtering,
 * disabled states, selection sanitizing and cascading clears live here so every
 * feature preserves the same fail-closed Router CSV semantics.
 */
export function PMScopeCascader({
  idPrefix,
  state,
  value,
  labels,
  summary,
  className,
  selectionMode = 'multiple',
  levels = 'all',
  presentation = 'group',
  'aria-label': ariaLabel,
  'aria-describedby': ariaDescribedBy,
  'aria-invalid': ariaInvalid,
  onChange,
}: PMScopeCascaderProps) {
  const [navigation, setNavigation] = useState<{
    projectSetId?: number
    projectId?: number
  }>(() => ({
    projectSetId: value.projectSetIds[0],
    projectId: value.projectIds[0],
  }))
  useEffect(() => {
    if (value.projectSetIds[0] === undefined && value.projectIds[0] === undefined) return
    setNavigation({
      projectSetId: value.projectSetIds[0],
      projectId: value.projectIds[0],
    })
  }, [value.projectIds[0], value.projectSetIds[0]])
  const description = presentation === 'group' && labels.description ? (
    <p className="m-0 text-xs text-muted-foreground">{labels.description}</p>
  ) : null
  if (state.status !== 'ready') {
    if (presentation === 'embedded') {
      return (
        <div data-slot="scope-cascader-embedded" className={cn('min-w-0 p-2', className)}>
          {state.status === 'loading' ? (
            <LoadingSkeleton rows={3} label={copy.ui.scope.loading} />
          ) : (
            <div role="alert" className="flex min-h-9 flex-wrap items-center justify-between gap-2 text-sm">
              <span>{copy.ui.scope.error}</span>
              <Button type="button" size="sm" variant="outline" onClick={state.onRetry}>
                {copy.ui.retry}
              </Button>
            </div>
          )}
        </div>
      )
    }
    if (presentation === 'control') {
      return (
        <div data-slot="scope-cascader-control" className={cn('min-w-0', className)}>
          {state.status === 'loading' ? (
            <Button
              id={`${idPrefix}-trigger`}
              type="button"
              variant="outline"
              disabled
              aria-busy="true"
              aria-label={ariaLabel}
              aria-describedby={ariaDescribedBy}
              className="w-full min-w-0 justify-start px-3 font-normal"
            >
              <span className="pm-loading-reveal">{copy.ui.scope.loading}</span>
            </Button>
          ) : (
            <div role="alert" className="flex min-h-9 flex-wrap items-center justify-between gap-2 text-sm">
              <span>{copy.ui.scope.error}</span>
              <Button type="button" size="sm" variant="outline" onClick={state.onRetry}>
                {copy.ui.retry}
              </Button>
            </div>
          )}
        </div>
      )
    }
    return (
      <div className="grid min-w-0 gap-1.5">
        {description}
        <ScopeCascader state={state} label={labels.label} className={className} />
      </div>
    )
  }

  const model = resolvePMScopeCascaderModel(state.options, value)
  const leaf: PMScopeLevel = levels === 'project-set-project' ? 'project' : 'requirement'
  const select = (level: PMScopeLevel, id: number) => {
    onChange(
      selectionMode === 'single-requirement'
        ? selectSinglePMScopePath(model, level, id)
        : level === 'requirement'
          ? togglePMScopeRequirementPath(model, id)
          : togglePMScopeSelection(model, level, id, leaf),
    )
  }

  const tree = (
    <PMScopeCascaderTree
      columns={columns(labels, model, selectionMode, levels, navigation)}
      onSelect={select}
      onNavigate={(level, id) => {
        if (level === 'projectSet') {
          setNavigation({ projectSetId: id })
          return
        }
        if (level === 'project') {
          const project = model.allOptions.projects.find((option) => option.id === id)
          setNavigation({ projectSetId: project?.parentId, projectId: id })
        }
      }}
    />
  )

  if (presentation === 'embedded') {
    return (
      <div data-slot="scope-cascader-embedded" className={cn('min-w-0', className)}>
        {tree}
      </div>
    )
  }

  const control = (
    <Popover>
      <PopoverTrigger
        id={`${idPrefix}-trigger`}
        aria-label={ariaLabel ?? (presentation === 'group' ? labels.label : undefined)}
        aria-describedby={ariaDescribedBy}
        aria-invalid={ariaInvalid || undefined}
        className={cn(
          buttonVariants({ variant: 'outline' }),
          'w-full min-w-0 justify-between px-3 text-left font-normal',
        )}
      >
        <span className="min-w-0 truncate">
          {summary ?? copy.ui.scope.open}
        </span>
        <PhCaretDown aria-hidden="true" className="size-4 text-muted-foreground" />
      </PopoverTrigger>
      <PopoverContent
        className="grid w-[min(44rem,calc(100vw-1rem))] grid-rows-[auto_minmax(0,1fr)_auto] max-h-[calc(100dvh-1rem)]"
      >
        <header className="grid gap-1 border-b border-border p-3">
          <PopoverTitle>{labels.label}</PopoverTitle>
          <PopoverDescription>
            {labels.hierarchyHint ?? copy.ui.scope.hierarchyHint}
          </PopoverDescription>
        </header>
        {tree}
        <footer className="flex items-center justify-between gap-2 border-t border-border p-2">
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={() => onChange({
              projectSetIds: [],
              projectIds: [],
              requirementIds: [],
            })}
          >
            {copy.ui.scope.reset}
          </Button>
          <PopoverClose className={buttonVariants({ size: 'sm' })}>
            {copy.ui.scope.done}
          </PopoverClose>
        </footer>
      </PopoverContent>
    </Popover>
  )

  if (presentation === 'control') {
    return (
      <div data-slot="scope-cascader-control" className={cn('min-w-0', className)}>
        {control}
      </div>
    )
  }

  return (
    <div className="grid min-w-0 gap-1.5">
      {description}
      <ScopeCascader
        state={{ status: 'ready' }}
        label={labels.label}
        className={className}
        layout="single"
      >
        {control}
      </ScopeCascader>
    </div>
  )
}

export function PMScopeCascaderField({
  error,
  ...props
}: Omit<PMScopeCascaderProps, 'presentation' | 'aria-describedby' | 'aria-invalid'> & {
  error?: ReactNode
}) {
  const descriptionId = props.labels.description ? `${props.idPrefix}-description` : undefined
  const errorId = error ? `${props.idPrefix}-error` : undefined
  const describedBy = [descriptionId, errorId].filter(Boolean).join(' ') || undefined

  return (
    <div data-slot="scope-cascader-field" className="grid min-w-0 gap-1.5">
      <Label htmlFor={`${props.idPrefix}-trigger`}>{props.labels.label}</Label>
      <PMScopeCascader
        {...props}
        presentation="control"
        aria-describedby={describedBy}
        aria-invalid={Boolean(error)}
      />
      {props.labels.description ? (
        <p id={descriptionId} className="m-0 text-xs text-muted-foreground">
          {props.labels.description}
        </p>
      ) : null}
      {error ? (
        <p id={errorId} role="alert" className="m-0 text-xs text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  )
}