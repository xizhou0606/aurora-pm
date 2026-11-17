import { useState } from 'react'

import { QUALITY_COPY } from '@/shared/copy'
import type { TableColumnPreferenceControl } from '@/shared/hooks'
import { Button, PreferenceSection, ResponsivePanel } from '@/shared/ui'
import {
  QUALITY_FILTER_CONTROL_OPTIONS,
  QUALITY_STATUS_TAB_OPTIONS,
  type QualityFilterControlId,
  type QualityStatusTabId,
} from '../../model/control-preferences'

export function FilterSettings({
  statusTabs,
  filterControls,
}: {
  statusTabs: TableColumnPreferenceControl<QualityStatusTabId>
  filterControls: TableColumnPreferenceControl<QualityFilterControlId>
}) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button type="button" variant="link" size="sm" className="px-1" onClick={() => setOpen(true)}>
        {QUALITY_COPY.filterSettings.trigger}
      </Button>
      <ResponsivePanel
        open={open}
        onOpenChange={setOpen}
        title={QUALITY_COPY.filterSettings.title}
        className="max-w-4xl"
      >
        <div className="grid min-w-0 gap-4 md:grid-cols-2">
          <PreferenceSection
            title={QUALITY_COPY.filterSettings.statusCard}
            options={QUALITY_STATUS_TAB_OPTIONS}
            resetLabel={QUALITY_COPY.filterSettings.reset}
            onReset={statusTabs.reset}
            preference={statusTabs.preference}
            onVisibleChange={statusTabs.setVisible}
            onMove={statusTabs.move}
            onReorder={statusTabs.moveTo}
          />
          <PreferenceSection
            title={QUALITY_COPY.filterSettings.controlsCard}
            options={QUALITY_FILTER_CONTROL_OPTIONS}
            resetLabel={QUALITY_COPY.filterSettings.reset}
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
