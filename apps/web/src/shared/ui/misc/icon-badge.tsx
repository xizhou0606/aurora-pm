import type { ComponentType, ReactNode, SVGProps } from 'react'

import { cn } from '@/shared/lib/utils'

export type IconBadgeTone =
  | 'brand'
  | 'blue'
  | 'indigo'
  | 'violet'
  | 'cyan'
  | 'emerald'
  | 'amber'
  | 'red'
  | 'neutral'

export type IconBadgeVariant = 'solid' | 'soft'

export type IconBadgeProps = {
  icon?: ComponentType<SVGProps<SVGSVGElement>>
  children?: ReactNode
  tone?: IconBadgeTone
  variant?: IconBadgeVariant
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

// The one sanctioned icon container (docs/ICON_SYSTEM.md): solid for section
// markers, soft for status chips. Hand-rolled rounded wrappers around icons
// must be replaced by this component.
const SOLID_TONE_CLASSES: Record<IconBadgeTone, string> = {
  brand: 'bg-primary text-primary-foreground',
  blue: 'bg-blue-600 text-white',
  indigo: 'bg-indigo-600 text-white',
  violet: 'bg-violet-600 text-white',
  cyan: 'bg-cyan-600 text-white',
  emerald: 'bg-emerald-600 text-white',
  amber: 'bg-amber-600 text-white',
  red: 'bg-red-600 text-white',
  neutral: 'bg-muted text-muted-foreground',
}

const SOFT_TONE_CLASSES: Record<IconBadgeTone, string> = {
  brand: 'bg-primary/10 text-primary',
  blue: 'bg-blue-100 text-blue-700 dark:bg-blue-950/40 dark:text-blue-300',
  indigo: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-950/40 dark:text-indigo-300',
  violet: 'bg-violet-100 text-violet-700 dark:bg-violet-950/40 dark:text-violet-300',
  cyan: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-950/40 dark:text-cyan-300',
  emerald: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300',
  amber: 'bg-amber-100 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300',
  red: 'bg-red-100 text-red-700 dark:bg-red-950/40 dark:text-red-300',
  neutral: 'bg-muted text-muted-foreground',
}

const SIZE_CLASSES: Record<'sm' | 'md' | 'lg', string> = {
  sm: 'size-7 [&_svg]:size-3.5',
  md: 'size-8 [&_svg]:size-4',
  lg: 'size-9 [&_svg]:size-4.5',
}

export function IconBadge({
  icon: Icon,
  children,
  tone = 'neutral',
  variant = 'solid',
  size = 'md',
  className,
}: IconBadgeProps) {
  return (
    <span
      className={cn(
        'grid shrink-0 place-items-center rounded-lg',
        variant === 'soft' ? SOFT_TONE_CLASSES[tone] : SOLID_TONE_CLASSES[tone],
        SIZE_CLASSES[size],
        className,
      )}
    >
      {Icon ? <Icon aria-hidden="true" /> : null}
      {children}
    </span>
  )
}
