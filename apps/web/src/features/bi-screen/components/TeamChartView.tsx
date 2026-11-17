import { useMemo } from 'react'

import { teamMetricChart } from '../charts'
import type {
  TeamDisplayMember,
  TeamDisplayMetric,
  TeamRoleKey,
} from '../model/team-display'
import { ChartPanel } from './ChartPanel'

export function TeamChartView({
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
}) {
  const chart = useMemo(() => teamMetricChart({
    role,
    roleLabel,
    members,
    metric,
    descending,
  }), [descending, members, metric, role, roleLabel])
  return <ChartPanel model={chart} fluid className="h-full min-h-[28rem] xl:min-h-0" />
}
