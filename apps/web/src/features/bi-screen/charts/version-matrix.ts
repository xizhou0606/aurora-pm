import { BI_SCREEN_COPY } from '@/shared/copy'

import type { DashboardData, VersionMatrixModel, VersionMatrixModule, VersionMatrixTone } from '../types'
import { safeCount } from './core'

const MAX_MATRIX_MODULES = 8

function completionTone(pct: number): VersionMatrixTone {
  if (pct >= 80) return 'positive'
  if (pct >= 45) return 'info'
  return 'warning'
}

function statusFor(
  pct: number,
  overdue: number,
): { statusLabel: string; statusTone: VersionMatrixTone } {
  if (overdue > 0) return { statusLabel: BI_SCREEN_COPY.charts.matrix.delayed, statusTone: 'danger' }
  const tone = completionTone(pct)
  const statusLabel = tone === 'positive'
    ? BI_SCREEN_COPY.charts.matrix.leading
    : tone === 'info'
      ? BI_SCREEN_COPY.charts.matrix.normal
      : BI_SCREEN_COPY.charts.matrix.lagging
  return { statusLabel, statusTone: tone }
}

/**
 * 版本交付矩阵：每个版本一张 HUD 模块卡——完成度色顶线、大号滚动百分数、
 * 刻度进度尺与三状态微数字；右上角状态签标示延期/领先/正常/滞后。
 */
export function versionMatrixModel(data: DashboardData): VersionMatrixModel {
  const copy = BI_SCREEN_COPY.charts.matrix
  const modules: VersionMatrixModule[] = [...data.version_progress]
    .sort((left, right) => safeCount(right.done_rate_pct) - safeCount(left.done_rate_pct))
    .slice(0, MAX_MATRIX_MODULES)
    .map((item) => {
      const pct = Math.min(100, Math.max(0, Math.round(safeCount(item.done_rate_pct))))
      const overdue = safeCount(item.overdue_tasks)
      const { statusLabel, statusTone } = statusFor(pct, overdue)
      return {
        id: item.id,
        label: item.label,
        pct,
        done: safeCount(item.done_tasks),
        remaining: Math.max(0, safeCount(item.total_tasks) - safeCount(item.done_tasks)),
        overdue,
        statusLabel,
        tone: completionTone(pct),
        statusTone,
        drilldown: { domain: 'tasks', scope: { projectIds: [item.id], requirementIds: [] } },
      }
    })
  return {
    id: 'matrix',
    title: copy.title,
    description: copy.description,
    meta: copy.meta(modules.length),
    ariaLabel: copy.ariaLabel,
    emptyTitle: copy.emptyTitle,
    emptyDescription: copy.emptyDescription,
    modules,
  }
}
