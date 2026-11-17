import type { SearchOptions } from '@/shared/lib/search'
import { copy } from '@/shared/copy'

import { Button } from '../primitives/button'
import { Label } from '../primitives/label'
import { MultiUserSelect } from './multi-user-select'
import { UserSelect } from './user-select'
import type { UserSelectOption } from './user-select-option'

export type UserDirectoryItem = {
  id: number
  name?: string
  label?: string
  username?: string
  secondary?: string
  avatarUrl?: string
  disabled?: boolean
}

export type UserDirectoryState =
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | { status: 'empty' }
  | {
      status: 'ready'
      items: readonly UserDirectoryItem[]
      onQueryChange?: (query: string) => void
      onLoadMore?: () => void
      hasMore?: boolean
      loadingMore?: boolean
    }

export type UserDirectorySelectProps = {
  id: string
  label: string
  value?: number | null
  state: UserDirectoryState
  search?: SearchOptions
  disabled?: boolean
  error?: string
  onValueChange: (value?: number) => void
}

export type UserDirectoryMultiSelectProps = {
  id: string
  label: string
  value: readonly number[]
  state: UserDirectoryState
  search?: SearchOptions
  description?: string
  placeholder?: string
  disabled?: boolean
  closeOnSelect?: boolean
  selectionDisplay?: 'chips' | 'inline'
  clearable?: boolean
  onValueChange: (value: number[]) => void
}

function directoryOptions(state: UserDirectoryState): UserSelectOption[] {
  if (state.status !== 'ready') return []
  const options = new Map<number, UserSelectOption>()
  for (const item of state.items) {
    const id = Number(item.id)
    if (!Number.isInteger(id) || id <= 0 || options.has(id)) continue
    options.set(id, {
      id,
      name: item.name?.trim() || item.label?.trim() || copy.ui.userSelect.fallback(id),
      username: item.username,
      secondary: item.secondary,
      avatarUrl: item.avatarUrl,
      disabled: item.disabled,
    })
  }
  return [...options.values()]
}

export function UserDirectorySelect({
  id,
  label,
  value,
  state,
  search,
  disabled = false,
  error,
  onValueChange,
}: UserDirectorySelectProps) {
  const selectedId = value ?? undefined
  const directory = directoryOptions(state)
  const selected = selectedId === undefined
    ? undefined
    : directory.find((item) => item.id === selectedId)
  const options: UserSelectOption[] = [
    ...(selectedId !== undefined && !selected ? [{
      id: selectedId,
      name: copy.ui.userSelect.fallback(selectedId),
      disabled: true,
    }] : []),
    ...directory,
  ]
  const historicalValueCanClear = selectedId !== undefined && state.status !== 'loading'
  const controlDisabled = disabled || (state.status !== 'ready' && !historicalValueCanClear)
  const statusId = state.status === 'ready' ? undefined : `${id}-directory-status`
  const errorId = error ? `${id}-error` : undefined
  const describedBy = [statusId, errorId].filter(Boolean).join(' ') || undefined

  return (
    <div data-slot="user-directory-select" className="grid gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <UserSelect
        search={search}
        id={id}
        label={label}
        value={selectedId}
        options={options}
        emptyLabel={copy.ui.userSelect.clear}
        noResultsLabel={copy.ui.userSelect.noMatches}
        placeholder={copy.ui.userSelect.placeholder}
        disabled={controlDisabled}
        aria-invalid={Boolean(error) || undefined}
        aria-describedby={describedBy}
        onValueChange={(next) => { if (!controlDisabled) onValueChange(next) }}
        onQueryChange={state.status === 'ready' ? state.onQueryChange : undefined}
        onLoadMore={state.status === 'ready' ? state.onLoadMore : undefined}
        hasMore={state.status === 'ready' ? state.hasMore : false}
        loadingMore={state.status === 'ready' ? state.loadingMore : false}
      />

      {state.status === 'loading' ? (
        <p
          id={statusId}
          role="status"
          aria-live="polite"
          className="pm-loading-reveal text-xs text-muted-foreground"
        >
          {copy.ui.userSelect.loading}
        </p>
      ) : null}
      {state.status === 'error' ? (
        <div id={statusId} role="alert" className="flex flex-wrap items-center gap-2 text-xs text-destructive">
          <span>{copy.ui.userSelect.loadFailed}</span>
          <Button type="button" size="xs" variant="outline" disabled={disabled} onClick={state.retry}>
            {copy.ui.userSelect.retry}
          </Button>
        </div>
      ) : null}
      {state.status === 'empty' ? (
        <p id={statusId} role="status" aria-live="polite" className="text-xs text-muted-foreground">
          {copy.ui.userSelect.empty}
        </p>
      ) : null}
      {error ? <p id={errorId} role="alert" className="text-xs text-destructive">{error}</p> : null}
    </div>
  )
}

export function UserDirectoryMultiSelect({
  id,
  label,
  value,
  state,
  search,
  description,
  placeholder = copy.ui.userSelect.placeholder,
  disabled = false,
  closeOnSelect = false,
  selectionDisplay = 'chips',
  clearable = false,
  onValueChange,
}: UserDirectoryMultiSelectProps) {
  const directory = directoryOptions(state)
  const directoryIds = new Set(directory.map((item) => item.id))
  const options: UserSelectOption[] = [
    ...value
      .filter((id) => !directoryIds.has(id))
      .map((id) => ({
        id,
        name: copy.ui.userSelect.fallback(id),
        disabled: true,
      })),
    ...directory,
  ]
  const descriptionId = description ? `${id}-description` : undefined

  return (
    <div data-slot="user-directory-multi-select" className="grid gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      {state.status === 'ready' ? (
        <MultiUserSelect
          search={search}
          id={id}
          label={label}
          value={options.filter((item) => value.includes(item.id))}
          options={options}
          placeholder={placeholder}
          disabled={disabled}
          closeOnSelect={closeOnSelect}
          selectionDisplay={selectionDisplay}
          clearable={clearable}
          aria-describedby={descriptionId}
          onValueChange={(next) => onValueChange(next.map((item) => item.id))}
          onQueryChange={state.onQueryChange}
          onLoadMore={state.onLoadMore}
          hasMore={state.hasMore}
          loadingMore={state.loadingMore}
        />
      ) : state.status === 'error' ? (
        <div role="alert" className="flex flex-wrap items-center gap-2 text-xs text-destructive">
          <span>{copy.ui.userSelect.loadFailed}</span>
          <Button type="button" size="xs" variant="outline" disabled={disabled} onClick={state.retry}>
            {copy.ui.userSelect.retry}
          </Button>
        </div>
      ) : (
        <p
          role="status"
          aria-live="polite"
          className={
            state.status === 'loading'
              ? 'pm-loading-reveal text-xs text-muted-foreground'
              : 'text-xs text-muted-foreground'
          }
        >
          {state.status === 'loading' ? copy.ui.userSelect.loading : copy.ui.userSelect.empty}
        </p>
      )}
      {description ? <p id={descriptionId} className="text-xs text-muted-foreground">{description}</p> : null}
    </div>
  )
}
