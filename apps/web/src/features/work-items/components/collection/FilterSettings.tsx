import { useState } from 'react'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { TableColumnPreferenceControl } from '@/shared/hooks'
import { Button, PreferenceSection, ResponsivePanel } from '@/shared/ui'
import {
  WORK_ITEMS_FILTER_CONTROL_OPTIONS,
  WORK_ITEMS_VIEW_CONTROL_OPTIONS,
  type WorkItemsFilterControlId,
  type WorkItemsViewControlId,
} from '../../model/control-preferences'

export function FilterSettings({
  viewControls,
  filterControls,
}: {
  viewControls: TableColumnPreferenceControl<WorkItemsViewControlId>
  filterControls: TableColumnPreferenceControl<WorkItemsFilterControlId>
}) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button type="button" variant="link" size="sm" className="px-1" onClick={() => setOpen(true)}>
        {WORK_ITEMS_COPY.filterSettings.trigger}
      </Button>
      <ResponsivePanel
        open={open}
        onOpenChange={setOpen}
        title={WORK_ITEMS_COPY.filterSettings.title}
        className="max-w-4xl"
      >
        <div className="grid min-w-0 gap-4 md:grid-cols-2">
          <PreferenceSection
            title={WORK_ITEMS_COPY.filterSettings.viewCard}
            options={WORK_ITEMS_VIEW_CONTROL_OPTIONS}
            resetLabel={WORK_ITEMS_COPY.filterSettings.reset}
            onReset={viewControls.reset}
            preference={viewControls.preference}
            onVisibleChange={viewControls.setVisible}
            onMove={viewControls.move}
            onReorder={viewControls.moveTo}
          />
          <PreferenceSection
            title={WORK_ITEMS_COPY.filterSettings.controlsCard}
            options={WORK_ITEMS_FILTER_CONTROL_OPTIONS}
            resetLabel={WORK_ITEMS_COPY.filterSettings.reset}
            onReset={filterControls.reset}
            preference={filterControls.preference}
            onVisibleChange={filterControls.setVisible}
            onMove={filterControls.move}
            onReorder={filterControls.moveTo}
          />
        </div>
      </ResponsivePanel>
    </>
  )
}
