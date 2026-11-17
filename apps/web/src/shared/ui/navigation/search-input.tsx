import { PhMagnifyingGlass } from '@/shared/ui/icons'
import * as React from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

import { ClearControlButton } from '../edit/clear-control-button'
import { Input } from '../primitives/input'

type SearchInputProps = React.ComponentProps<typeof Input> & {
  clearable?: boolean
  clearLabel?: string
  onClear?: () => void
}

function SearchInput({
  className,
  clearable = false,
  clearLabel = copy.ui.clearSelection,
  onClear,
  value,
  ...props
}: SearchInputProps) {
  const canClear = clearable && String(value ?? '').length > 0 && !props.disabled
  return (
    <div data-slot="search-input" className="relative min-w-0">
      <PhMagnifyingGlass
        aria-hidden="true"
        className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground"
      />
      <Input
        type="search"
        value={value}
        className={cn('pm-search-input pl-8', canClear && 'pr-9', className)}
        {...props}
      />
      {canClear ? (
        <ClearControlButton
          label={clearLabel}
          className="right-1"
          onClick={() => onClear?.()}
        />
      ) : null}
    </div>
  )
}

export { SearchInput }
export type { SearchInputProps }
