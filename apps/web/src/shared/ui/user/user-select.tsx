import { USER_SEARCH, type SearchOptions } from '@/shared/lib/search'
import { useDeferredValue, useMemo, useRef, useState } from 'react'

import { cn } from '@/shared/lib/utils'
import { copy } from '@/shared/copy'

import {
  Combobox,
  ComboboxClear,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxInputGroup,
  ComboboxItem,
  ComboboxList,
  ComboboxTrigger,
} from '../primitives/combobox'
import { UserPreview } from './user-preview'
import {
  USER_SELECT_VIRTUALIZE_THRESHOLD,
  VirtualizedComboboxItems,
  scrollHighlightedItemIntoView,
  type ComboboxVirtualizerRef,
} from './virtualized-combobox-items'
import {
  type UserSelectOption,
  textMatchesQuery,
  userSelectOptionMatches,
} from './user-select-option'

export type { UserSelectOption } from './user-select-option'

export type UserSelectProps = {
  id?: string
  label: string
  value?: number
  options: readonly UserSelectOption[]
  emptyLabel?: string
  noResultsLabel?: string
  openLabel?: string
  placeholder?: string
  disabled?: boolean
  defaultOpen?: boolean
  clearable?: boolean
  className?: string
  contentClassName?: string
  'aria-describedby'?: string
  'aria-invalid'?: boolean
  onValueChange: (value?: number) => void
  onOpenChange?: (open: boolean) => void
  search?: SearchOptions
  onQueryChange?: (query: string) => void
  onLoadMore?: () => void
  hasMore?: boolean
  loadingMore?: boolean
}

type UserSelectChoice =
  | { kind: 'empty'; label: string }
  | { kind: 'user'; option: UserSelectOption }

function choiceLabel(choice: UserSelectChoice): string {
  return choice.kind === 'empty' ? choice.label : choice.option.name
}

function choiceEquals(left: UserSelectChoice, right: UserSelectChoice): boolean {
  if (left.kind !== right.kind) return false
  return left.kind === 'empty' || (
    right.kind === 'user' && left.option.id === right.option.id
  )
}

function userSelectChoiceMatches(
  choice: UserSelectChoice,
  query: string,
  search: SearchOptions,
  itemToString?: (itemValue: UserSelectChoice) => string,
): boolean {
  if (choice && typeof choice === 'object' && 'kind' in choice) {
    if (choice.kind === 'empty') return textMatchesQuery(choice.label, query, search)
    if (userSelectOptionMatches(choice.option, query, undefined, search)) return true
  }
  const label = itemToString?.(choice) ?? (choice && typeof choice === 'object' && 'kind' in choice
    ? choiceLabel(choice)
    : '')
  return textMatchesQuery(label, query, search)
}

export function UserSelect({
  id,
  label,
  value,
  options,
  emptyLabel,
  noResultsLabel = copy.ui.userSelect.noMatches,
  openLabel,
  placeholder,
  disabled = false,
  defaultOpen = false,
  clearable = false,
  className,
  contentClassName,
  'aria-describedby': ariaDescribedBy,
  'aria-invalid': ariaInvalid,
  onValueChange,
  onOpenChange,
  search = USER_SEARCH,
  onQueryChange,
  onLoadMore,
  hasMore = false,
  loadingMore = false,
}: UserSelectProps) {
  const [open, setOpen] = useState(defaultOpen && !disabled)
  const [query, setQuery] = useState('')
  const virtualizerRef = useRef<ComboboxVirtualizerRef | null>(null)
  const shouldVirtualize = options.length > USER_SELECT_VIRTUALIZE_THRESHOLD
  const selected = options.find((option) => option.id === value)
  const items = useMemo<UserSelectChoice[]>(() => [
    ...(emptyLabel ? [{ kind: 'empty' as const, label: emptyLabel }] : []),
    ...options.map((option) => ({ kind: 'user' as const, option })),
  ], [emptyLabel, options])
  const selectedChoice = useMemo<UserSelectChoice | null>(() => selected
    ? { kind: 'user', option: selected }
    : null, [selected])
  const deferredQuery = useDeferredValue(query)
  const filteredItems = useMemo(
    () => onQueryChange
      ? items
      : items.filter((choice) => userSelectChoiceMatches(choice, deferredQuery, search)),
    [deferredQuery, items, onQueryChange, search],
  )
  return (
    <Combobox<UserSelectChoice>
      open={open}
      value={selectedChoice}
      items={items}
      disabled={disabled}
      openOnInputClick
      virtualized={shouldVirtualize}
      itemToStringLabel={choiceLabel}
      isItemEqualToValue={choiceEquals}
      filter={onQueryChange ? null : undefined}
      filteredItems={onQueryChange ? undefined : filteredItems}
      onItemHighlighted={(_item, eventDetails) => {
        if (shouldVirtualize) {
          scrollHighlightedItemIntoView(virtualizerRef.current, eventDetails.index, eventDetails.reason)
        }
      }}
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen)
        if (!nextOpen) {
          setQuery('')
          onQueryChange?.('')
        }
        onOpenChange?.(nextOpen)
      }}
      onInputValueChange={(nextQuery: string, details: { reason: string }) => {
        if (details.reason === 'item-press') return
        setQuery(nextQuery)
        onQueryChange?.(nextQuery)
      }}
      onValueChange={(next, eventDetails) => {
        // Base UI reports null both when an untouched popup is dismissed and
        // when the user deletes the selected input text. Only input-clear is
        // an explicit clear intent; dismissing the popup must remain a no-op.
        if (!next) {
          if (eventDetails.reason === 'input-clear' || eventDetails.reason === 'clear-press') {
            onValueChange(undefined)
          }
          return
        }
        onValueChange(next.kind === 'user' ? next.option.id : undefined)
      }}
    >
      <ComboboxInputGroup className={cn('w-full', className)}>
        {selected ? (
          <UserPreview
            aria-hidden="true"
            name={selected.name}
            avatarUrl={selected.avatarUrl}
            size="sm"
            className="pl-2 [&>span:last-child]:hidden"
          />
        ) : null}
        <ComboboxInput
          autoFocus={defaultOpen && !disabled}
          id={id}
          aria-label={label}
          aria-describedby={ariaDescribedBy}
          aria-invalid={ariaInvalid || undefined}
          disabled={disabled}
          placeholder={placeholder ?? emptyLabel}
          className={cn(clearable && selected && !disabled && 'pr-14')}
          onClick={() => { if (!disabled) setOpen(true) }}
        />
        {clearable && selected && !disabled ? (
          <ComboboxClear aria-label={copy.ui.clearSelection} />
        ) : null}
        <ComboboxTrigger
          aria-label={openLabel ?? copy.ui.userSelect.openOptions(label)}
          disabled={disabled}
        />
      </ComboboxInputGroup>
      <ComboboxContent className={cn('min-w-56', contentClassName /* eslint-disable-line shadcn/require-static-classes -- pass-through of the UserSelect contentClassName prop; the rule cannot trace non-className parameters */)}>
        <ComboboxEmpty>{noResultsLabel}</ComboboxEmpty>
        {shouldVirtualize ? (
          <ComboboxList className="max-h-none overflow-visible p-0">
            <VirtualizedComboboxItems<UserSelectChoice>
              virtualizerRef={virtualizerRef}
              onNearEnd={hasMore ? onLoadMore : undefined}
              nearEndBusy={loadingMore}
              renderItem={(choice, virtualItem, virtualizer) => choice.kind === 'empty' ? (
                <ComboboxItem
                  key="empty"
                  index={virtualItem.index}
                  value={choice}
                  ref={virtualizer.measureElement}
                  // Base UI virtualized pattern: absolute row; size/offset come
                  // from the `--combobox-item-*` custom properties in `style`.
                  className="absolute top-0 left-0 w-full h-(--combobox-item-height) translate-y-(--combobox-item-top)"
                  style={{
                    '--combobox-item-height': `${virtualItem.size}px`,
                    '--combobox-item-top': `${virtualItem.start}px`,
                  }}
                >{choice.label}</ComboboxItem>
              ) : (
                <ComboboxItem
                  key={choice.option.id}
                  index={virtualItem.index}
                  value={choice}
                  disabled={choice.option.disabled}
                  ref={virtualizer.measureElement}
                  // Base UI virtualized pattern: absolute row; size/offset come
                  // from the `--combobox-item-*` custom properties in `style`.
                  className="absolute top-0 left-0 w-full h-(--combobox-item-height) translate-y-(--combobox-item-top)"
                  style={{
                    '--combobox-item-height': `${virtualItem.size}px`,
                    '--combobox-item-top': `${virtualItem.start}px`,
                  }}
                >
                  <UserPreview
                    name={choice.option.name}
                    secondary={choice.option.secondary ?? choice.option.username}
                    avatarUrl={choice.option.avatarUrl}
                    size="md"
                  />
                </ComboboxItem>
              )}
            />
          </ComboboxList>
        ) : (
          <ComboboxList
            aria-busy={loadingMore || undefined}
            onScroll={(event) => {
              const list = event.currentTarget
              if (hasMore && !loadingMore && list.scrollHeight - list.scrollTop - list.clientHeight <= 32) {
                onLoadMore?.()
              }
            }}
          >
            {(choice: UserSelectChoice) => choice.kind === 'empty' ? (
              <ComboboxItem key="empty" value={choice}>{choice.label}</ComboboxItem>
            ) : (
              <ComboboxItem
                key={choice.option.id}
                value={choice}
                disabled={choice.option.disabled}
              >
                <UserPreview
                  name={choice.option.name}
                  secondary={choice.option.secondary ?? choice.option.username}
                  avatarUrl={choice.option.avatarUrl}
                  size="md"
                />
              </ComboboxItem>
            )}
          </ComboboxList>
        )}
        {loadingMore ? (
          <div role="status" className="border-t border-border px-3 py-2 text-center text-xs text-muted-foreground">
            {copy.ui.userSelect.loadingMore}
          </div>
        ) : null}
      </ComboboxContent>
    </Combobox>
  )
}
