import { QUALITY_COPY } from '@/shared/copy'
import { EmptyState, UploadFileCard } from '@/shared/ui'
import type { QualityMaterial } from '../../model/types'
import { materialDisplayName } from '../../model/presenters'

export function Materials({
  items,
  onPreview,
  onRemove,
}: {
  items: readonly QualityMaterial[]
  onPreview: (url: string, trigger: HTMLElement) => void
  onRemove?: (item: QualityMaterial) => void
}) {
  return (
    <div className="grid min-w-0 gap-3">
      {!items.length ? <EmptyState title={QUALITY_COPY.detail.noMaterials} /> : (
        <ul className="flex min-w-0 flex-wrap gap-3">
          {items.map((item) => (
            <UploadFileCard
              key={item.url}
              item={item}
              name={materialDisplayName(item.url)}
              disabled={false}
              onPreview={(_entry, url, trigger) => onPreview(url, trigger)}
              onRemove={onRemove}
            />
          ))}
        </ul>
      )}
    </div>
  )
}
