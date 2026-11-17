import { readThemePaint, withAlpha } from '@/shared/ui/theme/appearance'

type ThemePaint = ReturnType<typeof readThemePaint>

let cachedMode: string | null = null
let cachedPaint: ThemePaint | null = null

function paint(): ThemePaint {
  const mode = typeof document === 'undefined'
    ? 'light'
    : (document.documentElement.dataset.uiMode ?? 'light')
  if (mode !== cachedMode || !cachedPaint) {
    cachedPaint = readThemePaint()
    cachedMode = mode
  }
  return cachedPaint
}

export { withAlpha }

/**
 * Chart palette resolved from the shared chart semantic tokens
 * (`--design-chart-*`, see design-tokens.css). Getters re-resolve whenever the
 * active ui-mode changes so every chart follows light/dark automatically.
 */
export const COLORS = {
  get blue() { return paint().chartSeries[0] },
  get teal() { return paint().chartSeries[1] },
  get green() { return paint().chartSeries[1] },
  get tealSoft() { return paint().chartSeries[1] },
  get amber() { return paint().chartSeries[2] },
  get violet() { return paint().chartSeries[3] },
  get purple() { return paint().chartSeries[3] },
  get red() { return paint().chartSeries[4] },
  get orange() { return paint().chartNegative },
  get gray() { return paint().chartSeries[5] },
  get slate() { return paint().chartSeries[5] },
  get blueSoft() { return paint().chartSeries[0] },
  get border() { return paint().border },
  get grid() { return paint().chartGrid },
  get text() { return paint().foreground },
  get subtle() { return paint().muted },
} as const

const FONT = 'Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", sans-serif'

function entranceDelay(dataIndex: number) {
  return Math.min(Math.max(0, dataIndex) * 55, 420)
}

export function safeCount(value: number | undefined | null) {
  return Math.max(0, Number(value) || 0)
}

function truncateLabel(value: string | number, maxLength = 12) {
  const characters = Array.from(String(value ?? ''))
  return characters.length > maxLength
    ? `${characters.slice(0, maxLength).join('')}...`
    : characters.join('')
}

function landingPreview() {
  return typeof document !== 'undefined'
    && Boolean(document.querySelector('[data-slot="landing-live-frame"]'))
}

export function base(description: string): Readonly<Record<string, unknown>> {
  const paint = readThemePaint()
  const preview = landingPreview()
  return {
    animation: !preview,
    animationThreshold: 2000,
    animationDuration: preview ? 0 : 1150,
    animationDurationUpdate: preview ? 0 : 420,
    animationDelay: preview ? 0 : entranceDelay,
    animationDelayUpdate: 0,
    animationEasing: 'cubicOut',
    animationEasingUpdate: 'cubicOut',
    textStyle: { color: paint.foreground, fontFamily: FONT, fontSize: 12 },
    aria: { enabled: true, description, decal: { show: false } },
  }
}

export function itemTooltip() {
  const paint = readThemePaint()
  return {
    trigger: 'item',
    confine: true,
    backgroundColor: paint.card,
    borderColor: paint.border,
    borderWidth: 1,
    padding: [8, 10],
    textStyle: { color: paint.foreground, fontFamily: FONT, fontSize: 12 },
    extraCssText: 'box-shadow: var(--design-shadow-popover);',
  }
}

export function axisTooltip() {
  return {
    ...itemTooltip(),
    trigger: 'axis',
    axisPointer: { type: 'shadow', shadowStyle: { color: paint().chartGrid } },
  }
}

export function valueAxis(maxValue: number) {
  return {
    type: 'value',
    min: 0,
    max: Math.max(1, Math.ceil(maxValue * 1.18)),
    minInterval: 1,
    axisLine: { show: false },
    axisTick: { show: false },
    axisLabel: { color: COLORS.subtle, fontSize: 11, hideOverlap: true },
    splitLine: { lineStyle: { color: COLORS.grid, width: 1 } },
  }
}

export function categoryAxis(labels: readonly string[], maxLength: number) {
  return {
    type: 'category',
    inverse: true,
    data: labels,
    axisLine: { show: false },
    axisTick: { show: false },
    axisLabel: {
      color: COLORS.subtle,
      fontSize: 11,
      margin: 10,
      formatter: (value: string | number) => truncateLabel(value, maxLength),
    },
  }
}
