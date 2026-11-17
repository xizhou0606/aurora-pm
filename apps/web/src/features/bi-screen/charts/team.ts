import { BI_SCREEN_COPY } from '@/shared/copy'

import type { ChartModel } from '../types'
import type {
  TeamDisplayMember,
  TeamDisplayMetric,
  TeamRoleKey,
} from '../model/team-display'
import { isSeverityShareMetric, sortTeamMembers, teamMetricValue } from '../model/team-display'
import { axisTooltip, base, COLORS, safeCount, valueAxis } from './core'

export function teamMetricChart({
  role,
  roleLabel,
  members,
  metric,
  descending,
}: {
  role: TeamRoleKey
  roleLabel: string
  members: readonly TeamDisplayMember[]
  metric: TeamDisplayMetric
  descending: boolean
}): ChartModel {
  const copy = BI_SCREEN_COPY.team
  const visibleMembers = sortTeamMembers(members, metric, descending)
    .filter((member) => teamMetricValue(member, metric).value !== undefined)
    .slice(0, 10)
  const values = visibleMembers.map((member) => safeCount(teamMetricValue(member, metric).value))
  const maximum = Math.max(1, ...values)
  const title = copy.topTenTitle(metric.label)
  const severityShareMetric = isSeverityShareMetric(metric)
  return {
    id: `team-${role}-${metric.key}`,
    title,
    description: copy.topTenDescription(roleLabel, metric.label, descending),
    meta: copy.chartMeta(visibleMembers.length),
    ariaLabel: copy.topTenAria(roleLabel, metric.label),
    emptyTitle: copy.emptyChartTitle,
    emptyDescription: copy.emptyChartDescription,
    option: visibleMembers.length ? {
      ...base(copy.topTenAria(roleLabel, metric.label)),
      tooltip: axisTooltip(),
      grid: { left: 8, right: severityShareMetric ? 72 : 18, top: 12, bottom: 10, containLabel: true },
      xAxis: {
        ...valueAxis(maximum),
        axisLabel: {
          color: COLORS.subtle,
          fontSize: 11,
          hideOverlap: true,
          formatter: metric.unit === 'percent' ? '{value}%' : '{value}',
        },
      },
      yAxis: {
        type: 'category',
        inverse: true,
        data: visibleMembers.map((member) => member.user_name),
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: { color: COLORS.subtle, fontSize: 11, width: 84, overflow: 'truncate' },
      },
      series: [{
        name: metric.label,
        type: 'bar',
        barMaxWidth: 16,
        cursor: 'default',
        data: severityShareMetric
          ? visibleMembers.map((member) => ({
              value: safeCount(teamMetricValue(member, metric).value),
              teamSharePct: teamMetricValue(member, metric).supportingValue,
            }))
          : values,
        label: severityShareMetric ? {
          show: true,
          position: 'right',
          color: COLORS.subtle,
          fontSize: 11,
          formatter: ({ data }: { data?: { teamSharePct?: number } }) => (
            data?.teamSharePct === undefined ? '' : copy.percent(data.teamSharePct)
          ),
        } : { show: false },
        itemStyle: { color: COLORS.blue, borderRadius: [0, 3, 3, 0] },
      }],
    } : null,
    summaryCaption: copy.topTenSummary(roleLabel, metric.label),
    summaryColumns: severityShareMetric
		? [copy.member, metric.label, copy.severityTeamShareLabel]
      : [copy.member, metric.label],
    summaryRows: visibleMembers.map((member) => {
      const result = teamMetricValue(member, metric)
      return severityShareMetric
        ? [
            member.user_name,
            result.value ?? copy.noSample,
            result.supportingValue === undefined
              ? copy.noSample
              : copy.percent(result.supportingValue),
          ]
        : [member.user_name, result.value ?? copy.noSample]
    }),
    points: visibleMembers.map((member) => ({
      label: member.user_name,
      values: [
        { series: metric.label, value: teamMetricValue(member, metric).value ?? 0 },
        ...(severityShareMetric && teamMetricValue(member, metric).supportingValue !== undefined
          ? [{
			  series: copy.severityTeamShareLabel,
              value: teamMetricValue(member, metric).supportingValue ?? 0,
            }]
          : []),
      ],
    })),
    size: 'medium',
  }
}
