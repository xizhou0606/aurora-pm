import { useCallback, useEffect, useRef, useState } from 'react'

import { REPORTING_COPY } from '@/shared/copy'
import { AiLoadingOverlay, Button, DetailSection, EmptyState, LoadingSkeleton, StatusPill } from '@/shared/ui'

import { BugReportPanel } from './BugReportPanel'
import { CodeActivityPanel } from './CodeActivityPanel'
import { GroupRankingPanel } from './GroupRankingPanel'
import { Metric } from './Metric'
import { MarkdownView } from './markdown'
import { OverallCommentPanel } from './OverallCommentPanel'
import { RequirementPanel } from './RequirementPanel'
import { ScoreRadarPanel } from './ScoreCard'
import { UiBugPanel } from './UiBugPanel'
import { composeReportText, periodTypeLabel, periodRangeLabel } from '../model/period'
import type { CodeActivity, ExportedDocument, MemberReport, ReportPreview, ReportTarget, ReportingRun } from '../model/types'
import type { ReportingListState } from '../hooks'

type MemberReportPanelProps = {
  selected: { kind: 'member' | 'group' | 'department'; id: number; name: string } | null
  periodType: string
  previewState: ReportingListState<ReportPreview>
  facts: MemberReport | null
  runState: ReportingListState<ReportingRun> | null
  generating: boolean
  canGenerate: boolean
  canPublish: boolean
  onGenerate: () => void
  onExportFeishu: () => void
  feishuUploading: boolean
  exportedDocument: ExportedDocument | null
}

function downloadReport(report: MemberReport, narrative: string): void {
  const sections = [composeReportText(report)]
  if (narrative.trim()) sections.push('', REPORTING_COPY.report.aiCommentTitle, '', narrative.trim())
  const blob = new Blob([sections.join('\n')], {
    type: 'text/markdown;charset=utf-8',
  })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = `report-${report.period.type}-${report.period.anchorDate}.md`
  anchor.click()
  URL.revokeObjectURL(url)
}

function CopySummaryButton({ summary }: { summary: string }) {
  const [copied, setCopied] = useState(false)
  const timerRef = useRef<number | null>(null)
  useEffect(() => () => {
    if (timerRef.current !== null) window.clearTimeout(timerRef.current)
  }, [])
  const onCopy = useCallback(() => {
    const finish = () => {
      setCopied(true)
      timerRef.current = window.setTimeout(() => setCopied(false), 2_000)
    }
    if (navigator.clipboard?.writeText) {
      void navigator.clipboard.writeText(summary).then(finish, () => setCopied(false))
    }
  }, [summary])
  return (
    <Button type="button" variant="outline" size="sm" onClick={onCopy}>
      {copied ? REPORTING_COPY.report.copied : REPORTING_COPY.report.copy}
    </Button>
  )
}

function RunQueryError({ onRetry }: { onRetry: () => void }) {
  return (
    <section className="space-y-2" role="alert">
      <h4 className="text-sm font-medium text-foreground">{REPORTING_COPY.report.runQueryErrorTitle}</h4>
      <Button type="button" variant="outline" size="sm" onClick={onRetry}>
        {REPORTING_COPY.report.resumeQuery}
      </Button>
    </section>
  )
}

// 报告运行失败时的可见错误（保持在页面最顶部，成功态内容从工作事实开始）。
function ReportFailedNotice({ run }: { run: ReportingRun }) {
  if (run.status !== 'failed') return null
  return (
    <section className="space-y-2" role="alert">
      <h4 className="text-sm font-medium text-foreground">{REPORTING_COPY.report.failedTitle}</h4>
      <StatusPill tone="danger">{run.errorMessage || run.status}</StatusPill>
    </section>
  )
}

// 成员头衔区的角色 chips：生效角色（权限角色）优先，每个角色一枚中性小标签；
// 无生效角色时回退飞书职务并加「职务」小标注区分来源；两者都空则不渲染。
function TargetRoleChips({ target }: { target: ReportTarget }) {
  const roles = target.roleNames ?? []
  if (roles.length === 0 && !target.jobTitle) return null
  return (
    <div
      className="flex flex-wrap items-center gap-1"
      aria-label={REPORTING_COPY.report.rolesAria}
      data-testid="reporting-target-roles"
    >
      {roles.length > 0
        ? roles.map((role) => (
          <StatusPill key={role} tone="neutral">{role}</StatusPill>
        ))
        : (
          <>
            <StatusPill tone="neutral">{target.jobTitle}</StatusPill>
            <StatusPill tone="neutral" className="min-w-0 px-1 py-0">
              {REPORTING_COPY.directory.jobTitleBadge}
            </StatusPill>
          </>
        )}
    </div>
  )
}

export function MemberReportPanel({
  selected,
  previewState,
  facts,
  runState,
  generating,
  canGenerate,
  canPublish,
  onGenerate,
  onExportFeishu,
  feishuUploading,
  exportedDocument,
}: MemberReportPanelProps) {
  if (!selected || selected.id <= 0) {
    return (
      <div className="flex flex-1 items-center justify-center p-6">
        <EmptyState title={REPORTING_COPY.report.emptyTitle} name="hugeicons:user-multiple" />
      </div>
    )
  }
  if (previewState.status === 'loading' || previewState.status === 'unavailable') {
    return (
      <div className="space-y-3 p-6" role="status" aria-busy="true">
        <LoadingSkeleton rows={6} />
      </div>
    )
  }
  if (previewState.status === 'error') {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-3 p-6">
        <EmptyState role="alert" title={REPORTING_COPY.report.previewErrorTitle} />
        <Button type="button" variant="outline" size="sm" onClick={previewState.retry}>
          {REPORTING_COPY.report.retry}
        </Button>
      </div>
    )
  }
  // 上面已拦截 loading/unavailable/error，此处必为 ready。
  const report = facts ?? previewState.data.report
  const run = runState?.status === 'ready' ? runState.data : null
  const activeRun = run !== null && run.status !== 'completed' && run.status !== 'failed'
  // 固定五段顺序（团队/个人一致）：工作事实+评分雷达 → 综合评语 → 岗位焦点
  // 事实（测试岗提出 Bug / 产品岗需求 / 设计岗 UI 缺陷 / 其余代码活动）→
  // 成员排行（仅组/部门视图）→ 汇报正文（最下）。
  const isCollective = report.target.kind !== 'member'
  const rollups = report.members ?? []
  // 汇报正文：完成态展示 AI 叙述；未生成/失败时回退到确定性事实正文，
  // 复制/导出/飞书上传在任何状态下都可用（runId=0 事实版上传）。
  const bodyText = run?.status === 'completed' && run.summary.trim()
    ? run.summary
    : composeReportText(report)
  return (
    <div className="flex flex-1 flex-col gap-4 p-4 sm:p-6">
      {(generating || activeRun) && (
        <AiLoadingOverlay
          size={40}
          label={run !== null && run.status === 'queued'
            ? REPORTING_COPY.report.queuedLabel
            : REPORTING_COPY.report.runningLabel}
        />
      )}
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          {report.target.kind === 'member' && report.target.avatarUrl ? (
            <img
              src={report.target.avatarUrl}
              alt=""
              aria-hidden="true"
              className="size-10 shrink-0 rounded-full object-cover"
            />
          ) : null}
          <div className="min-w-0 space-y-0.5">
            <h2 className="truncate text-base font-semibold text-foreground">
              {report.target.name}
              {report.target.kind === 'group' && (
                <StatusPill tone="accent">{REPORTING_COPY.report.groupBadge}</StatusPill>
              )}
              {report.target.kind === 'department' && (
                <StatusPill tone="accent">{REPORTING_COPY.report.departmentBadge}</StatusPill>
              )}
            </h2>
            <p className="truncate text-xs text-muted-foreground">
              {periodTypeLabel(report.period.type)}
              {REPORTING_COPY.report.metaSeparator}
              {periodRangeLabel(report.period)}
              {report.target.subLabel ? (
                <>
                  {REPORTING_COPY.report.metaSeparator}
                  {report.target.subLabel}
                </>
              ) : null}
            </p>
            <TargetRoleChips target={report.target} />
          </div>
        </div>
        <Button
          type="button"
          size="sm"
          onClick={onGenerate}
          disabled={!canGenerate || generating || activeRun}
        >
          {generating || activeRun ? REPORTING_COPY.report.runningTitle : REPORTING_COPY.report.generate}
        </Button>
      </header>

      {run !== null ? <ReportFailedNotice run={run} /> : null}

      {/* 第一行：工作事实预览 + 多维评分雷达，团队/个人一致的两列网格。 */}
      <div className="grid gap-3 lg:grid-cols-2">
        <DetailSection
          title={REPORTING_COPY.report.previewTitle}
          className="content-start gap-2 p-3 shadow-xs"
          contentClassName="grid grid-cols-2 gap-2 sm:grid-cols-4 xl:grid-cols-2"
        >
            <Metric label={REPORTING_COPY.metrics.tasksCompleted} value={report.tasks.completed} />
            <Metric label={REPORTING_COPY.metrics.tasksCreated} value={report.tasks.created} />
            <Metric label={REPORTING_COPY.metrics.tasksActive} value={report.tasks.active} />
            <Metric label={REPORTING_COPY.metrics.tasksOverdue} value={report.tasks.overdue} />
            <Metric label={REPORTING_COPY.metrics.bugsResolved} value={report.bugs.resolved} />
            <Metric label={REPORTING_COPY.metrics.bugsClosed} value={report.bugs.closed} />
            <Metric label={REPORTING_COPY.metrics.bugsOpen} value={report.bugs.open} />
            <Metric label={REPORTING_COPY.metrics.bugsOverdue} value={report.bugs.overdue} />
        </DetailSection>
        <ScoreRadarPanel scores={report.scores} />
      </div>

      {/* 第二行：综合评语 + AI 综合评分（所有视图一致）。 */}
      <OverallCommentPanel overallComment={run?.overallComment} overallScore={run?.overallScore} />

      {/* 第三行：岗位焦点事实（测试岗提出 Bug / 产品岗需求 / 设计岗 UI 缺陷）
          优先；都没有时回退代码活动（未接入代码采集时不渲染）。后端保证互斥。 */}
      {report.bugReports ? (
        <BugReportPanel metrics={report.bugReports} />
      ) : report.requirements ? (
        <RequirementPanel metrics={report.requirements} />
      ) : report.uiBugs ? (
        <UiBugPanel metrics={report.uiBugs} />
      ) : report.code ? (
        <CodeActivityPanel code={report.code} aiAssessment={run?.commitQuality} />
      ) : null}

      {/* 第四行：成员排行（仅组/部门视图）。测试/产品/设计团队把代码列换成
          岗位核心指标列；key 让团队类型切换时重置排序状态。 */}
      {isCollective && rollups.length > 0 ? (
        <GroupRankingPanel
          key={report.bugReports
            ? 'testing'
            : report.requirements
              ? 'product'
              : report.uiBugs
                ? 'design'
                : 'default'}
          members={rollups}
          code={report.code}
          bugReports={report.bugReports}
          requirements={report.requirements}
          uiBugs={report.uiBugs}
          comments={run?.dimensionComments}
        />
      ) : null}

      {/* 第五行：汇报正文，固定在最下。 */}
      <section className="space-y-2">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h3 className="text-sm font-medium text-foreground">{REPORTING_COPY.report.summaryTitle}</h3>
          <div className="flex items-center gap-2">
            <CopySummaryButton summary={bodyText} />
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => downloadReport(report, run?.summary ?? '')}
            >
              {REPORTING_COPY.report.export}
            </Button>
            {exportedDocument ? (
              <a
                href={exportedDocument.url}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center rounded-md border border-border px-3 py-1.5 text-xs font-medium text-foreground hover:bg-muted"
              >
                {REPORTING_COPY.report.feishuUploaded}
              </a>
            ) : (
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={!canPublish || feishuUploading}
                onClick={onExportFeishu}
              >
                {feishuUploading ? REPORTING_COPY.report.feishuUploading : REPORTING_COPY.report.feishuUpload}
              </Button>
            )}
          </div>
        </div>
        <div className="rounded-md border border-border bg-card p-3 text-sm leading-6 text-foreground [&_h1]:mb-2 [&_h1]:text-base [&_h1]:font-semibold [&_h2]:mb-2 [&_h2]:mt-4 [&_h2]:text-sm [&_h2]:font-semibold [&_h3]:mb-1 [&_h3]:mt-3 [&_h3]:text-sm [&_h3]:font-medium [&_ul]:list-disc [&_ul]:pl-5 [&_ol]:list-decimal [&_ol]:pl-5 [&_li]:my-0.5 [&_p]:my-1.5 [&_strong]:font-semibold [&_blockquote]:border-l-2 [&_blockquote]:border-border [&_blockquote]:pl-3 [&_blockquote]:text-muted-foreground">
          <MarkdownView source={bodyText} />
        </div>
      </section>

      {runState?.status === 'error' && (
        <RunQueryError onRetry={runState.retry} />
      )}

    </div>
  )
}
