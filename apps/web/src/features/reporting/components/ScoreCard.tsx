import { REPORTING_COPY } from '@/shared/copy'
import { DetailSection, LazyEChart } from '@/shared/ui'

import { scoreRadarOption } from '../model/scoreRadar'
import type { ReportScoreDimension, ReportScores } from '../model/types'

// 得分分档只用于着色，不改变分数本身；颜色走语义 token 并带暗色变体。
function scoreTone(score: number): string {
  if (score >= 80) return 'text-emerald-600 dark:text-emerald-400'
  if (score >= 60) return 'text-amber-600 dark:text-amber-400'
  return 'text-red-600 dark:text-red-400'
}

export function ScoreRadarPanel({ scores }: { scores: ReportScores }) {
  const copy = REPORTING_COPY.scores
  return (
    <DetailSection
      title={copy.title}
      hint={copy.footnote}
      actions={<span className="rounded-md bg-muted px-2 py-0.5 text-xs font-semibold tabular-nums text-foreground">{copy.overallLabel} {scores.overall}</span>}
      className="content-start gap-2 p-3 shadow-xs"
    >
      <LazyEChart
        option={scoreRadarOption(scores)}
        ariaLabel={copy.radarAria}
        emptyTitle={copy.radarEmpty}
        loadingLabel={copy.radarLoading}
        loadErrorLabel={copy.radarLoadError}
        retryLabel={copy.retry}
        size="compact"
        summary={{
          caption: copy.summaryCaption,
          columns: [copy.dimensionColumn, copy.scoreColumn, copy.basisColumn],
          rows: scores.dimensions.map((dimension) => [dimension.label, dimension.score, dimension.basis]),
        }}
      />
    </DetailSection>
  )
}

function ScoreDimensionRow({ dimension }: { dimension: ReportScoreDimension }) {
  return (
    <li className="rounded-md bg-muted/50 px-2.5 py-2">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm text-foreground">{dimension.label}</span>
        <span className={`text-sm font-semibold tabular-nums ${scoreTone(dimension.score)}`}>
          {dimension.score}
        </span>
      </div>
      <p className="mt-0.5 text-xs leading-5 text-muted-foreground">{dimension.basis}</p>
    </li>
  )
}

export function ScoreDimensionsPanel({ scores }: { scores: ReportScores }) {
  const copy = REPORTING_COPY.scores
  return (
    <DetailSection title={copy.dimensionTitle} className="content-start gap-2 p-3 shadow-xs">
      <ul className="grid content-start gap-2">
        {scores.dimensions.map((dimension) => (
          <ScoreDimensionRow key={dimension.key} dimension={dimension} />
        ))}
      </ul>
    </DetailSection>
  )
}
