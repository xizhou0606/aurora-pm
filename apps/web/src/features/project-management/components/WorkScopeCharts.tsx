import { copy } from '@/shared/copy'
import { LazyEChart, readThemePaint, withAlpha } from '@/shared/ui'
import type { ProjectWorkBreakdown } from '../model'

const FONT = 'Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", sans-serif'
// Status colors resolve from the shared chart semantic tokens so the donuts
// follow the active light/dark palette instead of a local hardcoded set.
const COLORS = {
  get done() { return readThemePaint().chartPositive },
  get active() { return readThemePaint().chartSeries[0] },
  get backlog() { return readThemePaint().chartSeries[5] },
  get cancelled() { return withAlpha(readThemePaint().chartSeries[5], 0.55) },
  get critical() { return readThemePaint().chartNegative },
  get normal() { return readThemePaint().chartSeries[2] },
  get minor() { return readThemePaint().chartSeries[5] },
} as const
type Slice = {
  name: string
  value: number
  color: string
}

const workCopy = copy.projectManagement.overview.workBreakdown
const chartCopy = workCopy.charts
const taskCopy = copy.projectManagement.overview.taskExecution

function donutOption(input: {
  ariaLabel: string
  center: string
  subtitle: string
  slices: readonly Slice[]
}) {
  const visible = input.slices.filter((slice) => slice.value > 0)
  if (visible.length === 0) return null
  const paint = readThemePaint()
  return {
    animation: true,
    animationDuration: 700,
    animationEasing: 'cubicOut',
    color: visible.map((slice) => slice.color),
    textStyle: { color: paint.foreground, fontFamily: FONT, fontSize: 12 },
    aria: { enabled: true, description: input.ariaLabel, decal: { show: false } },
    tooltip: {
      trigger: 'item',
      confine: true,
      backgroundColor: paint.card,
      borderColor: paint.border,
      borderWidth: 1,
      padding: [8, 10],
      textStyle: { color: paint.foreground, fontFamily: FONT, fontSize: 12 },
    },
    legend: {
      type: 'scroll',
      left: 'center',
      bottom: 0,
      itemWidth: 8,
      itemHeight: 8,
      itemGap: 12,
      icon: 'circle',
      textStyle: { color: paint.muted, fontSize: 11 },
    },
    title: {
      text: input.center,
      subtext: input.subtitle,
      left: 'center',
      top: '28%',
      textStyle: { color: paint.foreground, fontFamily: FONT, fontSize: 22, fontWeight: 650 },
      subtextStyle: { color: paint.muted, fontFamily: FONT, fontSize: 11, lineHeight: 18 },
    },
    series: [{
      type: 'pie',
      radius: ['54%', '74%'],
      center: ['50%', '40%'],
      startAngle: 90,
      minAngle: 4,
      itemStyle: { borderColor: paint.card, borderWidth: 3, borderRadius: 4 },
      label: { show: false },
      labelLine: { show: false },
      data: visible.map((slice) => ({ name: `${slice.name} ${slice.value}`, value: slice.value })),
    }],
  }
}

function chartProps(ariaLabel: string, slices: readonly Slice[]) {
  return {
    ariaLabel,
    emptyTitle: chartCopy.empty,
    emptyDescription: chartCopy.emptyDescription,
    loadingLabel: chartCopy.loading,
    loadErrorLabel: chartCopy.loadError,
    retryLabel: copy.ui.retry,
    summary: {
      caption: chartCopy.summary,
      columns: [chartCopy.statusColumn, chartCopy.countColumn],
      rows: slices.map((slice) => [slice.name, slice.value]),
    },
  }
}

export function WorkScopeCharts({ work }: { work: ProjectWorkBreakdown }) {
  const requirementSlices: Slice[] = [
    { name: chartCopy.requirementDone, value: work.requirements.done, color: COLORS.done },
    { name: chartCopy.requirementActive, value: work.requirements.active, color: COLORS.active },
    { name: chartCopy.requirementBacklog, value: work.requirements.backlog, color: COLORS.backlog },
  ]
  const taskSlices: Slice[] = [
    { name: taskCopy.todo, value: work.tasks.todo, color: COLORS.backlog },
    { name: taskCopy.active, value: work.tasks.active, color: COLORS.active },
    { name: taskCopy.done, value: work.tasks.done, color: COLORS.done },
    { name: taskCopy.cancelled, value: work.tasks.cancelled, color: COLORS.cancelled },
  ]
  const bugSlices: Slice[] = [
    { name: chartCopy.bugCritical, value: work.bugs.critical, color: COLORS.critical },
    { name: chartCopy.bugNormal, value: work.bugs.normal, color: COLORS.normal },
    { name: chartCopy.bugMinor, value: work.bugs.minor, color: COLORS.minor },
  ]
  const taskPercent = work.tasks.total > 0
    ? Math.round((work.tasks.done / work.tasks.total) * 100)
    : 0

  const charts = [
    {
      id: 'requirements',
      title: chartCopy.requirementTitle,
      option: donutOption({
        ariaLabel: chartCopy.requirementAria(
          work.requirements.done,
          work.requirements.active,
          work.requirements.backlog,
        ),
        center: String(work.requirements.total),
        subtitle: chartCopy.total,
        slices: requirementSlices,
      }),
      props: chartProps(
        chartCopy.requirementAria(
          work.requirements.done,
          work.requirements.active,
          work.requirements.backlog,
        ),
        requirementSlices,
      ),
    },
    {
      id: 'tasks',
      title: chartCopy.taskTitle,
      option: donutOption({
        ariaLabel: taskCopy.ariaLabel(
          work.tasks.todo,
          work.tasks.active,
          work.tasks.done,
          work.tasks.cancelled,
        ),
        center: workCopy.taskDoneValue(taskPercent),
        subtitle: workCopy.taskDone,
        slices: taskSlices,
      }),
      props: chartProps(
        taskCopy.ariaLabel(
          work.tasks.todo,
          work.tasks.active,
          work.tasks.done,
          work.tasks.cancelled,
        ),
        taskSlices,
      ),
    },
    {
      id: 'bugs',
      title: chartCopy.bugTitle,
      option: donutOption({
        ariaLabel: chartCopy.bugAria(work.bugs.critical, work.bugs.normal, work.bugs.minor),
        center: String(work.bugs.open),
        subtitle: chartCopy.total,
        slices: bugSlices,
      }),
      props: chartProps(
        chartCopy.bugAria(work.bugs.critical, work.bugs.normal, work.bugs.minor),
        bugSlices,
      ),
    },
  ]

  return (
    <section
      aria-label={workCopy.heading}
      data-testid="pm-work-charts"
      className="grid min-w-0 overflow-hidden rounded-lg border border-border bg-card sm:grid-cols-3"
    >
      {charts.map((chart, index) => (
        <article
          key={chart.id}
          data-testid={`pm-work-chart-${chart.id}`}
          className={index === 0
            ? 'min-w-0'
            : 'min-w-0 border-t border-border sm:border-t-0 sm:border-l'}
        >
          <h3 className="px-4 pt-4 text-sm font-semibold text-foreground">{chart.title}</h3>
          <LazyEChart {...chart.props} option={chart.option} size="compact" />
        </article>
      ))}
    </section>
  )
}
