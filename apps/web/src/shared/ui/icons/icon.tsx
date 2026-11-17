import type { ComponentType, ComponentProps, SVGProps } from 'react'
import { cn } from '@/shared/lib/utils'
import { ICON_DATA, type GeneratedIconName } from './icons.data'

export type { GeneratedIconName }

// Hugeicons Stroke Rounded, generated offline from the registry.
// Shared SVG rendering keeps sizing, currentColor and accessibility consistent.
export const ICON_CONTEXT_SIZES = {
  control: 16,
  nav: 18,
  display: 24,
  emptySmall: 32,
  emptyMedium: 40,
  emptyLarge: 48,
} as const

export type IconTone = 'inherit' | 'neutral' | 'success' | 'warning' | 'danger' | 'info'

const TONE_CLASSES: Record<Exclude<IconTone, 'inherit'>, string> = {
  neutral: 'text-muted-foreground',
  success: 'text-emerald-600 dark:text-emerald-400',
  warning: 'text-amber-600 dark:text-amber-400',
  danger: 'text-red-600 dark:text-red-400',
  info: 'text-blue-600 dark:text-blue-400',
}

export type IconProps = Omit<SVGProps<SVGSVGElement>, 'name' | 'color'> & {
  name: GeneratedIconName
  size?: number
  tone?: IconTone
}

export function Icon({ name, size = 24, tone = 'inherit', className, ...rest }: IconProps) {
  const glyph = ICON_DATA[name]
  const labelled = rest['aria-label'] !== undefined
  return (
    <svg
      viewBox={`0 0 ${glyph.width} ${glyph.height}`}
      width={size}
      height={size}
      fill="none"
      className={cn('shrink-0', tone !== 'inherit' ? TONE_CLASSES[tone] : undefined, className)}
      {...(labelled ? { role: 'img' } : { 'aria-hidden': true })}
      {...(rest as ComponentProps<'svg'>)}
      dangerouslySetInnerHTML={{ __html: glyph.body }}
    />
  )
}

export type IconComponentProps = Omit<IconProps, 'name'>
export type IconComponent = ComponentType<IconComponentProps>
