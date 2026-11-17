import { useState } from 'react'

import { REPORTING_COPY } from '@/shared/copy'
import { Button, DetailSection, StatusPill } from '@/shared/ui'

import type { CodeActivity, CommitQualityAssessment, CommitReview } from '../model/types'
import { Metric } from './Metric'

// 提交明细内联展示的条数上限：超过时折叠，用「展开全部提交」切换。
const INLINE_COMMIT_LIMIT = 8

// 历史聚合行没有构成拆分（四类全 0），此时只展示总数，避免误读成零构成。
function codeClassSum(code: CodeActivity): number {
  return code.sourceAdditions + code.testAdditions + code.docAdditions + code.configAdditions
}

// 提交明细行：日期、主题、增删行与调试/TODO 命中，让报告读者能直接核对
// AI 的点评依据。
function CommitRow({ commit }: { commit: CommitReview }) {
  return (
    <li className="flex flex-wrap items-center gap-2 text-xs">
      <span className="shrink-0 text-muted-foreground">{commit.day}</span>
      <span className="min-w-0 flex-1 truncate text-foreground">{commit.subject}</span>
      <span className="shrink-0 font-mono text-muted-foreground">
        {REPORTING_COPY.code.commitDelta(commit.additions, commit.deletions)}
      </span>
      {commit.debugHits > 0 ? (
        <StatusPill tone="warning">{REPORTING_COPY.code.debugHitChip(commit.debugHits)}</StatusPill>
      ) : null}
      {commit.todoHits > 0 ? (
        <StatusPill tone="info">{REPORTING_COPY.code.todoChip(commit.todoHits)}</StatusPill>
      ) : null}
    </li>
  )
}

// 逐提交 AI 点评：专用审查技能对每一条提交的结论（评级 + 一句话 + 问题）。
function CommitVerdictList({ assessment }: { assessment: CommitQualityAssessment }) {
  if (!assessment.commits || assessment.commits.length === 0) return null
  return (
    <div className="border-t border-border py-2">
      <p className="text-xs font-medium text-muted-foreground">{REPORTING_COPY.code.verdictsTitle}</p>
      <ul className="mt-1 grid gap-1.5" aria-label={REPORTING_COPY.code.verdictsTitle}>
        {assessment.commits.map((commit) => (
          <li key={commit.sha} className="rounded-md bg-muted/50 px-2 py-1.5">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-mono text-xs text-muted-foreground">{commit.sha.slice(0, 8)}</span>
              <StatusPill
                tone={commit.verdict === '优' ? 'success' : commit.verdict === '良' ? 'info' : commit.verdict === '中' ? 'warning' : 'danger'}
              >
                {commit.verdict}
              </StatusPill>
              {commit.summary ? (
                <span className="min-w-0 flex-1 truncate text-xs text-foreground">{commit.summary}</span>
              ) : null}
            </div>
            {commit.issues && commit.issues.length > 0 ? (
              <p className="mt-0.5 text-xs text-muted-foreground">{commit.issues.join('；')}</p>
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  )
}

// AI 代码质量评估：专用审查技能的结构化结论（生成完成后随运行状态返回），
// 与确定性卫生扣分互相印证。
function AiAssessmentBlock({ assessment }: { assessment: CommitQualityAssessment }) {
  return (
    <div className="border-t border-border py-2" aria-label={REPORTING_COPY.code.aiAssessmentTitle}>
      <div className="flex items-center gap-2">
        <p className="text-sm font-medium text-foreground">{REPORTING_COPY.code.aiAssessmentTitle}</p>
        <StatusPill tone={assessment.score >= 80 ? 'success' : assessment.score >= 60 ? 'warning' : 'danger'}>
          {REPORTING_COPY.code.aiAssessmentScore} {assessment.score}
        </StatusPill>
      </div>
      <p className="mt-1 text-xs leading-5 text-foreground">{assessment.conclusion}</p>
      {(assessment.commonIssues?.length ?? 0) > 0 ? (
        <p className="mt-1 text-xs text-muted-foreground">
          {REPORTING_COPY.code.aiCommonIssues}{(assessment.commonIssues ?? []).join('；')}
        </p>
      ) : null}
      {(assessment.risks?.length ?? 0) > 0 ? (
        <p className="mt-1 text-xs text-muted-foreground">
          {REPORTING_COPY.code.aiRisks}{(assessment.risks ?? []).join('；')}
        </p>
      ) : null}
      {(assessment.suggestions?.length ?? 0) > 0 ? (
        <p className="mt-1 text-xs text-muted-foreground">
          {REPORTING_COPY.code.aiSuggestions}{(assessment.suggestions ?? []).join('；')}
        </p>
      ) : null}
    </div>
  )
}

export function CodeActivityPanel({
  code,
  aiAssessment,
}: {
  code: CodeActivity
  aiAssessment?: CommitQualityAssessment
}) {
  const [expanded, setExpanded] = useState(false)
  const visibleCommits = expanded
    ? code.commitReviews
    : code.commitReviews.slice(0, INLINE_COMMIT_LIMIT)
  return (
    <DetailSection title={REPORTING_COPY.code.title} className="content-start gap-2 p-3 shadow-xs">
      <div className="grid grid-cols-3 gap-2 pb-2 sm:grid-cols-6">
        <Metric label={REPORTING_COPY.code.repos} value={code.repos} />
        <Metric label={REPORTING_COPY.code.prsMerged} value={code.prsMerged} />
        <Metric label={REPORTING_COPY.code.commits} value={code.commits} />
        <Metric label={REPORTING_COPY.code.additions} value={code.additions} />
        <Metric label={REPORTING_COPY.code.deletions} value={code.deletions} />
      </div>
      {codeClassSum(code) > 0 ? (
        <div className="border-t border-border grid grid-cols-2 gap-2 py-2 sm:grid-cols-4">
          <Metric label={REPORTING_COPY.code.sourceLines} value={code.sourceAdditions} />
          <Metric label={REPORTING_COPY.code.testLines} value={code.testAdditions} />
          <Metric label={REPORTING_COPY.code.docLines} value={code.docAdditions} />
          <Metric label={REPORTING_COPY.code.configLines} value={code.configAdditions} />
        </div>
      ) : null}
      {code.commitReviews.length > 0 ? (
        <div className="border-t border-border py-2">
          <div className="flex items-center justify-between gap-2">
            <p className="text-xs font-medium text-muted-foreground">{REPORTING_COPY.code.commitsTitle}</p>
            {code.commitReviews.length > INLINE_COMMIT_LIMIT ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setExpanded((value) => !value)}
              >
                {expanded ? REPORTING_COPY.code.commitsHide : REPORTING_COPY.code.commitsShow}
              </Button>
            ) : null}
          </div>
          <ul className="mt-1 grid gap-1" aria-label={REPORTING_COPY.code.commitsTitle}>
            {visibleCommits.map((commit) => (
              <CommitRow key={commit.sha} commit={commit} />
            ))}
          </ul>
          <p className="mt-2 text-xs text-muted-foreground">{REPORTING_COPY.code.aiReviewNote}</p>
        </div>
      ) : null}
      {aiAssessment ? (
        <>
          <AiAssessmentBlock assessment={aiAssessment} />
          <CommitVerdictList assessment={aiAssessment} />
        </>
      ) : null}
      {code.freshness.degraded ? (
        <div className="border-t border-border py-2">
          <p className="text-xs text-muted-foreground">
            {REPORTING_COPY.code.degraded(code.freshness.error)}
          </p>
        </div>
      ) : null}
    </DetailSection>
  )
}
