import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { Button } from '@/shared/ui'
import { BitableInput as Input } from '../controls'
import { PhCaretLeft, PhCaretRight, PhMagnifyingGlass, PhX } from '@/shared/ui/icons'
import type { GridSearchControl } from '../../model/grid-port'

export function SearchControl({ control }: { control: GridSearchControl }) {
  return <div role="search" className="flex min-w-0 max-w-full items-center gap-1 rounded-md border border-border/70 bg-background px-2 focus-within:border-ring">
    <PhMagnifyingGlass aria-hidden="true" className="size-3.5 shrink-0 text-muted-foreground" />
    <Input aria-label={text.search} placeholder={text.search} value={control.query} onChange={(event) => control.onQueryChange(event.currentTarget.value)} className="h-7 min-w-0 w-28 border-0 px-1 text-xs shadow-none sm:w-36" />
    {control.query ? <>
      <span role="status" className="whitespace-nowrap text-xs text-muted-foreground">{control.pending ? text.searching : text.searchPosition(control.current, control.total)}</span>
      <Button size="icon-xs" variant="ghost" aria-label={text.previousMatch} disabled={!control.total || control.pending} onClick={() => control.onNavigate(-1)}><PhCaretLeft /></Button>
      <Button size="icon-xs" variant="ghost" aria-label={text.nextMatch} disabled={!control.total || control.pending} onClick={() => control.onNavigate(1)}><PhCaretRight /></Button>
      <Button size="icon-xs" variant="ghost" aria-label={text.clearSearch} onClick={() => control.onQueryChange('')}><PhX /></Button>
    </> : null}
  </div>
}
