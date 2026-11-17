import { useState } from 'react'
import { BITABLE_GRID_COPY as copy } from '@/shared/copy/bitables-grid'
import { Button, Checkbox, Icon, LazyFileUploader, Popover, PopoverContent, PopoverTrigger, UserIdentity } from '@/shared/ui'
import { cn } from '@/shared/lib/utils'
import { formatDateOnly } from '@/shared/lib/date-boundary'
import { ChoiceTag } from './choice-tag'
import { AttachmentChip } from './attachment-chip'
import type { AttachmentRef, CellValue, Field, Row, UserRef } from '../../model/types'
import type { GridProps } from '../../model/grid-port'

export function GridCellDisplay({ field, row, value, text, height, capabilities, pending, onUpload, onPreview, onDownload, onLoadPreview }: {
  field: Field; row: Row; value: CellValue; text: string; height: number; pending: boolean
  capabilities: GridProps['capabilities']; onUpload: GridProps['onUploadAttachment']; onPreview: GridProps['onPreviewAttachment']; onDownload: GridProps['onDownloadAttachment']
  onLoadPreview?: GridProps['onLoadAttachmentPreview']
}) {
  const [open, setOpen] = useState(false)
  const address = { recordId: row.id, fieldId: field.id }
  if (field.type === 'attachment') {
    const attachments = Array.isArray(value) ? value.filter((item): item is AttachmentRef => typeof item === 'object' && typeof item.id === 'string' && 'downloadPath' in item) : []
    return <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger render={<Button size="xs" variant="ghost" className="h-full w-full min-h-(--cell-min-h) min-w-0 cursor-pointer justify-start gap-1 overflow-hidden rounded-none px-2 font-normal" style={{ '--cell-min-h': `${height}px` }} />} aria-label={text ? `${copy.attachments}: ${text}` : copy.attachments} title={text || copy.attachments}>
        {attachments.length ? <><AttachmentChip attachment={attachments[0]} address={address} load={onLoadPreview} />{attachments.length > 1 ? <span className="shrink-0 text-xs text-muted-foreground">{copy.moreValues(attachments.length - 1)}</span> : null}</> : <Icon name="hugeicons:add-01" size={14} className="rounded-sm bg-muted p-0.5 text-muted-foreground" />}
        <Icon name="hugeicons:arrow-down-01" size={14} className="ml-auto shrink-0 text-muted-foreground" />
      </PopoverTrigger>
      <PopoverContent className="w-96 overflow-y-auto p-2" aria-label={copy.attachments} onPointerDown={event => event.stopPropagation()} onClick={event => event.stopPropagation()} onKeyDown={event => event.stopPropagation()} onDoubleClick={event => event.stopPropagation()} onDrop={event => event.stopPropagation()}>
        <div className="grid gap-2">{attachments.map((attachment) => <div key={attachment.id} className="flex items-center gap-2 rounded border border-border p-2">
          <span className="min-w-0 flex-1 truncate text-sm">{attachment.name}</span>
          {onPreview && attachment.mimeType.startsWith('image/') ? <Button size="xs" variant="outline" onClick={() => onPreview(address, attachment.id)}>{copy.preview}</Button> : null}
          {onDownload ? <Button size="xs" variant="outline" onClick={() => onDownload(address, attachment.id)}>{copy.download}</Button> : null}
        </div>)}</div>
        {onUpload && capabilities.canUpdateRecords && capabilities.canUploadAttachments && capabilities.writableFieldIds.includes(field.id) && !field.readOnly ? <LazyFileUploader label={copy.upload} compact={{ dropLabel: copy.attachmentDrop, addLabel: copy.attachmentAdd }} pending={pending} onUpload={(file) => onUpload(address, row.fieldRevisions[field.id], file)} /> : null}
      </PopoverContent>
    </Popover>
  }
  if (field.type === 'user' || field.type === 'created_by' || field.type === 'last_modified_by') {
    const people = Array.isArray(value) ? value.filter((item): item is UserRef => typeof item === 'object' && typeof item.id === 'number') : []
    return <div title={text} aria-label={text} className="flex min-w-0 max-h-(--cell-max-h) items-center gap-1 overflow-hidden px-2" style={{ '--cell-max-h': `${height}px` }}>{people.slice(0, 2).map((person) => <UserIdentity key={person.id} name={person.name} avatarUrl={person.avatarUrl} size="sm" />)}{people.length > 2 ? <span className="shrink-0 text-xs text-muted-foreground">{copy.moreValues(people.length - 2)}</span> : null}</div>
  }
  if (field.type === 'single_select' || field.type === 'multiple_select') {
    const ids = Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : typeof value === 'string' ? [value] : []
    return <div title={text} aria-label={text} className="flex min-w-0 items-center gap-1 overflow-hidden px-2 py-1">{ids.slice(0, 2).map(id => <ChoiceTag key={id} field={field} id={id} />)}{ids.length > 2 ? <span className="shrink-0 text-xs text-muted-foreground">{copy.moreValues(ids.length - 2)}</span> : null}</div>
  }
  if (field.type === 'checkbox') return <span className="flex items-center px-2"><Checkbox aria-label={field.name} checked={value === true} disabled className="disabled:opacity-100" /></span>
  const numeric = field.type === 'number'
  const date = ['date', 'created_time', 'last_modified_time'].includes(field.type)
  const label = date ? formatDateOnly(text) || text : text
  return <div title={text} className={cn('max-h-(--cell-max-h) overflow-hidden px-2 py-1 text-sm', field.type === 'long_text' ? 'whitespace-pre-wrap break-words' : 'truncate', (numeric || date) && 'tabular-nums', numeric && 'text-right')} style={{ '--cell-max-h': `${height}px` }}>{label}</div>
}
