import { REPORTING_COPY } from '@/shared/copy'
import { DetailSection } from '@/shared/ui'

import type { UIBugMetrics } from '../model/types'
import { Metric } from './Metric'

const metricKeys = ['resolved', 'open', 'overdue'] as const

// 设计岗位的核心事实面板：UI 域缺陷处理情况（解决按本期统计，存量/逾期为
// 当前快照）。与代码活动面板互斥——后端按岗位焦点保证二者只出现其一。
// 数据来自「UI 域缺陷」的流程约定：走查/视觉问题录缺陷时需选择 UI 域。
export function UiBugPanel({ metrics }: { metrics: UIBugMetrics }) {
  const copy = REPORTING_COPY.uiBug
  return (
    <DetailSection
      title={copy.title}
      hint={copy.footnote}
      className="content-start gap-2 p-3 shadow-xs"
      contentClassName="grid grid-cols-3 gap-2"
    >
      {metricKeys.map((key) => <Metric key={key} label={copy[key]} value={metrics[key]} />)}
    </DetailSection>
  )
}
