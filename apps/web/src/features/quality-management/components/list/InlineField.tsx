import { type ComponentProps, type ReactNode } from 'react'

import { QUALITY_COPY } from '@/shared/copy'
import {
  INLINE_EDIT_CONTROL_CLASS,
  InlineEditFrame,
  InlineEditText,
  InlineEditTrigger,
  UserPreview,
  UserSelect,
  useInlineEditField,
} from '@/shared/ui'
import type { QualitySelectOption } from '../controls'
import { QualitySelectField } from '../controls'

function ReadValue({
  label,
  display,
  editable,
  triggerRef,
  onOpen,
}: {
  label: string
  display: ReactNode
  editable: boolean
  triggerRef: ComponentProps<typeof InlineEditTrigger>['triggerRef']
  onOpen: () => void
}) {
  return (
    <InlineEditTrigger
      label={label}
      display={display}
      editable={editable}
      triggerRef={triggerRef}
      onOpen={onOpen}
    />
  )
}

export function InlineText({
  id,
  label,
  editLabel = label,
  value,
  editable,
  onSave,
}: {
  id: string
  label: string
  editLabel?: string
  value: string
  editable: boolean
  onSave: (value: string) => Promise<void>
}) {
  return (
    <InlineEditText
      id={id}
      value={value}
      label={editLabel}
      editable={editable}
      onSave={async (draft) => {
        const next = draft.trim()
        if (!next) return false
        if (next === value) return true
        await onSave(next)
      }}
    />
  )
}

export function InlineSelect({
  id,
  label,
  value,
  display,
  options,
  editable,
  emptyLabel,
  contentClassName,
  onSave,
}: {
  id: string
  label: string
  value: string
  display: ReactNode
  options: readonly QualitySelectOption[]
  editable: boolean
  emptyLabel: string
  contentClassName?: string
  onSave: (value: string) => Promise<void>
}) {
  const editor = useInlineEditField(value)
  if (!editor.editing) {
    return (
      <ReadValue
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
      onCancel={editor.cancel}
    >
      <QualitySelectField
        id={id}
        label={label}
        value={editor.draft}
        options={options}
        emptyLabel={emptyLabel}
        showLabel={false}
        disabled={editor.saving}
        // Shared inline-edit token is the single source of control geometry.
        // eslint-disable-next-line shadcn/require-static-classes
        className={INLINE_EDIT_CONTROL_CLASS}
        // Static popup classes are forwarded from the inline-field consumer.
        // eslint-disable-next-line shadcn/require-static-classes
        contentClassName={contentClassName}
        onOpenChange={(open) => {
          if (!open) editor.cancel({ restoreFocus: false })
        }}
        onValueChange={(next) => {
          if (!next || next === value) return editor.cancel()
          editor.setDraft(next)
          void editor.save(next, onSave)
        }}
      />
    </InlineEditFrame>
  )
}

type OwnerIdentity = {
  value?: number
  displayName: string
  avatarUrl?: string
}

export function InlineOwner({
  id,
  label,
  assignee,
  options,
  editable,
  emptyLabel,
  onSave,
}: {
  id: string
  label: string
  assignee: OwnerIdentity
  options: readonly {
    id: number
    name: string
    secondary?: string
    avatarUrl?: string
  }[]
  editable: boolean
  emptyLabel: string
  onSave: (value?: number) => Promise<void>
}) {
  const editor = useInlineEditField(assignee.value ? String(assignee.value) : '')
  const display = assignee.value ? (
    <UserPreview
      name={assignee.displayName}
      avatarUrl={assignee.avatarUrl}
      size="sm"
      className="min-w-0"
    />
  ) : emptyLabel
  if (!editor.editing) {
    return (
      <ReadValue
        label={label}
        display={display}
        editable={editable}
        triggerRef={editor.triggerRef}
        onOpen={editor.open}
      />
    )
  }
  return (
    <InlineEditFrame editorRef={editor.editorRef} onCancel={editor.cancel}>
      <UserSelect
        id={id}
        label={label}
        value={assignee.value}
        options={options}
        emptyLabel={emptyLabel}
        clearable
        disabled={editor.saving}
        className="h-8 w-full min-w-0 max-w-full"
        onOpenChange={(open) => {
          if (!open) editor.cancel({ restoreFocus: false })
        }}
        onValueChange={(next) => {
          if (next === assignee.value) return editor.cancel()
          editor.setDraft(next ? String(next) : '')
          void editor.save(next, onSave)
        }}
      />
    </InlineEditFrame>
  )
}
