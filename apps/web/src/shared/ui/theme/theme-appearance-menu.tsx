import { PhDesktop, PhMoonStars, PhSun } from '@/shared/ui/icons'

import { cn } from '@/shared/lib/utils'
import {
  Popover,
  PopoverClose,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
} from '../primitives/popover'

export type AppearanceMode = 'light' | 'dark' | 'system'

export type ThemeAppearanceLabels = {
  menu: string
  light: string
  dark: string
  system: string
}

const optionIcons = {
  light: PhSun,
  dark: PhMoonStars,
  system: PhDesktop,
} as const

export function ThemeAppearanceMenu({
  appearance,
  resolved,
  onChange,
  labels,
  className,
  triggerClassName,
}: {
  appearance: AppearanceMode
  resolved: 'light' | 'dark'
  onChange: (next: AppearanceMode) => void
  labels: ThemeAppearanceLabels
  className?: string
  triggerClassName?: string
}) {
  const TriggerIcon = resolved === 'dark' ? PhMoonStars : PhSun
  const options: Array<{ value: AppearanceMode; label: string }> = [
    { value: 'light', label: labels.light },
    { value: 'dark', label: labels.dark },
    { value: 'system', label: labels.system },
  ]

  return (
    <div data-slot="theme-appearance-menu" className={className}>
      <Popover>
        <PopoverTrigger
          type="button"
          aria-label={labels.menu}
          className={cn(
            'inline-flex size-10 items-center justify-center rounded-full outline-none',
            // eslint-disable-next-line shadcn/require-static-classes -- pass-through of the ThemeAppearanceMenu triggerClassName prop; the rule cannot trace non-className parameters across components
            triggerClassName,
          )}
        >
          <TriggerIcon size={19} aria-hidden="true" />
        </PopoverTrigger>
        <PopoverContent
          align="end"
          side="bottom"
          sideOffset={8}
          className="w-44 rounded-lg p-1.5"
        >
          <PopoverTitle className="sr-only">{labels.menu}</PopoverTitle>
          <PopoverDescription className="sr-only">{labels.menu}</PopoverDescription>
          {options.map((option) => {
            const Icon = optionIcons[option.value]
            const selected = appearance === option.value
            return (
              <PopoverClose
                key={option.value}
                type="button"
                aria-checked={selected}
                role="menuitemradio"
                className="flex w-full cursor-pointer items-center gap-2.5 rounded-lg px-2 py-2 text-left text-sm text-foreground hover:bg-muted"
                onClick={() => onChange(option.value)}
              >
                <span className="grid size-3 place-items-center">
                  {selected ? <span className="size-1.5 rounded-full bg-foreground/55" /> : null}
                </span>
                <Icon size={17} aria-hidden="true" />
                {option.label}
              </PopoverClose>
            )
          })}
        </PopoverContent>
      </Popover>
    </div>
  )
}
