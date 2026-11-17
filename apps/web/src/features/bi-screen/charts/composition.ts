import { BI_SCREEN_COPY } from '@/shared/copy'

import type { CompositionGroup, CompositionModel, DashboardData, DistributionItem } from '../types'
import { safeCount } from './core'

export function compositionModel(data: DashboardData): CompositionModel {
  const copy = BI_SCREEN_COPY.charts
  const definitions: readonly CompositionGroup[] = [
    {
      id: 'status',
      label: copy.status.title,
      total: 0,
      items: data.status_progress,
    },
    {
      id: 'priority',
      label: copy.priority.title,
      total: 0,
      items: data.priority_distribution,
    },
    {
      id: 'type',
      label: copy.type.title,
      total: 0,
      items: data.type_distribution,
      drilldownQuery: 'filter_kinds',
    },
    {
      id: 'due',
      label: copy.due.title,
      total: 0,
      items: data.due_distribution,
    },
  ]
  const groups = definitions
    .map((group) => {
      const items = group.items.filter((item) => safeCount(item.count) > 0)
      return { ...group, items, total: items.reduce((sum, item) => sum + safeCount(item.count), 0) }
    })
    .filter((group) => group.items.length > 0 && group.total > 0)
  return {
    id: 'composition',
    title: copy.composition.title,
    description: copy.composition.description,
    meta: copy.composition.meta(groups.length),
    ariaLabel: copy.composition.ariaLabel,
    emptyTitle: copy.composition.emptyTitle,
    emptyDescription: copy.composition.emptyDescription,
    groups,
    summaryCaption: copy.composition.summaryCaption,
    summaryColumns: [BI_SCREEN_COPY.chart.dimension, BI_SCREEN_COPY.chart.category, BI_SCREEN_COPY.chart.value],
    summaryRows: groups.flatMap((group) => group.items.map((item: DistributionItem) => [
      group.label,
      item.label,
      safeCount(item.count),
    ])),
  }
}
