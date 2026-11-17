import { REPORTING_COPY } from '@/shared/copy'
import { DetailSection, StatusPill } from '@/shared/ui'

// 综合评语：替代机械的评分维度清单。AI 在报告生成时针对工作情况、代码
// 产出与代码质量给出综合分析，并按岗位侧重给出 0-100 综合评分；生成前
// 显示占位说明。
export function OverallCommentPanel({
  overallComment,
  overallScore,
}: {
  overallComment?: string
  overallScore?: number
}) {
  return (
    <DetailSection
      title={REPORTING_COPY.code.overallCommentTitle}
      className="content-start gap-2 p-3 shadow-xs"
      actions={typeof overallScore === 'number' ? (
        <StatusPill
          tone={overallScore >= 80 ? 'success' : overallScore >= 60 ? 'warning' : 'danger'}
        >
          {REPORTING_COPY.code.overallScoreLabel} {overallScore}
        </StatusPill>
      ) : null}
    >
      {overallComment ? (
        <p className="text-sm leading-6 text-foreground">{overallComment}</p>
      ) : (
        <p className="text-xs text-muted-foreground">
          {REPORTING_COPY.code.overallCommentPlaceholder}
        </p>
      )}
    </DetailSection>
  )
}
