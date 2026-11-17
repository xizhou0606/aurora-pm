import { searchFieldsMatch, USER_SEARCH } from '@/shared/lib/search'
import { Button, UserPreview } from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { useMemo } from 'react'

import type { WorkItemUserOption } from '../../model/types'

export function filterMentionOptions(
  options: readonly WorkItemUserOption[],
  query: string,
): readonly WorkItemUserOption[] {
  const normalized = query.trim().toLocaleLowerCase()
  return options.filter((option) => {
    if (!option.username) return false
    if (!normalized) return true
    return searchFieldsMatch([option.label, option.username], normalized, USER_SEARCH)
  })
}

export function MentionCombobox({
  options,
  query,
  activeIndex,
  onActiveIndexChange,
  onSelect,
}: {
  options: readonly WorkItemUserOption[]
  query: string
  activeIndex: number
  onActiveIndexChange: (index: number) => void
  onSelect: (user: WorkItemUserOption) => void
}) {
  const filtered = useMemo(() => filterMentionOptions(options, query), [options, query])

  return (
    <div
      role="listbox"
      id="work-items-comment-mention-options"
      aria-label={WORK_ITEMS_COPY.detail.mentionOptions}
      className="absolute inset-x-2 bottom-full z-20 mb-1 max-h-52 overflow-y-auto rounded-lg border border-border bg-popover p-1 text-popover-foreground shadow-lg"
    >
      {filtered.length ? filtered.map((option, index) => (
        <Button
          key={option.id}
          type="button"
          role="option"
          aria-selected={index === activeIndex}
          tabIndex={-1}
          variant="ghost"
          className="h-auto w-full justify-start px-2 py-1.5 data-[active=true]:bg-muted"
          data-active={index === activeIndex}
          onPointerMove={() => onActiveIndexChange(index)}
          onPointerDown={(event) => {
            event.preventDefault()
            onSelect(option)
          }}
        >
          <UserPreview
            name={option.label}
            secondary={option.username ? `@${option.username}` : undefined}
            avatarUrl={option.avatarUrl}
            size="md"
          />
        </Button>
      )) : (
        <p role="status" className="m-0 px-2 py-3 text-sm text-muted-foreground">
          {WORK_ITEMS_COPY.detail.noMentionMatches}
        </p>
      )}
    </div>
  )
}
