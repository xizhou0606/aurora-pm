import { PhX } from '@/shared/ui/icons'

import { cn } from '@/shared/lib/utils'

import { Button } from '../primitives/button'

export function ClearControlButton({
  label,
  className,
  disabled = false,
  onClick,
}: {
  label: string
  className?: string
  disabled?: boolean
  onClick: () => void
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon-xs"
      aria-label={label}
      disabled={disabled}
      className={cn(
        'absolute right-7 top-1/2 z-10 -translate-y-1/2 rounded-sm text-muted-foreground hover:bg-muted hover:text-foreground',
        className,
      )}
      onClick={(event) => {
        event.preventDefault()
        event.stopPropagation()
        onClick()
      }}
    >
      <PhX aria-hidden="true" className="size-3.5" />
    </Button>
  )
}
