import { DatePicker } from '@/shared/ui'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { REPORTING_COPY } from '@/shared/copy'
import { Button, EmptyState, OperationalPage, ViewTabs } from '@/shared/ui'
import {
  REPORTING_PERIOD_TYPES,
  type ExportedDocument,
  type MemberReport,
  type ReportingDirectory,
  type ReportingMember,
  type ReportingPeriodType,
  type StartReportResult,
} from '../model/types'
import type { FeishuExportInput, ReportGenerateInput } from '../hooks'
import { isValidAnchorDate, yesterdayAnchorDate } from '../model/period'
import {
  useFeishuExportMutation,
  useGroupReportPreviewQuery,
  useDepartmentReportPreviewQuery,
  useMemberReportPreviewQuery,
  useReportGenerateMutation,
  useReportRunQuery,
  useReportingDirectoryQuery,
  useReportingOperationAccess,
} from '../hooks'
import type { ReportSelection } from '../components/MemberDirectory'
import { MemberDirectory } from '../components/MemberDirectory'
import { MemberReportPanel } from '../components/MemberReportPanel'
import {
  useInjectedReportingEnvironment,
  usePMReportingEnvironment,
  type ReportingEnvironment,
} from '../components/environment'

export function ReportingPage({ onRuntimeFailure }: { onRuntimeFailure?: (error: unknown) => void }) {
  const injected = useInjectedReportingEnvironment()
  return injected
    ? <ReportingContent environment={injected} onRuntimeFailure={onRuntimeFailure} />
    : <PMReportingPage onRuntimeFailure={onRuntimeFailure} />
}

function PMReportingPage({ onRuntimeFailure }: { onRuntimeFailure?: (error: unknown) => void }) {
  const environment = usePMReportingEnvironment()
  return <ReportingContent environment={environment} onRuntimeFailure={onRuntimeFailure} />
}

function ReportingContent({
  environment,
  onRuntimeFailure,
}: {
  environment: ReportingEnvironment
  onRuntimeFailure?: (error: unknown) => void
}) {
  const access = useReportingOperationAccess()
  const directory = useReportingDirectoryQuery(environment, access.status === 'ready' && access.canViewMembers)

  const [selected, setSelected] = useState<ReportSelection | null>(null)
  const [periodType, setPeriodType] = useState<ReportingPeriodType>('daily')
  // 日报默认昨天：定时报 09:00 生成的是前一日的汇报，锚点落昨天直接命中缓存。
  const [anchorDate, setAnchorDate] = useState<string>(yesterdayAnchorDate())
  const [exportedDocument, setExportedDocument] = useState<BoundExport | null>(null)
  // R1: every async result (run, export, generation facts) is bound to the
  // report context it was requested for — workspace + target + period + date.
  // A stale response that arrives after a switch never attaches to the new
  // report, and switching back to a context restores its in-flight run.
  const [activeRun, setActiveRun] = useState<ActiveRun | null>(null)
  const [generatedFacts, setGeneratedFacts] = useState<GeneratedFacts | null>(null)

  const selectedId = selected?.id ?? 0
  const contextKey = selected
    ? `${selected.kind}:${selected.id}:${periodType}:${anchorDate}`
    : ''
  const contextKeyRef = useRef(contextKey)
  contextKeyRef.current = contextKey

  const preview = useMemberReportPreviewQuery(
    environment, selected?.kind === 'member' ? selectedId : 0,
    { type: periodType, anchorDate },
    selected?.kind === 'member',
  )
  const groupPreview = useGroupReportPreviewQuery(
    environment, selected?.kind === 'group' ? selectedId : 0,
    { type: periodType, anchorDate },
    selected?.kind === 'group' && access.canViewGroup,
  )
  const departmentPreview = useDepartmentReportPreviewQuery(
    environment, selected?.kind === 'department' ? selectedId : 0,
    { type: periodType, anchorDate },
    selected?.kind === 'department' && access.canViewGroup,
  )
  const previewState = selected?.kind === 'group'
    ? groupPreview
    : selected?.kind === 'department' ? departmentPreview : preview

  const run = useReportRunQuery(
    environment, activeRun?.runId ?? 0, activeRun?.startedAt ?? 0, activeRun !== null,
  )
  const runBound = activeRun !== null && activeRun.ctx === contextKey
  const handleGenerated = useCallback((result: StartReportResult, input: ReportGenerateInput) => {
    const ctx = `${input.kind}:${input.targetId}:${input.type}:${input.anchorDate}`
    if (ctx !== contextKeyRef.current) return
    setActiveRun({ runId: result.runId, ctx, startedAt: Date.now() })
    setGeneratedFacts({ ctx, report: result.report })
    setExportedDocument(null)
  }, [])
  const generate = useReportGenerateMutation(
    environment, access.canGenerateMember, access.canGenerateGroup, handleGenerated,
  )
  const handleExported = useCallback((document: ExportedDocumentLike, input: FeishuExportInput) => {
    if (contextKeyRef.current !== input.ctx) return
    // Facts exports (runId=0) bind to the context only; run exports must
    // also match the run that was active when the upload started.
    if (input.runId > 0 && activeRun?.runId !== input.runId) return
    setExportedDocument({ ctx: input.ctx, runId: input.runId, document })
  }, [activeRun])
  const feishuExport = useFeishuExportMutation(environment, access.canPublish, handleExported)
  const handleExportFeishu = useCallback(() => {
    if (!selected) return
    const runState = runBound ? run : null
    const completed = runState?.status === 'ready' ? runState.data : null
    const runId = completed && completed.status === 'completed' && completed.summary
      ? completed.id
      : 0
    feishuExport.exportToFeishu({
      runId,
      ctx: contextKeyRef.current,
      facts: {
        targetKind: selected.kind,
        targetId: selected.id,
        type: periodType,
        anchorDate,
      },
    })
  }, [anchorDate, feishuExport, periodType, run, runBound, selected])

  // Default the selection to the first group, then its first member.
  useEffect(() => {
    if (directory.status !== 'ready') return
    const directoryData = directory.data
    if (selected && selectionVisible(directoryData, selected)) return
    const firstGroup = directoryData.groups[0]
    if (firstGroup) {
      setSelected(firstGroup.members[0]
        ? { kind: 'member', id: firstGroup.members[0].id }
        : { kind: 'group', id: firstGroup.id })
      return
    }
    const firstUngrouped = directoryData.ungrouped[0]
    setSelected(firstUngrouped ? { kind: 'member', id: firstUngrouped.id } : null)
  }, [directory, selected])

  useEffect(() => {
    if ((preview.status === 'error' || groupPreview.status === 'error') && onRuntimeFailure) {
      onRuntimeFailure(new Error('reporting preview query failed'))
    }
  }, [groupPreview.status, onRuntimeFailure, preview.status])

  // Facts shown follow the current context; a just-finished generation
  // replaces the cached preview with the backend's generation-time snapshot.
  const facts = useMemo(() => {
    if (previewState.status !== 'ready') return null
    if (generatedFacts && generatedFacts.ctx === contextKey) return generatedFacts.report
    return previewState.data.report
  }, [contextKey, generatedFacts, previewState])

  // 定时生成的汇报没有前端回执：预览响应随行该视图最近一次 run。完成态
  // 直接带出评语与正文，进行中/排队则接管轮询；同 context 已有更绑定的
  // run（例如刚点的手动生成）时不覆盖。
  const previewReady = previewState.status === 'ready' ? previewState.data : null
  useEffect(() => {
    if (!previewReady || !selected) return
    const latest = previewReady.latestRun
    if (!latest) return
    const ctx = `${selected.kind}:${selected.id}:${periodType}:${anchorDate}`
    if (activeRun !== null && activeRun.ctx === ctx) return
    setActiveRun({ runId: latest.id, ctx, startedAt: Date.now() })
  }, [activeRun, anchorDate, periodType, previewReady, selected])

  const selectedName = useMemo(() => {
    if (!selected || directory.status !== 'ready') return null
    const directoryData = directory.data
    if (selected.kind === 'group') {
      return directoryData.groups.find((group) => group.id === selected.id)?.name
        ?? REPORTING_COPY.directory.ungrouped
    }
    if (selected.kind === 'department') {
      return (directoryData.departments ?? []).find((department) => department.id === selected.id)?.name ?? null
    }
    return directoryData.groups
      .flatMap((group) => group.members)
      .concat(directoryData.ungrouped)
      .find((member) => member.id === selected.id)?.displayName ?? null
  }, [directory, selected])

  const periodControls = useMemo(() => (
    <div className="flex flex-wrap items-center gap-2">
      <ViewTabs
        value={periodType}
        label={REPORTING_COPY.period.label}
        density="compact"
        tabs={REPORTING_PERIOD_TYPES.map((type) => ({
          id: type,
          label: <span data-testid={`reporting-period-${type}`}>{REPORTING_COPY.period[type]}</span>,
        }))}
        onValueChange={(next) => {
          const type = REPORTING_PERIOD_TYPES.find((type) => type === next)
          if (type) setPeriodType(type)
        }}
      />
      <label className="flex items-center gap-1.5 text-xs text-muted-foreground">
        {REPORTING_COPY.period.dateLabel}
        <DatePicker
          value={anchorDate}
          onChange={(value) => {
            if (isValidAnchorDate(value)) setAnchorDate(value)
          }}
          className="h-8 w-auto px-2 text-xs"
        />
      </label>
      {directory.status === 'error' && (
        <Button type="button" variant="outline" size="sm" onClick={directory.retry}>
          {REPORTING_COPY.page.retry}
        </Button>
      )}
    </div>
  ), [anchorDate, directory, periodType])

  if (!environment.allowed || access.status !== 'ready' || !access.canViewMembers) {
    return (
      <OperationalPage title={REPORTING_COPY.page.title}>
        <EmptyState role="alert" title={REPORTING_COPY.page.unavailableTitle} />
      </OperationalPage>
    )
  }

  return (
    <OperationalPage
      title={REPORTING_COPY.page.title}
      actions={periodControls}
      bodyOverflow="hidden"
    >
      {directory.status === 'ready' && directory.data.total === 0 ? (
        <EmptyState title={REPORTING_COPY.directory.emptyTitle} />
      ) : (
        <div className="grid h-full min-h-0 grid-cols-1 gap-0 lg:grid-cols-[280px_minmax(0,1fr)]">
          <aside className="min-h-0 overflow-y-auto border-b border-border lg:border-b-0 lg:border-r">
            <MemberDirectory
              state={directory}
              selected={selected}
              onSelectMember={(member: ReportingMember) => {
                setSelected({ kind: 'member', id: member.id })
                // Switching context detaches the bound run/exports by key; the
                // in-flight run keeps executing and reattaches if the user
                // returns to the same context.
                setExportedDocument(null)
              }}
              onSelectGroup={(groupId: number) => {
                setSelected({ kind: 'group', id: groupId })
                setExportedDocument(null)
              }}
              onSelectDepartment={(departmentId: number) => {
                setSelected({ kind: 'department', id: departmentId })
                setExportedDocument(null)
              }}
            />
          </aside>
          <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
            <MemberReportPanel
              selected={selected && selectedName ? { ...selected, name: selectedName } : null}
              periodType={periodType}
              previewState={previewState}
              facts={facts}
              runState={runBound ? run : null}
              generating={generate.isPending}
              canGenerate={selected?.kind === 'member' ? access.canGenerateMember : access.canGenerateGroup}
              canPublish={access.canPublish}
              onGenerate={() => {
                if (selected) {
                  generate.generate({ kind: selected.kind, targetId: selected.id, type: periodType, anchorDate })
                }
              }}
              onExportFeishu={handleExportFeishu}
              feishuUploading={feishuExport.isPending}
              exportedDocument={
                exportedDocument && exportedDocument.ctx === contextKey
                  && (exportedDocument.runId === 0
                    || (activeRun?.runId === exportedDocument.runId && runBound))
                  ? exportedDocument.document
                  : null
              }
            />
          </div>
        </div>
      )}
    </OperationalPage>
  )
}

type ExportedDocumentLike = {
  documentId: string
  url: string
  title: string
}

type ActiveRun = {
  runId: number
  ctx: string
  startedAt: number
}

type GeneratedFacts = {
  ctx: string
  report: MemberReport
}

type BoundExport = {
  ctx: string
  runId: number
  document: ExportedDocumentLike
}

function selectionVisible(
  directory: ReportingDirectory,
  selected: ReportSelection,
): boolean {
  if (selected.kind === 'group') {
    return directory.groups.some((group) => group.id === selected.id)
  }
  if (selected.kind === 'department') {
    return (directory.departments ?? []).some((department) => department.id === selected.id)
  }
  return directory.groups.some((group) => group.members.some((member) => member.id === selected.id))
    || directory.ungrouped.some((member) => member.id === selected.id)
}
