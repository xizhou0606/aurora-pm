import type { ReactNode } from 'react'
import { Button } from '@/shared/ui'
import type { IconComponent } from '@/shared/ui/icons'
import { cn } from '@/shared/lib/utils'

export type ResourceToolbarAction = {
  key: string
  label: string
  icon: IconComponent
  onClick: () => void
  visible?: boolean
  disabled?: boolean
  text?: boolean
  count?: number
}

/** Resource-level affordances only; each renderer owns its capabilities and workflows. */
export function ResourceToolbar({ actions, leading, trailing, className }: {
  actions: readonly ResourceToolbarAction[]
  leading?: ReactNode
  trailing?: ReactNode
  className?: string
}) {
  return <div data-slot="resource-toolbar" className={cn('pointer-events-auto flex shrink-0 items-center gap-1', className)}>
    {leading}
    {actions.filter(action => action.visible !== false).map(({ key, label, icon: Icon, onClick, disabled, text, count }) => (
      <Button key={key} type="button" variant="outline" size="sm" aria-label={label} title={label}
        className="shrink-0 cursor-pointer border-border/60 text-xs hover:bg-accent hover:text-accent-foreground active:bg-accent disabled:cursor-not-allowed"
        disabled={disabled} onClick={onClick}>
        <Icon aria-hidden="true" />
        {text ? label : null}
        {count !== undefined && count > 0 ? <span className="tabular-nums">{count}</span> : null}
      </Button>
    ))}
    {trailing}
  </div>
}
