import { QUALITY_COPY } from '@/shared/copy'
import { MetricStrip } from '@/shared/ui'
import type { QualitySummary } from '../../model/types'

export function Metrics({
  summary,
  todoCount,
}: {
  summary: QualitySummary
  todoCount: number
}) {
  return <MetricStrip items={[
    { id: 'total', label: QUALITY_COPY.metrics.total, value: summary.total },
    { id: 'open', label: QUALITY_COPY.metrics.open, value: todoCount },
    { id: 'handling', label: QUALITY_COPY.metrics.handling, value: summary.handling },
    { id: 'review', label: QUALITY_COPY.metrics.review, value: summary.review },
    { id: 'closed', label: QUALITY_COPY.metrics.closed, value: summary.closed },
    { id: 'high', label: QUALITY_COPY.metrics.highSeverity, value: summary.highSeverity },
  ]} />
}
