import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { Label, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui'
import type { DocumentNode } from '../../model'

export function DocumentTreeDestinationField({
  id,
  label,
  parentId,
  folders,
  onChange,
}: {
  id: string
  label: string
  parentId?: number
  folders: readonly DocumentNode[]
  onChange: (parentId?: number) => void
}) {
  return (
    <div className="grid gap-2">
      <Label htmlFor={id}>{label}</Label>
      <Select
        value={parentId ? String(parentId) : 'root'}
        onValueChange={(value) => onChange(value && value !== 'root' ? Number(value) : undefined)}
      >
        <SelectTrigger id={id}><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem value="root">{DOCUMENTS_COPY.move.root}</SelectItem>
          {folders.map((folder) => (
            <SelectItem key={folder.id} value={String(folder.id)}>{folder.title}</SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
