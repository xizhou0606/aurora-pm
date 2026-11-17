import { REPORTING_COPY } from '@/shared/copy'
import { DetailSection } from '@/shared/ui'

import type { RequirementMetrics } from '../model/types'
import { Metric } from './Metric'

const metricKeys = ['total', 'createdInPeriod', 'done', 'acceptance', 'development', 'pending'] as const

// 产品岗位的核心事实面板：负责的需求总数、本期新增与状态分布（当前快照
// 口径，需求快照没有完成时间字段）。与代码活动面板互斥——后端按岗位焦点
// 保证二者只出现其一。
export function RequirementPanel({ metrics }: { metrics: RequirementMetrics }) {
  const copy = REPORTING_COPY.requirement
  return (
    <DetailSection
      title={copy.title}
      hint={copy.footnote}
      className="content-start gap-2 p-3 shadow-xs"
      contentClassName="grid grid-cols-2 gap-2 sm:grid-cols-3"
    >
      {metricKeys.map((key) => <Metric key={key} label={copy[key]} value={metrics[key]} />)}
    </DetailSection>
  )
}
