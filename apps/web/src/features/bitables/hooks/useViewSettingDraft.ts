import { useRef, useState } from 'react'
import type { View, ViewConfig } from '../model/types'

export type ViewSettingKey = 'rowHeight' | 'groupFieldId' | 'filters' | 'sorts'

export function useViewSettingDraft({ view, setting, pending, readOnly, onSave, onClose }: {
  view: View; setting: ViewSettingKey; pending: boolean; readOnly: boolean
  onSave: (id: string, config: ViewConfig) => Promise<boolean>; onClose: () => void
}) {
  const [draft, setDraft] = useState(view.config)
  const [phase, setPhase] = useState<'editing' | 'saving' | 'error'>('editing')
  const saving = useRef(false)
  const disabled = pending || readOnly || phase === 'saving'
  const save = async (value: ViewConfig = draft) => {
    if (disabled || saving.current) return
    saving.current = true; setPhase('saving')
    // Preserve updates to unrelated controls received while this popover was open.
    const next = { ...view.config, [setting]: value[setting] }
    try { if (await onSave(view.id, next)) { onClose(); return } } catch { /* Command owner provides detailed feedback. */ }
    saving.current = false; setPhase('error')
  }
  return { draft, setDraft, phase, disabled, save }
}
