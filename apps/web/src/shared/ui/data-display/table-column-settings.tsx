import { useState } from 'react'

import { copy } from '@/shared/copy'
import type { TableColumnOption, TableColumnPreference } from '@/shared/model/table-column-preferences'

import { Button } from '../primitives/button'
import { PreferenceFieldList } from './preference-field-list'
import { ResponsivePanel } from '../layout/responsive-panel'

const text = copy.ui.tableColumns

export function TableColumnSettings<TId extends string>({
  columns,
  preference,
  onVisibleChange,
  onMove,
  onReorder,
  onReset,
}: {
  columns: readonly TableColumnOption<TId>[]
  preference: TableColumnPreference<TId>
  onVisibleChange: (id: TId, visible: boolean) => void
  onMove: (id: TId, direction: -1 | 1) => void
  onReorder: (id: TId, targetId: TId) => void
  onReset: () => void
}) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button type="button" variant="link" size="sm" className="px-1" onClick={() => setOpen(true)}>
        {text.trigger}
      </Button>
      <ResponsivePanel
        open={open}
        onOpenChange={setOpen}
        title={text.title}
        description={text.description}
        footer={(
          <Button type="button" variant="outline" onClick={onReset}>{text.reset}</Button>
        )}
      >
        <PreferenceFieldList
          options={columns}
          preference={preference}
          onVisibleChange={onVisibleChange}
          onMove={onMove}
          onReorder={onReorder}
        />
      </ResponsivePanel>
    </>
  )
}
