import { DatePicker, Input, Label } from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { useEffect, useRef, useState } from 'react'

import { WorkItemsSelect } from '../collection/SelectField'
import type { MutationState } from '../../model/types'
import { isValidEstimatedMinutes } from '../../model/utils'

type SelectOption = { value: string; label: string }

function useBlurSaveDraft(source: string, resetKey: string | number) {
  const [draft, setDraft] = useState(source)
  const [dirty, setDirty] = useState(false)
  const [saving, setSaving] = useState(false)
  const savingRef = useRef(false)
  const skipBlurRef = useRef(false)

  useEffect(() => {
    setDraft(source)
    setDirty(false)
    savingRef.current = false
    setSaving(false)
  }, [resetKey])
  useEffect(() => {
    if (!dirty) setDraft(source)
  }, [dirty, source])

  const cancel = () => {
    skipBlurRef.current = true
    setDraft(source)
    setDirty(false)
  }
  const save = async <T,>(next: T, run: (value: T) => Promise<boolean>) => {
    if (savingRef.current) return false
    savingRef.current = true
    setSaving(true)
    try {
      const saved = await run(next)
      if (saved) setDirty(false)
      return saved
    } finally {
      savingRef.current = false
      setSaving(false)
    }
  }

  return {
    draft,
    dirty,
    saving,
    skipBlurRef,
    setDraft: (next: string) => {
      setDraft(next)
      setDirty(true)
    },
    cancel,
    save,
  }
}

export function SavedSelectField({
  id,
  label,
  value,
  options,
  placeholder,
  mutation,
  resetKey,
  onSave,
}: {
  id: string
  label: string
  value: string
  options: readonly SelectOption[]
  placeholder?: string
  mutation: MutationState
  resetKey: string | number
  onSave: (value: string) => Promise<boolean>
}) {
  const pending = mutation.status === 'pending'
  const [draft, setDraft] = useState(value)
  const [dirty, setDirty] = useState(false)
  useEffect(() => {
    setDraft(value)
    setDirty(false)
  }, [resetKey])
  useEffect(() => {
    if (!dirty) setDraft(value)
  }, [dirty, value])
  return (
    <WorkItemsSelect
      id={id}
      label={label}
      value={draft}
      disabled={pending}
      placeholder={placeholder}
      options={options}
      onValueChange={(next) => {
        if (next === draft) return
        setDraft(next)
        setDirty(true)
        void onSave(next).then((saved) => {
          if (saved) setDirty(false)
        })
      }}
    />
  )
}

export function DateField({
  id,
  label,
  value,
  mutation,
  resetKey,
  onSave,
}: {
  id: string
  label: string
  value?: string
  mutation: MutationState
  resetKey: string | number
  onSave: (value: string | null) => Promise<boolean>
}) {
  const source = value || ''
  const field = useBlurSaveDraft(source, resetKey)
  const pending = mutation.status === 'pending' || field.saving
  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <DatePicker id={id} value={field.draft} disabled={pending}
        onChange={(next) => {
          field.setDraft(next)
          if (next !== source) void field.save(next || null, onSave)
        }}
      />
    </div>
  )
}

export function EstimatedMinutesField({
  value,
  mutation,
  resetKey,
  onSave,
}: {
  value?: number
  mutation: MutationState
  resetKey: string | number
  onSave: (value: number) => Promise<boolean>
}) {
  const source = value === undefined ? '' : String(value)
  const field = useBlurSaveDraft(source, resetKey)
  const pending = mutation.status === 'pending' || field.saving
  const commit = async (intent: 'blur' | 'submit') => {
    if (!field.dirty || field.draft === source) return true
    const parsed = Number(field.draft)
    if (field.draft === '' || !isValidEstimatedMinutes(parsed)) {
      if (intent === 'blur') field.cancel()
      return false
    }
    return field.save(parsed, onSave)
  }
  return (
    <div className="grid gap-1.5">
      <Label htmlFor="work-items-detail-estimated-minutes">{WORK_ITEMS_COPY.detail.estimatedTime}</Label>
      <Input
        id="work-items-detail-estimated-minutes"
        type="number"
        min={0}
        max={999_960}
        value={field.draft}
        disabled={pending}
        onChange={(event) => field.setDraft(event.currentTarget.value)}
        onBlur={() => {
          if (field.skipBlurRef.current) {
            field.skipBlurRef.current = false
            return
          }
          void commit('blur')
        }}
        onKeyDown={(event) => {
          if (event.key === 'Escape') {
            event.preventDefault()
            field.cancel()
            event.currentTarget.blur()
          } else if (event.key === 'Enter' && !event.nativeEvent.isComposing) {
            event.preventDefault()
            field.skipBlurRef.current = true
            const input = event.currentTarget
            void commit('submit').finally(() => input.blur())
          }
        }}
      />
    </div>
  )
}
