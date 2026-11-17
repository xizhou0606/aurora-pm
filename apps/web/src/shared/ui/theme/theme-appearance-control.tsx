import { copy } from '@/shared/copy'

import { useAppearance } from './appearance'
import { ThemeAppearanceMenu, type ThemeAppearanceLabels } from './theme-appearance-menu'

export const appearanceMenuLabels: ThemeAppearanceLabels = {
  menu: copy.ui.appearance.menu,
  light: copy.ui.appearance.light,
  dark: copy.ui.appearance.dark,
  system: copy.ui.appearance.system,
}

export function ThemeAppearanceControl({
  className,
  triggerClassName,
  labels = appearanceMenuLabels,
}: {
  className?: string
  triggerClassName?: string
  labels?: ThemeAppearanceLabels
}) {
  const { appearance, resolved, setAppearance } = useAppearance()
  return (
    <ThemeAppearanceMenu
      appearance={appearance}
      resolved={resolved}
      onChange={setAppearance}
      labels={labels}
      className={className}
      // eslint-disable-next-line shadcn/require-static-classes -- pass-through of the ThemeAppearanceControl triggerClassName prop; the rule cannot trace non-className parameters across components
      triggerClassName={triggerClassName}
    />
  )
}
