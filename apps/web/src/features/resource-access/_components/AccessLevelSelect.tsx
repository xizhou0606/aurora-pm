import { RESOURCE_ACCESS_COPY as copy } from '@/shared/copy'
import { Label, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui'

import type { ResourceAccessLevel } from '../model'

export function AccessLevelSelect({
  id,
  value,
  disabled,
  onChange,
}: {
  id: string
  value: ResourceAccessLevel
  disabled?: boolean
  onChange: (level: ResourceAccessLevel) => void
}) {
  return (
    <div className="grid gap-2">
      <Label htmlFor={id}>{copy.level.label}</Label>
      <Select
        value={value}
        disabled={disabled}
        onValueChange={(next) => {
          if (next === 'viewer' || next === 'editor') onChange(next)
        }}
      >
        <SelectTrigger id={id}><SelectValue>{copy.level[value]}</SelectValue></SelectTrigger>
        <SelectContent>
          <SelectItem value="viewer">{copy.level.viewer}</SelectItem>
          <SelectItem value="editor">{copy.level.editor}</SelectItem>
        </SelectContent>
      </Select>
    </div>
  )
}
