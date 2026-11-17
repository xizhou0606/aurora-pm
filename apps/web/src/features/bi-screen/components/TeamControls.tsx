import { BI_SCREEN_COPY } from '@/shared/copy'
import {
  Button,
  Select,
  SelectContent,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
  ViewTabs,
} from '@/shared/ui'

import type { TeamDimensionControl, TeamSliceControl } from '../types'
import { teamDimensionOptionLabel } from '../model'

const ALL = 'all'

export function TeamControls({
  dimension,
  slice,
  onOpenMetricSettings,
  onOpenScoreSettings,
}: {
  dimension: TeamDimensionControl
  slice: TeamSliceControl
  onOpenMetricSettings?: () => void
  onOpenScoreSettings?: () => void
}) {
  const copy = BI_SCREEN_COPY.team
  const allLabel = dimension.value === 'version'
    ? copy.allVersions
    : dimension.value === 'month' ? copy.allMonths : copy.allWeeks
  const selectItems = [
    { value: ALL, label: allLabel },
    ...slice.options.map((option) => ({
      value: option.key,
      label: teamDimensionOptionLabel(dimension.value, option),
    })),
  ]
  return (
    <div
      className="flex min-w-0 flex-1 flex-col gap-2 sm:flex-row sm:items-center sm:justify-end"
      aria-busy={dimension.loading || slice.loading || undefined}
    >
      <ViewTabs
        value={dimension.value}
        label={copy.dimensionLabel}
        tabs={[
          { id: 'version', label: copy.versionDimension },
          { id: 'month', label: copy.monthDimension },
          { id: 'week', label: copy.weekDimension },
        ]}
        onValueChange={(value) => dimension.onChange(
          value === 'month' || value === 'week' ? value : 'version',
        )}
      />
      <Select<string>
        items={selectItems}
        value={slice.value ?? ALL}
        onValueChange={(value) => slice.onChange(!value || value === ALL ? undefined : value)}
      >
        <SelectLabel>{copy.filterLabel}</SelectLabel>
        <SelectTrigger
          aria-label={copy.filterLabel}
          className={dimension.value === 'week' ? 'w-full sm:w-72' : 'w-full sm:w-48'}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent
          className={dimension.value === 'week'
            ? 'w-max min-w-[var(--anchor-width)]'
            : undefined}
        >
          <SelectItem value={ALL}>{allLabel}</SelectItem>
          {slice.options.map((option) => (
            <SelectItem key={option.key} value={option.key}>
              {teamDimensionOptionLabel(dimension.value, option)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {onOpenMetricSettings ? <Button
        type="button"
        variant="link"
        size="sm"
        className="h-8 shrink-0 px-1"
        onClick={onOpenMetricSettings}
      >
        {copy.metricSettings.open}
      </Button> : null}
      {onOpenScoreSettings ? <Button
        type="button"
        variant="link"
        size="sm"
        className="h-8 shrink-0 px-1"
        onClick={onOpenScoreSettings}
      >
        {copy.settings}
      </Button> : null}
    </div>
  )
}
