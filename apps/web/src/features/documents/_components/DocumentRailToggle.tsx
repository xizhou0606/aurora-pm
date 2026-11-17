import { Button } from '@/shared/ui'
import { PhCaretLeft, PhCaretRight, PhList } from '@/shared/ui/icons'

export const documentRailMotion = 'transition-[width] duration-150 motion-reduce:transition-none'

export function DocumentRailToggle({ collapsed, side = 'left', directory = false, label, onClick }: {
  collapsed: boolean
  side?: 'left' | 'right'
  directory?: boolean
  label: string
  onClick: () => void
}) {
  const Glyph = collapsed && directory ? PhList : (side === 'left') === collapsed ? PhCaretRight : PhCaretLeft
  return <Button type="button" variant="ghost" size="icon-sm" className="shrink-0"
    aria-expanded={!collapsed} aria-label={label} title={label} onClick={onClick}>
    <Glyph aria-hidden="true" />
  </Button>
}
