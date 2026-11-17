import {
  InlineEditFrame,
  InlineEditText,
  Input,
  DatePicker,
  UserPreview,
  UserSelect,
  useInlineEditField,
} from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { WorkItemUserOption } from '../../model/types'
import { workItemUserSelectOption } from '../../model/utils'
import type { SelectOption } from './SelectField'
import { WorkItemsSelect } from './SelectField'
import type { CommonProps } from './InlineFieldCommon'
import { ReadValue } from './InlineFieldCommon'

export type { CommonProps }

export function InlineTextField({
  id,
  label,
  rowTitle,
  value,
  display,
  editable,
  mutation,
  maxLength,
  onSave,
}: CommonProps & {
  value: string
  maxLength?: number
  onSave: (value: string) => Promise<boolean>
}) {
  return (
    <InlineEditText
      id={id}
      label={WORK_ITEMS_COPY.format.fieldLabel(label, rowTitle)}
      value={value}
      display={display}
      editable={editable}
      pending={mutation.status === 'pending'}
      maxLength={maxLength}
      onSave={onSave}
    />
  )
}

export function InlineSelectField({
  id,
  label,
  rowTitle,
  value,
  display,
  options,
  editable,
  mutation,
  placeholder,
  contentClassName,
  onSave,
}: CommonProps & {
  value: string
  options: readonly SelectOption[]
  placeholder?: string
  contentClassName?: string
  onSave: (value: string) => Promise<boolean>
}) {
  const editor = useInlineEditField(value)
  const pending = editor.saving || mutation.status === 'pending'
  const save = (next: string) => {
    editor.setDraft(next)
    void editor.save(next, onSave)
  }

  if (!editor.editing) {
    return (
      <ReadValue
        label={label}
        rowTitle={rowTitle}
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
      <WorkItemsSelect
        id={id}
        label={WORK_ITEMS_COPY.format.fieldLabel(label, rowTitle)}
        value={editor.draft}
        options={options}
        placeholder={placeholder}
        // Static popup classes are forwarded from the inline-field consumer.
        // eslint-disable-next-line shadcn/require-static-classes
        contentClassName={contentClassName}
        disabled={pending}
        onOpenChange={(open) => {
          if (!open) editor.cancel({ restoreFocus: false })
        }}
        onValueChange={save}
      />
    </InlineEditFrame>
  )
}

export function InlineUserField({
  id,
  label,
  rowTitle,
  value,
  displayName,
  avatarUrl,
  options,
  editable,
  mutation,
  onSave,
}: Omit<CommonProps, 'display'> & {
  value?: number
  displayName: string
  avatarUrl?: string
  options: readonly WorkItemUserOption[]
  onSave: (value: number) => Promise<boolean>
}) {
  const editor = useInlineEditField(value ? String(value) : '')
  const pending = editor.saving || mutation.status === 'pending'
  const display = value ? (
    <UserPreview name={displayName} avatarUrl={avatarUrl} size="sm" />
  ) : WORK_ITEMS_COPY.list.unassigned

  if (!editor.editing) {
    return (
      <ReadValue
        label={label}
        rowTitle={rowTitle}
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
      <UserSelect
        id={id}
        label={WORK_ITEMS_COPY.format.fieldLabel(label, rowTitle)}
        value={value}
        options={options.map(workItemUserSelectOption)}
        placeholder={WORK_ITEMS_COPY.list.unassigned}
        disabled={pending}
        onOpenChange={(open) => {
          if (!open) editor.cancel({ restoreFocus: false })
        }}
        onValueChange={(next) => {
          if (next === undefined) return
          editor.setDraft(String(next))
          void editor.save(next, onSave)
        }}
      />
    </InlineEditFrame>
  )
}

export function InlineDateField({
  id,
  label,
  rowTitle,
  value,
  display,
  editable,
  mutation,
  onSave,
}: CommonProps & {
  value: string
  onSave: (value: string | null) => Promise<boolean>
}) {
  const editor = useInlineEditField(value, { closeOnOutsidePointer: false })
  const pending = editor.saving || mutation.status === 'pending'
  if (!editor.editing) {
    return (
      <ReadValue
        label={label}
        rowTitle={rowTitle}
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
      <DatePicker
        id={id} defaultOpen
        className="h-8 w-full min-w-0 max-w-full"
        value={editor.draft} disabled={pending}
        aria-label={WORK_ITEMS_COPY.format.fieldLabel(label, rowTitle)}
        onChange={(next) => {
          editor.setDraft(next)
          if (next === value) editor.cancel()
          else void editor.save(next, async (date) => onSave(date || null))
        }}
        onOpenChange={(open) => { if (!open) editor.cancel() }}
      />
    </InlineEditFrame>
  )
}

export function InlineEstimatedField({
  id,
  label,
  rowTitle,
  value,
  display,
  editable,
  mutation,
  onSave,
}: CommonProps & {
  value?: number
  onSave: (value: number) => Promise<boolean>
}) {
  const hours = value === undefined ? '' : String(Number((value / 60).toFixed(1)))
  const editor = useInlineEditField(hours, { closeOnOutsidePointer: false })
  const pending = editor.saving || mutation.status === 'pending'
  const commit = (intent: 'blur' | 'submit') => {
    if (editor.draft === hours) {
      editor.cancel()
      return
    }
    const parsed = Number(editor.draft)
    if (!Number.isFinite(parsed) || parsed < 0 || parsed > 16_666) {
      if (intent === 'blur') editor.cancel({ restoreFocus: false })
      return
    }
    void editor.save(Math.round(parsed * 60), onSave)
  }

  if (!editor.editing) {
    return (
      <ReadValue
        label={label}
        rowTitle={rowTitle}
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
      <Input
        id={id}
        type="number"
        min={0}
        max={16666}
        step={0.5}
        className="h-8 w-full min-w-0 max-w-full"
        value={editor.draft}
        disabled={pending}
        aria-label={WORK_ITEMS_COPY.format.fieldLabel(label, rowTitle)}
        autoFocus
        onKeyDown={(event) => {
          if (event.key === 'Escape') editor.cancel()
          if (event.key === 'Enter') commit('submit')
        }}
        onChange={(event) => editor.setDraft(event.currentTarget.value)}
        onBlur={() => commit('blur')}
      />
    </InlineEditFrame>
  )
}
