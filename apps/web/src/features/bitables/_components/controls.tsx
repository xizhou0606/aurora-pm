import type { ComponentProps } from 'react'
import { Input, Textarea, SelectTrigger, SelectContent } from '@/shared/ui'
import { cn } from '@/shared/lib/utils'

// Dense grid controls use the existing border/caret and a subtle focus background,
// not an additional outer ring that competes with the cell or popover boundary.
const focusStyle = 'shadow-none focus-visible:ring-0 focus-visible:ring-offset-0 focus-visible:bg-muted/30'

export function BitableInput({ className, ...props }: ComponentProps<typeof Input>) {
  return <Input {...props} className={cn(focusStyle, className)} />
}

export function BitableTextarea({ className, ...props }: ComponentProps<typeof Textarea>) {
  return <Textarea {...props} className={cn(focusStyle, className)} />
}

export function BitableSelectTrigger({ className, ...props }: ComponentProps<typeof SelectTrigger>) {
  return <SelectTrigger {...props} className={cn('h-8 w-36 min-w-0 shrink-0 px-2 text-xs shadow-none', className)} />
}

export function BitableSelectContent({ className, ...props }: ComponentProps<typeof SelectContent>) {
  return <SelectContent {...props} className={cn('min-w-40 text-xs [&_[data-slot=select-item]]:text-xs', className)} />
}
