import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
} from 'react'

import { cn } from '@/shared/lib/utils'
import { Button } from '../primitives/button'
import { Input } from '../primitives/input'

type InlineEditPhase = 'view' | 'editing' | 'saving'

type CancelOptions = {
  restoreFocus?: boolean
}

export const INLINE_EDIT_CONTROL_CLASS = 'h-8 w-full min-w-0 max-w-full'

const INLINE_EDIT_READ_VALUE_CLASS =
  'inline-flex items-center justify-start overflow-hidden border border-transparent px-2 text-left font-normal'

export function useInlineEditField(
  value: string,
  { closeOnOutsidePointer = true }: { closeOnOutsidePointer?: boolean } = {},
) {
  const [phase, setPhase] = useState<InlineEditPhase>('view')
  const [draft, setDraft] = useState(value)
  const savingRef = useRef(false)
  const restoreFocusRef = useRef(false)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const editorRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (phase === 'view') setDraft(value)
  }, [phase, value])
  useEffect(() => {
    if (phase !== 'view' || !restoreFocusRef.current) return
    restoreFocusRef.current = false
    triggerRef.current?.focus()
  }, [phase])

  const cancel = useCallback(({ restoreFocus = true }: CancelOptions = {}) => {
    if (savingRef.current) return false
    restoreFocusRef.current = restoreFocus
    setDraft(value)
    setPhase('view')
    return true
  }, [value])

  useEffect(() => {
    if (phase !== 'editing' || !closeOnOutsidePointer) return undefined
    const handleOutsidePointer = (event: PointerEvent) => {
      const target = event.target
      if (!(target instanceof Element)) return
      if (editorRef.current?.contains(target)) return
      if (target.closest('[data-slot="select-content"],[data-slot="combobox-content"]')) return
      cancel({ restoreFocus: false })
    }
    document.addEventListener('pointerdown', handleOutsidePointer, true)
    return () => document.removeEventListener('pointerdown', handleOutsidePointer, true)
  }, [cancel, closeOnOutsidePointer, phase])

  const open = useCallback(() => {
    setDraft(value)
    setPhase('editing')
  }, [value])

  return {
    editing: phase !== 'view',
    saving: phase === 'saving',
    draft,
    editorRef,
    triggerRef,
    setDraft,
    open,
    cancel,
    save: async <T,>(next: T, run: (value: T) => Promise<boolean | void>) => {
      if (savingRef.current) return
      savingRef.current = true
      setPhase('saving')
      try {
        if (await run(next) !== false) {
          savingRef.current = false
          restoreFocusRef.current = true
          setPhase('view')
          return
        }
      } catch {
        // The domain mutation boundary owns user-facing retry feedback.
      }
      savingRef.current = false
      setPhase('editing')
    },
  }
}

export function InlineEditTrigger({
  label,
  display,
  editable,
  triggerRef,
  className,
  onOpen,
}: {
  label: string
  display: ReactNode
  editable: boolean
  triggerRef: RefObject<HTMLButtonElement | null>
  className?: string
  onOpen: () => void
}) {
  if (!editable) {
    return (
      <span
        data-inline-field-control="true"
        className={cn(INLINE_EDIT_CONTROL_CLASS, INLINE_EDIT_READ_VALUE_CLASS)}
      >
        <span className="min-w-0 truncate">{display}</span>
      </span>
    )
  }
  return (
    <Button
      ref={triggerRef}
      data-inline-field-control="true"
      type="button"
      variant="ghost"
      size="xs"
      className={cn(
        INLINE_EDIT_CONTROL_CLASS,
        INLINE_EDIT_READ_VALUE_CLASS,
        'hover:bg-muted',
        className,
      )}
      aria-label={label}
      onClick={onOpen}
    >
      <span className="min-w-0 truncate">{display}</span>
    </Button>
  )
}

export function InlineEditFrame({
  editorRef,
  children,
  className,
  onCancel,
}: {
  editorRef: RefObject<HTMLDivElement | null>
  children: ReactNode
  className?: string
  onCancel: () => void
}) {
  return (
    <div
      ref={editorRef}
      data-inline-field-control="true"
      className={cn(INLINE_EDIT_CONTROL_CLASS, className)}
      onKeyDownCapture={(event) => {
        if (event.key === 'Escape') onCancel()
      }}
    >
      {children}
    </div>
  )
}

export function InlineEditText({
  id,
  label,
  value,
  display = value,
  editable,
  pending = false,
  maxLength,
  onSave,
}: {
  id: string
  label: string
  value: string
  display?: ReactNode
  editable: boolean
  pending?: boolean
  maxLength?: number
  onSave: (value: string) => Promise<boolean | void>
}) {
  const editor = useInlineEditField(value, { closeOnOutsidePointer: false })
  const commit = () => {
    if (editor.draft === value) return editor.cancel()
    void editor.save(editor.draft, onSave)
  }

  if (!editor.editing) {
    return (
      <InlineEditTrigger
        label={label}
        display={display}
        editable={editable}
        triggerRef={editor.triggerRef}
        onOpen={editor.open}
      />
    )
  }

  return (
    <InlineEditFrame
      editorRef={editor.editorRef}
      className="h-auto"
      onCancel={editor.cancel}
    >
      <Input
        id={id}
        data-inline-field-control="true"
        type="text"
        className={INLINE_EDIT_CONTROL_CLASS}
        value={editor.draft}
        maxLength={maxLength}
        disabled={pending || editor.saving}
        aria-label={label}
        autoFocus
        onKeyDown={(event) => {
          if (event.key === 'Escape') editor.cancel()
          if (event.key === 'Enter') commit()
        }}
        onChange={(event) => editor.setDraft(event.currentTarget.value)}
        onBlur={commit}
      />
    </InlineEditFrame>
  )
}
