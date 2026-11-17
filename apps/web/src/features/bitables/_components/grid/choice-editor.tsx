import { useRef, useState } from 'react'
import { BITABLE_GRID_COPY as copy } from '@/shared/copy/bitables-grid'
import { Button, EmptyState, Popover, PopoverContent, PopoverTrigger } from '@/shared/ui'
import { BitableInput as Input } from '../controls'
import type { CellValue, Field } from '../../model/types'
import { ChoiceTag } from './choice-tag'
import { cn } from '@/shared/lib/utils'

export function ChoiceEditor({ field, value, pending, onChange, onCommit, onCancel }: {
  field: Field; value: CellValue; pending: boolean; onChange: (value: CellValue) => void
  onCommit: () => Promise<boolean>; onCancel: () => void
}) {
  const [query, setQuery] = useState('')
  const canceled = useRef(false)
  const cancel = () => { canceled.current = true; onCancel() }
  const multiple = field.type === 'multiple_select'
  const selected = Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : typeof value === 'string' ? [value] : []
  const choices = (field.options?.choices ?? []).filter(choice => choice.name.toLocaleLowerCase().includes(query.toLocaleLowerCase()))
  const change = (next: CellValue) => { onChange(next); if (!multiple) void onCommit() }
  const choose = (id: string) => change(multiple ? selected.includes(id) ? selected.filter(item => item !== id) : [...selected, id] : selected.includes(id) ? null : id)
  return <Popover open onOpenChange={(open) => { if (!open && !pending && !canceled.current) void onCommit() }}>
    <PopoverTrigger render={<Button variant="ghost" size="sm" />} aria-label={field.name} className={cn('h-full min-h-0 rounded-none border-0 bg-transparent px-2 shadow-none outline-none focus-visible:border-transparent focus-visible:ring-0 focus-visible:ring-offset-0 aria-invalid:border-transparent aria-invalid:ring-0', 'w-full justify-start overflow-hidden hover:bg-transparent')}>
      {selected.length ? selected.map(id => <ChoiceTag key={id} field={field} id={id} />) : copy.emptyValue}
    </PopoverTrigger>
    <PopoverContent className="w-60 p-1" onPointerDown={event => event.stopPropagation()} onKeyDownCapture={event => { if (event.key === 'Escape' && !event.nativeEvent.isComposing) { event.stopPropagation(); cancel() } }} onKeyDown={event => event.stopPropagation()}>
      <Input autoFocus aria-label={copy.choiceSearch} placeholder={copy.choiceSearch} value={query} onChange={event => setQuery(event.currentTarget.value)} className="h-8 border-0 shadow-none" onKeyDown={event => {
        if (event.nativeEvent.isComposing) return
        if (event.key === 'ArrowDown') { event.preventDefault(); event.currentTarget.parentElement?.querySelector<HTMLButtonElement>('[role="option"]')?.focus() }
      }} />
      <div role="listbox" aria-label={field.name} aria-multiselectable={multiple || undefined} className="max-h-52 overflow-y-auto" onKeyDown={event => {
        if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
        const options = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('[role="option"]'))
        const current = options.indexOf(document.activeElement as HTMLButtonElement)
        const index = event.key === 'Home' ? 0 : event.key === 'End' ? options.length - 1 : (current + (event.key === 'ArrowDown' ? 1 : -1) + options.length) % options.length
        event.preventDefault(); options[index]?.focus()
      }}>
        {choices.length ? choices.map(choice => <Button key={choice.id} role="option" aria-selected={selected.includes(choice.id)} variant="ghost" size="sm" disabled={pending} onClick={() => choose(choice.id)} className="h-8 w-full justify-start rounded-md px-2 aria-selected:bg-muted hover:bg-muted focus:bg-muted"><ChoiceTag field={field} id={choice.id} /></Button>) : <EmptyState title={copy.noMatchingChoices} />}
      </div>
      <div className="mt-1 flex items-center justify-end gap-1 border-t border-border pt-1">
        <Button size="xs" variant="ghost" disabled={pending || !selected.length} onClick={() => change(null)}>{copy.clearSelection}</Button>
      </div>
    </PopoverContent>
  </Popover>
}
