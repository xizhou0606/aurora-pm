import type { ColumnDef } from '@tanstack/react-table'

import { QUALITY_COPY } from '@/shared/copy'
import { Button, Checkbox, HoverPreviewText } from '@/shared/ui'
import type { QualityBug } from '../../model/types'

export type ListLinkColumnsDeps = {
  canSelectBug: (bug: QualityBug) => boolean
  onOpen: (bug: QualityBug, mode?: 'view' | 'edit') => void
  onOpenRequirement: (requirementId: number) => void
  onToggleSelected: (bug: QualityBug) => void
  selectedIds: readonly number[]
}

export function linkColumns(deps: ListLinkColumnsDeps): ColumnDef<QualityBug>[] {
  const { canSelectBug, onOpen, onOpenRequirement, onToggleSelected, selectedIds } = deps
  return [
    {
      id: 'selection',
      size: 48,
      header: QUALITY_COPY.bulk.selection,
      cell: ({ row }) => (
        <Checkbox
          checked={selectedIds.includes(row.original.id)}
          disabled={!canSelectBug(row.original)}
          aria-label={QUALITY_COPY.bulk.selectRow(row.original.title)}
          onCheckedChange={() => onToggleSelected(row.original)}
        />
      ),
    },
    {
      id: 'key',
      size: 128,
      header: QUALITY_COPY.list.key,
      cell: ({ row }) => (
        <Button
          type="button"
          variant="link"
          size="sm"
          className="h-auto min-w-24 justify-start p-0 font-mono text-xs font-semibold text-primary no-underline hover:underline"
          aria-label={QUALITY_COPY.list.open(row.original.title)}
          onClick={() => onOpen(row.original, 'view')}
        >
          {row.original.bugKey || QUALITY_COPY.format.fallbackId(row.original.id)}
        </Button>
      ),
    },
    {
      id: 'title',
      accessorKey: 'title',
      size: 288,
      header: QUALITY_COPY.list.title,
      cell: ({ row }) => (
        <Button
          type="button"
          variant="link"
          size="sm"
          className="h-auto min-w-0 max-w-full justify-start p-0 text-left font-normal text-primary no-underline hover:underline"
          aria-label={QUALITY_COPY.list.openTitle(row.original.title)}
          onClick={() => onOpen(row.original, 'view')}
        >
          <HoverPreviewText text={row.original.title} />
        </Button>
      ),
    },
    {
      id: 'requirement',
      size: 192,
      header: QUALITY_COPY.list.requirement,
      cell: ({ row }) => {
        const requirementId = row.original.requirementId
        if (!requirementId) return QUALITY_COPY.list.notSet
        const requirement = row.original.requirementTitle
          || QUALITY_COPY.list.fallbackRequirement(requirementId)
        return (
          <Button
            type="button"
            variant="link"
            size="sm"
            className="h-auto min-w-0 max-w-full justify-start p-0 text-left font-normal text-primary no-underline hover:underline"
            aria-label={QUALITY_COPY.list.openRequirement(requirement)}
            title={requirement}
            onClick={() => onOpenRequirement(requirementId)}
          >
            <span className="block min-w-0 truncate">{requirement}</span>
          </Button>
        )
      },
    },
  ]
}
