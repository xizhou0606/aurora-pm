import { useState } from 'react'

import { copy } from '@/shared/copy'
import type { TableColumnPreferenceControl } from '@/shared/hooks'
import { Button, PreferenceSection, ResponsivePanel } from '@/shared/ui'
import {
  REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS,
  type RequirementPoolFilterControlId,
} from '../utils/requirement-pool-filter-fields'

const poolCopy = copy.aiPM.requirementPool

export function RequirementPoolFilterSettings({
  filterControls,
}: {
  filterControls: TableColumnPreferenceControl<RequirementPoolFilterControlId>
}) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button type="button" variant="link" size="sm" className="px-1" onClick={() => setOpen(true)}>
        {poolCopy.filterSettings.trigger}
      </Button>
      <ResponsivePanel
        open={open}
        onOpenChange={setOpen}
        title={poolCopy.filterSettings.title}
        className="max-w-xl"
      >
        <PreferenceSection
          title={poolCopy.filterSettings.controlsCard}
          resetLabel={poolCopy.filterSettings.reset}
          onReset={filterControls.reset}
          options={REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS}
          preference={filterControls.preference}
          onVisibleChange={filterControls.setVisible}
          onMove={filterControls.move}
          onReorder={filterControls.moveTo}
        />
      </ResponsivePanel>
    </>
  )
}
