import type { ComponentProps, ReactNode } from 'react'

import { cn } from '@/shared/lib/utils'
import { Button } from '../primitives/button'

type TableActionButtonBaseProps = Omit<
  ComponentProps<typeof Button>,
  'aria-label' | 'children' | 'size' | 'title' | 'variant'
> & {
  label: string
}

type TableActionButtonProps = TableActionButtonBaseProps & (
  | { text: string; icon?: never }
  | { icon: ReactNode; text?: never }
)

export function TableActionButton({
  className,
  icon,
  label,
  text,
  ...props
}: TableActionButtonProps) {
  return (
    <Button
      {...props}
      type={props.type ?? 'button'}
      size={text ? 'xs' : 'icon-sm'}
      variant="ghost"
      aria-label={label}
      title={label}
      data-slot="table-action-button"
      className={cn('text-muted-foreground hover:text-foreground', className)}
    >
      {text ?? icon}
    </Button>
  )
}
