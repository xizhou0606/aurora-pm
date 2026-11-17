import { Tabs } from '@base-ui/react/tabs'
import type { ReactNode } from 'react'

import { cn } from '@/shared/lib/utils'

export type ViewTab = {
  id: string
  label: ReactNode
  secondaryLabel?: ReactNode
  disabled?: boolean
}

export function ViewTabs({
  value,
  tabs,
  onValueChange,
  label,
  className,
  density = 'default',
  lines = 1,
}: {
  value: string
  tabs: readonly ViewTab[]
  onValueChange: (value: string) => void
  label: string
  className?: string
  density?: 'default' | 'compact'
  lines?: 1 | 2
}) {
  return (
    <Tabs.Root value={value} onValueChange={onValueChange}>
      <Tabs.List
        aria-label={label}
        className={cn(
          'relative isolate inline-flex min-w-0 items-center gap-1 rounded-lg bg-muted/60',
          density === 'compact' ? 'p-0.5' : 'p-1',
          lines === 2 && 'items-stretch',
          className,
        )}
      >
        <Tabs.Indicator
          data-slot="view-tabs-indicator"
          className="pointer-events-none absolute z-0 rounded-lg bg-card shadow-[var(--design-shadow-card)] ring-1 ring-border/60 transition-[left,top,width,height] duration-200 ease-out motion-reduce:transition-none left-[var(--active-tab-left)] top-[var(--active-tab-top)] h-[var(--active-tab-height)] w-[var(--active-tab-width)] dark:bg-card dark:ring-border dark:shadow-none"
        />
        {tabs.map((tab) => (
          <Tabs.Tab
            key={tab.id}
            value={tab.id}
            disabled={tab.disabled}
            aria-label={typeof tab.label === 'string' ? tab.label : undefined}
            className={cn(
              'relative z-10 min-w-0 rounded-lg text-sm text-muted-foreground outline-none transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring data-active:text-foreground dark:data-active:text-white disabled:opacity-50',
              lines === 1 && (density === 'compact' ? 'px-2.5 py-1' : 'px-3 py-1.5'),
              lines === 2 && 'grid min-w-20 content-center items-start gap-0.5 px-3 py-2 text-left',
            )}
          >
            <span className={cn('truncate', lines === 2 && 'w-full font-medium leading-5')}>{tab.label}</span>
            {lines === 2 ? (
              <span
                aria-hidden={tab.secondaryLabel == null ? true : undefined}
                className="w-full truncate text-xs leading-4 text-muted-foreground"
              >
                {tab.secondaryLabel ?? '\u00a0'}
              </span>
            ) : null}
          </Tabs.Tab>
        ))}
      </Tabs.List>
    </Tabs.Root>
  )
}
