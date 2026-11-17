import { useRef, useState } from 'react'
import { AlertBanner, Button, Checkbox, DatePicker } from '@/shared/ui'
import { BitableInput as Input, BitableTextarea as Textarea } from '../controls'
import { BITABLE_COLLABORATION_COPY as collaborationCopy } from '@/shared/copy/bitables-collaboration'
import { ChoiceEditor } from './choice-editor'
import { UserEditor } from './user-editor'
import { cn } from '@/shared/lib/utils'
import type { CellValue, Field, UserRef } from '../../model/types'
import type { CellEditorState } from '../../hooks/grid/use-grid-interaction'

export function CellEditor({ field, state, people, height = 36, onChange, onCommit, onCancel, onComposition, onTab }: {
  field: Field; state: Exclude<CellEditorState, { phase: 'view' }>; people: readonly UserRef[]
  onChange: (value: CellValue) => void; onCommit: () => Promise<boolean>; onCancel: () => void
  onComposition: (composing: boolean) => void
  height?: number
  onTab?: (direction: -1 | 1) => void
}) {
  const composing = useRef(false)
  const dateHandled = useRef(false)
  const pending = state.phase === 'saving'
  const [copyFailed, setCopyFailed] = useState(false)
  const stringValue = typeof state.value === 'string' || typeof state.value === 'number' ? String(state.value) : ''
  const inputProps = {
    autoFocus: true, 'aria-label': field.name, 'aria-invalid': state.phase === 'save_error',
    disabled: pending, value: stringValue,
    onChange: (event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => onChange(event.currentTarget.value),
    onBlur: () => { if (!composing.current) void onCommit() },
    onCompositionStart: () => { composing.current = true; onComposition(true) },
    onCompositionEnd: () => { composing.current = false; onComposition(false) },
    onKeyDown: (event: React.KeyboardEvent) => {
      event.stopPropagation()
      if (event.nativeEvent.isComposing || composing.current || event.keyCode === 229) return
      if (event.key === 'Escape') { event.preventDefault(); onCancel() }
      if (event.key === 'Tab' && onTab) { event.preventDefault(); void onCommit().then((ok) => { if (ok) onTab(event.shiftKey ? -1 : 1) }) }
      if (event.key === 'Enter' && (field.type !== 'long_text' || event.ctrlKey || event.metaKey)) { event.preventDefault(); void onCommit() }
    },
  }
  return (
    <div className="relative h-(--editor-h) min-w-0 bg-transparent [&>div]:h-full" style={{ '--editor-h': `${height}px` }} data-bitable-editor="true" onPointerDown={(event) => event.stopPropagation()} onKeyDownCapture={event => {
      if (field.type === 'date' && event.key === 'Escape' && !event.nativeEvent.isComposing) { dateHandled.current = true; event.stopPropagation(); onCancel() }
    }} onKeyDown={(event) => { event.stopPropagation(); if (event.key === 'Escape' && !event.nativeEvent.isComposing) onCancel() }}>
      {field.type === 'long_text' ? <Textarea {...inputProps} className={cn('h-full min-h-0 rounded-none border-0 bg-transparent px-2 shadow-none outline-none focus-visible:border-transparent focus-visible:ring-0 focus-visible:ring-offset-0 aria-invalid:border-transparent aria-invalid:ring-0', 'resize-none py-1 text-sm')} />
        : field.type === 'checkbox' ? <span className="flex h-full items-center px-2"><Checkbox ref={node => { node?.focus({ preventScroll: true }) }} aria-label={field.name} className="focus-visible:ring-0 disabled:opacity-100" checked={state.value === true} disabled={pending} onCheckedChange={(checked) => { onChange(checked === true); void onCommit() }} /></span>
          : field.type === 'single_select' || field.type === 'multiple_select' ? <ChoiceEditor field={field} value={state.value} pending={pending} onChange={onChange} onCommit={onCommit} onCancel={onCancel} />
          : field.type === 'user' ? <UserEditor field={field} value={state.value} people={people} pending={pending} onChange={onChange} onCommit={onCommit} onCancel={onCancel} />
          : field.type === 'date' ? <DatePicker defaultOpen aria-label={field.name} value={stringValue.slice(0, 10)} disabled={pending} className="h-full min-h-0 rounded-none border-0 bg-transparent px-2 shadow-none outline-none focus-visible:border-transparent focus-visible:ring-0 focus-visible:ring-offset-0 aria-invalid:border-transparent aria-invalid:ring-0" onChange={value => { dateHandled.current = true; onChange(value); void onCommit() }} onOpenChange={open => {
            if (open) dateHandled.current = false
            else if (!pending && !dateHandled.current) { dateHandled.current = true; onCancel() }
          }} />
            : <Input {...inputProps} type="text" inputMode={field.type === 'number' ? 'decimal' : undefined} className={cn('h-full min-h-0 rounded-none border-0 bg-transparent px-2 shadow-none outline-none focus-visible:border-transparent focus-visible:ring-0 focus-visible:ring-offset-0 aria-invalid:border-transparent aria-invalid:ring-0', field.type === 'number' && 'text-right tabular-nums')} />}
      {state.phase === 'save_error' ? <aside className="absolute top-full left-0 z-50 min-w-52 max-w-80 pt-1"><AlertBanner tone="error">
        {copyFailed ? collaborationCopy.draftCopyFailed : state.message}
        <Button size="sm" variant="outline" className="mt-2" onPointerDown={event => event.preventDefault()} onClick={async () => {
          try {
            await navigator.clipboard.writeText(typeof state.value === 'string' ? state.value : JSON.stringify(state.value))
            onCancel()
          } catch { setCopyFailed(true) }
        }}>{collaborationCopy.copyDraftAndExit}</Button>
      </AlertBanner></aside> : null}
    </div>
  )
}
