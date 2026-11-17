import { REPORTING_COPY } from '@/shared/copy'
import { DetailSection } from '@/shared/ui'

import type { BugReportMetrics } from '../model/types'
import { Metric } from './Metric'

const metricKeys = ['total', 'p0', 'p1', 'p2'] as const

// 测试岗位的核心事实面板：本期提出的缺陷按严重级别分档（总数 + P0/P1/P2，
// P3 不单列）。与代码活动面板互斥——后端按岗位焦点保证二者只出现其一。
export function BugReportPanel({ metrics }: { metrics: BugReportMetrics }) {
  const copy = REPORTING_COPY.bugReport
  return (
    <DetailSection
      title={copy.title}
      hint={copy.footnote}
      className="content-start gap-2 p-3 shadow-xs"
      contentClassName="grid grid-cols-2 gap-2 sm:grid-cols-4"
    >
      {metricKeys.map((key) => <Metric key={key} label={copy[key]} value={metrics[key]} />)}
    </DetailSection>
  )
}
