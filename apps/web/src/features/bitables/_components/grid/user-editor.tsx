import { useRef } from 'react'
import { copy } from '@/shared/copy'
import { MultiUserSelect, UserSelect } from '@/shared/ui'
import type { CellValue, Field, UserRef } from '../../model/types'

/** Shared PM controls own search, selection indications, avatar chips and list virtualization. */
export function UserEditor({ field, value, people, pending, onChange, onCommit, onCancel }: {
  field: Field; value: CellValue; people: readonly UserRef[]; pending: boolean
  onChange: (value: CellValue) => void
  onCommit: () => Promise<boolean>; onCancel: () => void
}) {
  const canceled = useRef(false)
  const selected = Array.isArray(value) ? value.filter((item): item is UserRef => typeof item === 'object' && typeof item.id === 'number') : []
  return <div className="h-full [&>div]:h-full" onKeyDownCapture={event => { if (event.key === 'Escape' && !event.nativeEvent.isComposing) { canceled.current = true; event.stopPropagation(); onCancel() } }}>
    {field.options?.multiple
      ? <MultiUserSelect defaultOpen label={field.name} value={selected} options={people} disabled={pending} clearable placeholder={copy.ui.userSelect.placeholder} selectionDisplay="inline" className="h-full min-h-0 rounded-none border-0 bg-transparent px-2 shadow-none outline-none focus-visible:border-transparent focus-visible:ring-0 focus-visible:ring-offset-0 aria-invalid:border-transparent aria-invalid:ring-0" contentClassName="max-h-72" onValueChange={onChange} onOpenChange={open => { if (!open && !pending && !canceled.current) void onCommit() }} />
      : <UserSelect defaultOpen label={field.name} value={selected[0]?.id} options={people} disabled={pending} clearable placeholder={copy.ui.userSelect.placeholder} className="h-full min-h-0 rounded-none border-0 bg-transparent px-2 shadow-none outline-none focus-visible:border-transparent focus-visible:ring-0 focus-visible:ring-offset-0 aria-invalid:border-transparent aria-invalid:ring-0" contentClassName="max-h-72" onValueChange={id => { onChange(people.filter(person => person.id === id)); void onCommit() }} onOpenChange={open => { if (!open && !pending && !canceled.current) void onCommit() }} />}
  </div>
}
