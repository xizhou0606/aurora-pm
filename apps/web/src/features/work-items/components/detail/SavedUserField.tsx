import { useEffect, useMemo, useState } from 'react'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import { UserPreview, UserSelect } from '@/shared/ui'

import type { MutationState, WorkItemUserOption } from '../../model/types'
import { workItemUserSelectOption } from '../../model/utils'

export function SavedUserField({
  id,
  label,
  value,
  displayName,
  options,
  editable,
  mutation,
  resetKey,
  onSave,
}: {
  id: string
  label: string
  value?: number
  displayName?: string
  options: readonly WorkItemUserOption[]
  editable: boolean
  mutation: MutationState
  resetKey: string | number
  onSave: (value: number | null) => Promise<boolean>
}) {
  const [draft, setDraft] = useState(value)
  const [dirty, setDirty] = useState(false)
  const currentOption = options.find((option) => option.id === value)
  const selectOptions = useMemo(() => {
    const normalized = options.map(workItemUserSelectOption)
    if (!value || currentOption || !displayName) return normalized
    return [{ id: value, name: displayName }, ...normalized]
  }, [currentOption, displayName, options, value])

  useEffect(() => {
    setDraft(value)
    setDirty(false)
  }, [resetKey])
  useEffect(() => {
    if (dirty && draft === value) setDirty(false)
    else if (!dirty) setDraft(value)
  }, [dirty, draft, value])

  if (!editable) {
    return value ? (
      <UserPreview
        name={currentOption?.label || displayName || WORK_ITEMS_COPY.list.unassigned}
        avatarUrl={currentOption?.avatarUrl}
        size="sm"
      />
    ) : <span>{WORK_ITEMS_COPY.list.unassigned}</span>
  }

  return (
    <UserSelect
      id={id}
      label={label}
      value={draft}
      options={selectOptions}
      placeholder={WORK_ITEMS_COPY.list.unassigned}
      disabled={mutation.status === 'pending'}
      onValueChange={(next) => {
        if (next === undefined) return
        if (next === draft) return
        setDraft(next)
        setDirty(true)
        void onSave(next).then((saved) => {
          if (!saved) {
            setDraft(value)
            setDirty(false)
          }
        })
      }}
    />
  )
}
