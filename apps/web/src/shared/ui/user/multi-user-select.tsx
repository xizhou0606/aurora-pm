import { USER_SEARCH, type SearchOptions } from '@/shared/lib/search'
import { PhX } from '@/shared/ui/icons'
import { Fragment, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react'
import { Tooltip } from '@base-ui/react/tooltip'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

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
import { useOverlayPortalContainer } from '../misc/overlay-portal-container'
import { UserPreview } from './user-preview'
import {
  USER_SELECT_VIRTUALIZE_THRESHOLD,
  VirtualizedComboboxItems,
  scrollHighlightedItemIntoView,
  type ComboboxVirtualizerRef,
} from './virtualized-combobox-items'
import {
  type UserSelectOption,
  userSelectOptionEquals,
  userSelectOptionMatches,
} from './user-select-option'

export type MultiUserSelectProps = {
  id?: string
  label: string
  value: readonly UserSelectOption[]
  options: readonly UserSelectOption[]
  noResultsLabel?: string
  openLabel?: string
  placeholder?: string
  disabled?: boolean
  defaultOpen?: boolean
  clearable?: boolean
  closeOnSelect?: boolean
  selectionDisplay?: 'chips' | 'inline'
  className?: string
  contentClassName?: string
  'aria-describedby'?: string
  'aria-invalid'?: boolean
  onValueChange: (value: UserSelectOption[]) => void
  onOpenChange?: (open: boolean) => void
  search?: SearchOptions
  onQueryChange?: (query: string) => void
  onLoadMore?: () => void
  hasMore?: boolean
  loadingMore?: boolean
}

function userOptionLabel(option: UserSelectOption): string {
  return option.name
}

export function MultiUserSelect({
  id,
  label,
  value,
  options,
  noResultsLabel = copy.ui.userSelect.noMatches,
  openLabel,
  placeholder,
  disabled = false,
  defaultOpen = false,
  clearable = false,
  closeOnSelect = false,
  selectionDisplay = 'chips',
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
}: MultiUserSelectProps) {
  const [open, setOpen] = useState(defaultOpen && !disabled)
  const [query, setQuery] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const virtualizerRef = useRef<ComboboxVirtualizerRef | null>(null)
  const shouldVirtualize = options.length > USER_SELECT_VIRTUALIZE_THRESHOLD
  const portalContainer = useOverlayPortalContainer()
  const deferredQuery = useDeferredValue(query)
  const items = useMemo(() => [...options], [options])
  const selectedValue = useMemo(() => [...value], [value])
  const filteredItems = useMemo(
    () => onQueryChange
      ? items
      : items.filter((option) => userSelectOptionMatches(option, deferredQuery, undefined, search)),
    [deferredQuery, items, onQueryChange, search],
  )
  const inlineSelection = selectionDisplay === 'inline' && value.length
    ? value.map((option) => option.name).join('、')
    : undefined

  useEffect(() => {
    if (open) inputRef.current?.focus()
  }, [open])

  return (
    <Combobox<UserSelectOption, true>
      multiple
      open={open}
      value={selectedValue}
      items={items}
      disabled={disabled}
      openOnInputClick
      virtualized={shouldVirtualize}
      itemToStringLabel={userOptionLabel}
      isItemEqualToValue={userSelectOptionEquals}
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
      onValueChange={(next) => {
        onValueChange(next)
        if (closeOnSelect) setOpen(false)
      }}
    >
      <div className="grid gap-1.5">
        <ComboboxInputGroup className={cn('h-8 min-h-8 w-full', className)}>
          {inlineSelection && !open ? (
            <Tooltip.Root>
              <Tooltip.Trigger
                type="button"
                tabIndex={-1}
                delay={150}
                aria-label={copy.ui.userSelect.selectedOptions(label, inlineSelection)}
                className={cn(
                  'absolute inset-y-0 left-0 z-1 flex min-w-0 cursor-pointer items-center overflow-hidden px-2.5 py-1 text-left outline-none',
                  clearable && !disabled ? 'right-14' : 'right-8',
                )}
                onClick={() => { if (!disabled) setOpen(true) }}
              >
                <span className="flex min-w-0 items-center gap-1">
                  {value.map((option, index) => (
                    <Fragment key={option.id}>
                      {index > 0 ? (
                        <span aria-hidden="true" className="text-xs text-muted-foreground">
                          {copy.ui.userFaceStack.nameSeparator}
                        </span>
                      ) : null}
                      <UserPreview
                        name={option.name}
                        avatarUrl={option.avatarUrl}
                        size="sm"
                        className="shrink-0 [&>span:last-child]:max-w-28"
                      />
                    </Fragment>
                  ))}
                </span>
              </Tooltip.Trigger>
              <Tooltip.Portal container={portalContainer ?? undefined}>
                <Tooltip.Positioner
                  side="bottom"
                  sideOffset={6}
                  className="z-80"
                  positionMethod="fixed"
                >
                  <Tooltip.Popup className="max-h-64 w-max min-w-48 max-w-72 overflow-y-auto rounded-lg border border-border bg-popover text-popover-foreground shadow-[var(--design-shadow-popover)] outline-none">
                    <p className="sticky top-0 z-1 m-0 border-b border-border bg-popover px-3 py-2 text-xs font-medium text-muted-foreground">
                      {copy.ui.userSelect.selected}
                    </p>
                    <ul className="m-0 grid list-none gap-1 p-2">
                      {value.map((option) => (
                        <li key={option.id} className="rounded-sm px-1.5 py-1">
                          <UserPreview
                            name={option.name}
                            secondary={option.secondary ?? option.username}
                            avatarUrl={option.avatarUrl}
                            size="sm"
                          />
                        </li>
                      ))}
                    </ul>
                  </Tooltip.Popup>
                </Tooltip.Positioner>
              </Tooltip.Portal>
            </Tooltip.Root>
          ) : null}
          <ComboboxInput
            ref={inputRef}
            id={id}
            aria-label={label}
            aria-describedby={ariaDescribedBy}
            aria-invalid={ariaInvalid || undefined}
            disabled={disabled}
            placeholder={inlineSelection && !open ? undefined : placeholder}
            className={cn(
              'min-w-0 flex-1 px-2.5',
              clearable && value.length && !disabled && 'pr-14',
            )}
            onClick={() => { if (!disabled) setOpen(true) }}
          />
          {clearable && value.length && !disabled ? (
            <ComboboxClear aria-label={copy.ui.clearSelection} />
          ) : null}
          <ComboboxTrigger
            aria-label={openLabel ?? copy.ui.userSelect.openOptions(label)}
            disabled={disabled}
          />
        </ComboboxInputGroup>
        {selectionDisplay === 'chips' && value.length ? (
          <ul
            data-slot="multi-user-value-row"
            className="m-0 flex list-none flex-wrap gap-1 p-0"
          >
            {value.map((option) => (
              <li
                key={option.id}
                className="inline-flex min-w-0 max-w-full items-center gap-1 rounded-md border border-border bg-muted px-1.5 py-0.5"
              >
                <UserPreview
                  name={option.name}
                  avatarUrl={option.avatarUrl}
                  size="sm"
                  className="min-w-0 [&>span:last-child]:max-w-32"
                />
                <button
                  type="button"
                  aria-label={copy.ui.userSelect.removeOption(option.name)}
                  disabled={disabled}
                  className="grid size-4 shrink-0 place-items-center rounded-sm text-muted-foreground outline-none hover:bg-background hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
                  onClick={(event) => {
                    event.stopPropagation()
                    onValueChange(value.filter((entry) => entry.id !== option.id))
                  }}
                >
                  <PhX aria-hidden="true" className="size-3" />
                </button>
              </li>
            ))}
          </ul>
        ) : null}
      </div>
      <ComboboxContent className={cn('min-w-56', contentClassName /* eslint-disable-line shadcn/require-static-classes -- pass-through of the MultiUserSelect contentClassName prop; the rule cannot trace non-className parameters */)}>
        <ComboboxEmpty>{noResultsLabel}</ComboboxEmpty>
        {shouldVirtualize ? (
          <ComboboxList className="max-h-none overflow-visible p-0">
            <VirtualizedComboboxItems<UserSelectOption>
              virtualizerRef={virtualizerRef}
              onNearEnd={hasMore ? onLoadMore : undefined}
              nearEndBusy={loadingMore}
              renderItem={(option, virtualItem, virtualizer) => (
                <ComboboxItem
                  key={option.id}
                  index={virtualItem.index}
                  value={option}
                  disabled={option.disabled}
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
                    name={option.name}
                    secondary={option.secondary ?? option.username}
                    avatarUrl={option.avatarUrl}
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
            {(option: UserSelectOption) => (
              <ComboboxItem key={option.id} value={option} disabled={option.disabled}>
                <UserPreview
                  name={option.name}
                  secondary={option.secondary ?? option.username}
                  avatarUrl={option.avatarUrl}
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
