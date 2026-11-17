import { useState } from 'react'
import { Icon } from '@/shared/ui'
import { useAttachmentThumbnail } from '../../hooks/grid/use-attachment-thumbnail'
import type { GridProps } from '../../model/grid-port'
import type { AttachmentRef, CellAddress } from '../../model/types'

export function AttachmentChip({ attachment, address, load }: { attachment: AttachmentRef; address: CellAddress; load: GridProps['onLoadAttachmentPreview'] }) {
  const { ref, source } = useAttachmentThumbnail(address, attachment.id, attachment.mimeType.startsWith('image/'), load)
  const [failedSource, setFailedSource] = useState('')
  return <span ref={ref} title={attachment.name} className="inline-flex min-w-0 max-w-full items-center gap-1 rounded border border-border bg-muted/40 px-1 text-xs leading-5">
    {source && source !== failedSource ? <img src={source} alt="" className="h-6 w-8 shrink-0 rounded-sm object-cover" onError={() => setFailedSource(source)} /> : <Icon name="hugeicons:file-01" size={14} className="shrink-0 text-muted-foreground" />}
    <span className="truncate">{attachment.name}</span>
  </span>
}
