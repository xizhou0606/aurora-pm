import { Checkbox as CheckboxPrimitive } from '@base-ui/react/checkbox'
import { PhCheck, PhMinus } from '@/shared/ui/icons'

import { cn } from '@/shared/lib/utils'

export function Checkbox({ className, indeterminate, ...props }: CheckboxPrimitive.Root.Props) {
  return (
    <CheckboxPrimitive.Root
      data-slot="checkbox"
      indeterminate={indeterminate}
      className={cn(
        'peer grid size-4 shrink-0 cursor-pointer place-items-center rounded-sm border border-input bg-background text-primary-foreground shadow-xs outline-none transition-colors',
        'hover:border-primary/60 focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50',
        'data-checked:border-primary data-checked:bg-primary data-indeterminate:border-primary data-indeterminate:bg-primary disabled:cursor-not-allowed data-disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      {...props}
    >
      <CheckboxPrimitive.Indicator className="grid place-items-center">
        {indeterminate ? <PhMinus aria-hidden="true" className="size-3" /> : <PhCheck aria-hidden="true" className="size-3" />}
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  )
}
