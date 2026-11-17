import { useRef, useState } from 'react'
import { Button, EmptyState, Label, Popover, PopoverContent, PopoverTrigger, Select, SelectContent, SelectItem, SelectTrigger, StatusPill } from '@/shared/ui'
import { BitableInput as Input } from '../controls'
import { PhCaretDown, PhCheck, PhDotsSixVertical, PhPlus, PhX } from '@/shared/ui/icons'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { cn } from '@/shared/lib/utils'
import { CHOICE_COLORS, type Choice, type ChoiceColor } from '../../model/types'
import { choiceTone } from '../../model/choice-tone'

type Props = {
  choices: readonly Choice[]; selected: readonly string[]; multiple: boolean; disabled: boolean
  onAdd: () => void; onUpdate: (id: string, patch: { name?: string; color?: ChoiceColor }) => void
  onRemove: (id: string) => void; onMove: (id: string, to: number) => void; onSelect: (id: string) => void; onClear: () => void
}
function Swatch({ choice, children }: { choice: Choice; children?: React.ReactNode }) {
  return <StatusPill tone={choiceTone(choice)} className={cn('min-w-0 rounded-full px-1.5', choice.color === 'pink' && 'border-pink-200 bg-pink-50 text-pink-700 dark:border-pink-400/35 dark:bg-pink-400/14 dark:text-pink-200')}>{children ?? <PhCaretDown className="size-3" />}</StatusPill>
}
export function FieldChoiceOptions(props: Props) {
  const dragging = useRef<string | null>(null)
  const [open, setOpen] = useState(false), [query, setQuery] = useState('')
  const available = props.choices.filter(choice => choice.name.trim())
  const matches = available.filter(choice => choice.name.toLocaleLowerCase().includes(query.toLocaleLowerCase()))
  return <div className="grid gap-4 border-t border-border pt-4">
    <div className="grid gap-2"><Label>{text.choiceContent}</Label>
      <div className="grid gap-2 rounded-md bg-muted/60 p-2">
        <Button variant="ghost" size="sm" className="h-7 justify-start px-1 text-primary" disabled={props.disabled} onClick={props.onAdd}><PhPlus />{text.addChoice}</Button>
        {props.choices.map((choice, index) => <div key={choice.id} className="flex min-w-0 items-center gap-1.5" onDragOver={event => { if (dragging.current && !props.disabled) event.preventDefault() }} onDrop={event => { event.preventDefault(); if (dragging.current && !props.disabled) props.onMove(dragging.current, index); dragging.current = null }}>
          <Button size="icon-xs" variant="ghost" className="shrink-0 cursor-grab text-muted-foreground" aria-label={text.moveChoice(index + 1)} title={text.moveChoice(index + 1)} disabled={props.disabled} draggable={!props.disabled} onDragStart={event => { dragging.current = choice.id; event.dataTransfer.setData('text/plain', choice.id); event.dataTransfer.effectAllowed = 'move' }} onDragEnd={() => { dragging.current = null }} onKeyDown={event => { if (event.altKey && (event.key === 'ArrowUp' || event.key === 'ArrowDown')) { event.preventDefault(); props.onMove(choice.id, index + (event.key === 'ArrowUp' ? -1 : 1)) } }}><PhDotsSixVertical /></Button>
          <Select value={choice.color ?? 'blue'} items={CHOICE_COLORS.map(value => ({ value, label: text.choiceColors[value] }))} disabled={props.disabled} onValueChange={color => { if (color) props.onUpdate(choice.id, { color }) }}>
            <SelectTrigger aria-label={text.choiceColor(index + 1)} className="h-7 w-7 justify-center border-0 bg-transparent p-0 shadow-none [&_[data-slot=select-icon]]:hidden"><Swatch choice={choice} /></SelectTrigger>
            <SelectContent>{CHOICE_COLORS.map(color => <SelectItem key={color} value={color}><Swatch choice={{ id: color, name: color, color }}>{text.choiceColors[color]}</Swatch></SelectItem>)}</SelectContent>
          </Select>
          <Input className="h-8 min-w-0 flex-1 bg-card text-sm" aria-label={text.choiceName(index + 1)} placeholder={text.choicePlaceholder} value={choice.name} maxLength={128} disabled={props.disabled} onChange={event => props.onUpdate(choice.id, { name: event.currentTarget.value })} />
          <Button size="icon-xs" variant="ghost" className="shrink-0 text-muted-foreground" aria-label={text.removeChoice(index + 1)} disabled={props.disabled} onClick={() => props.onRemove(choice.id)}><PhX /></Button>
        </div>)}
      </div>
    </div>
    <div className="grid gap-2"><Label>{text.defaultValue}</Label>
      <Popover open={open} onOpenChange={value => { setOpen(value); if (!value) setQuery('') }}>
        <PopoverTrigger render={<Button variant="outline" />} disabled={props.disabled || !available.length} aria-label={text.defaultValue} className="h-9 w-full justify-between gap-2 px-2 font-normal">
          <span className="flex min-w-0 items-center gap-1 overflow-hidden">{props.selected.length ? available.filter(choice => props.selected.includes(choice.id)).map(choice => <Swatch key={choice.id} choice={choice}>{choice.name}</Swatch>) : <span className="text-muted-foreground">{text.chooseDefault}</span>}</span><PhCaretDown className="size-4 shrink-0" />
        </PopoverTrigger>
        <PopoverContent className="w-[var(--anchor-width)] p-1">
          <Input autoFocus aria-label={text.choiceSearch} placeholder={text.choiceSearch} value={query} onChange={event => setQuery(event.currentTarget.value)} className="h-8 border-0 shadow-none" />
          <div role="listbox" aria-label={text.defaultValue} aria-multiselectable={props.multiple || undefined} className="max-h-48 overflow-y-auto">
            {matches.length ? matches.map(choice => <Button key={choice.id} role="option" aria-selected={props.selected.includes(choice.id)} variant="ghost" size="sm" className="w-full justify-between px-2 aria-selected:bg-muted" onClick={() => { props.onSelect(choice.id); if (!props.multiple) setOpen(false) }}><Swatch choice={choice}>{choice.name}</Swatch>{props.selected.includes(choice.id) ? <PhCheck className="size-4" /> : null}</Button>) : <EmptyState title={text.noMatchingChoices} />}
          </div>
          <Button size="sm" variant="ghost" className="w-full justify-start border-t border-border text-muted-foreground" onClick={() => { props.onClear(); setOpen(false) }}>{text.noDefault}</Button>
        </PopoverContent>
      </Popover>
    </div>
  </div>
}
