import * as React from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { ICON_CONTEXT_SIZES, Icon, type GeneratedIconName } from '../icons'

import {
  Empty,
  EmptyContent,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from './empty'

type EmptyStateProps = Omit<React.ComponentProps<'div'>, 'title'> & {
  title?: React.ReactNode
  /** Opt-in custom icon node; wins over `name`. */
  icon?: React.ReactNode
  /** Opt-in icon-system glyph; defaults to the registry's empty-state glyph. */
  name?: GeneratedIconName
  action?: React.ReactNode
}

/** Default Rounded glyph, matching registry.json emptyStates. */
const EMPTY_STATE_DEFAULT_ICON: GeneratedIconName = 'hugeicons:aspect-ratio'

/**
 * Shared empty state for tables, lists, cards, and panels (AGENTS.md Frontend).
 * Deliberately minimal: an icon plus one title line — no description text.
 */
function EmptyState({
  title = copy.ui.empty.title,
  icon,
  name = EMPTY_STATE_DEFAULT_ICON,
  action,
  className,
  ...props
}: EmptyStateProps) {
  return (
    <Empty
      data-slot="empty-state"
      className={cn('min-h-32 gap-2 rounded-lg p-4 py-6 md:p-6', className)}
      {...props}
    >
      <EmptyMedia
        variant="icon"
        className="size-14 rounded-none bg-transparent text-muted-foreground [&_svg:not([class*='size-'])]:size-10"
      >
        {icon ?? (
          <Icon name={name} size={ICON_CONTEXT_SIZES.emptyMedium} aria-hidden="true" />
        )}
      </EmptyMedia>
      <EmptyHeader className="max-w-md">
        <EmptyTitle role="heading" aria-level={3} className="text-sm font-medium text-foreground">
          {title}
        </EmptyTitle>
      </EmptyHeader>
      {action ? (
        <EmptyContent className="max-w-none">
          <div
            data-slot="empty-state-action"
            className="flex flex-wrap items-center justify-center gap-2"
          >
            {action}
          </div>
        </EmptyContent>
      ) : null}
    </Empty>
  )
}

export { EmptyState }
export type { EmptyStateProps }
