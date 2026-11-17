import type {
  ExportedDocument,
  MemberReport,
  ReportingDepartment,
  ReportDimensionComments,
  ReportPreview,
  ReportScoreDimension,
  ReportScores,
  ReportingMemberRollup,
  ReportTarget,
  ReportingBugItem,
  ReportingBugMetrics,
  ReportingDirectory,
  ReportingGroup,
  ReportingMember,
  ReportingPeriod,
  ReportingPeriodType,
  ReportingRun,
  ReportingTaskItem,
  ReportingTaskMetrics,
  StartReportResult,
  BugReportMetrics,
  UIBugMetrics,
  CodeActivity,
  CommitReview,
  CommitQualityAssessment,
  RequirementMetrics,
} from '../model/types'
import { REPORTING_COPY } from '@/shared/copy'
import type { ReportingRaw } from './contracts'

function record(value: ReportingRaw): Record<string, unknown> {
  return value !== null && typeof value === 'object' ? (value as Record<string, unknown>) : {}
}

function string(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : fallback
}

function number(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function optionalTime(value: unknown): string | undefined {
  return typeof value === 'string' && value ? value : undefined
}

function stringArray(value: unknown): string[] {
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string')
    : []
}

const PERIOD_TYPES: readonly ReportingPeriodType[] = ['daily', 'weekly', 'monthly']

function periodType(value: unknown): ReportingPeriodType {
  return PERIOD_TYPES.includes(value as ReportingPeriodType)
    ? (value as ReportingPeriodType)
    : 'daily'
}

// Unknown enum values stay visible instead of silently mapping to a known
// status; the run status switch renders them through the generic pending view.
export const REPORT_RUN_TERMINAL_STATUSES = ['completed', 'failed'] as const

export function normalizeMember(value: ReportingRaw): ReportingMember {
  const member = record(value)
  return {
    id: number(member.id),
    username: string(member.username),
    displayName: string(member.display_name) || string(member.username),
    avatarUrl: optionalTime(member.avatar_url),
    groupName: string(member.group_name),
    roleNames: stringArray(member.role_names),
    jobTitle: string(member.job_title) || undefined,
    departmentId: string(member.department_id) || undefined,
    departmentName: string(member.department_name) || undefined,
  }
}

function normalizeDepartments(value: unknown): ReportingDepartment[] {
  return Array.isArray(value) ? value.map((entry): ReportingDepartment => {
    const department = record(entry)
    return {
      id: number(department.id),
      openId: string(department.open_department_id),
      parentId: string(department.parent_department_id) || '0',
      name: string(department.name),
      order: number(department.order),
      memberCount: number(department.member_count),
    }
  }).filter((department) => department.id > 0 && department.openId !== '') : []
}

export function normalizeDirectory(raw: ReportingRaw): ReportingDirectory {
  const body = record(raw)
  const groups = Array.isArray(body.groups) ? body.groups.map((group): ReportingGroup => {
    const entry = record(group)
    return {
      id: number(entry.id),
      code: string(entry.code),
      name: string(entry.name) || REPORTING_COPY.directory.ungrouped,
      members: Array.isArray(entry.members) ? entry.members.map(normalizeMember) : [],
    }
  }) : []
  const ungrouped = Array.isArray(body.ungrouped) ? body.ungrouped.map(normalizeMember) : []
  const departments = normalizeDepartments(body.departments)
  return {
    groups,
    ungrouped,
    total: number(body.total) || groups.reduce((sum, group) => sum + group.members.length, 0)
      + ungrouped.length,
    departments: departments.length > 0 ? departments : undefined,
  }
}

export function normalizePeriod(value: ReportingRaw): ReportingPeriod {
  const period = record(value)
  return {
    type: periodType(period.type),
    start: string(period.start),
    end: string(period.end),
    anchorDate: string(period.anchor_date),
  }
}

function normalizeTaskItems(value: unknown): ReportingTaskItem[] {
  return Array.isArray(value) ? value.map((item): ReportingTaskItem => {
    const entry = record(item)
    return {
      id: number(entry.id),
      key: string(entry.key),
      title: string(entry.title),
      status: string(entry.status),
      completedAt: optionalTime(entry.completed_at),
      dueAt: optionalTime(entry.due_at),
    }
  }) : []
}

function normalizeBugItems(value: unknown): ReportingBugItem[] {
  return Array.isArray(value) ? value.map((item): ReportingBugItem => {
    const entry = record(item)
    return {
      id: number(entry.id),
      key: string(entry.key),
      title: string(entry.title),
      status: string(entry.status),
      severity: string(entry.severity),
      resolvedAt: optionalTime(entry.resolved_at),
      dueAt: optionalTime(entry.due_at),
    }
  }) : []
}

function normalizeCommitReviews(value: unknown): CommitReview[] {
  if (!Array.isArray(value)) return []
  return value.flatMap((entry) => {
    if (!entry || typeof entry !== 'object') return []
    const record = entry as Record<string, unknown>
    const sha = typeof record.sha === 'string' ? record.sha : ''
    const subject = typeof record.subject === 'string' ? record.subject : ''
    if (!sha || !subject) return []
    const diffSample = typeof record.diff_sample === 'string' ? record.diff_sample : ''
    return [{
      sha,
      day: typeof record.day === 'string' ? record.day : '',
      subject,
      additions: number(record.additions ?? 0),
      deletions: number(record.deletions ?? 0),
      files: number(record.files ?? 0),
      debugHits: number(record.debug_hits ?? 0),
      todoHits: number(record.todo_hits ?? 0),
      ...(diffSample ? { diffSample } : {}),
    }]
  })
}

function normalizeCode(value: unknown): CodeActivity | undefined {
  if (value === null || typeof value !== 'object') {
    return undefined
  }
  const entry = record(value)
  const freshness = record(entry.freshness)
  return {
    repos: number(entry.repos ?? 0),
    commits: number(entry.commits ?? 0),
    additions: number(entry.additions ?? 0),
    deletions: number(entry.deletions ?? 0),
    prsMerged: number(entry.prs_merged ?? 0),
    reviewsCompleted: number(entry.reviews_completed ?? 0),
    sourceAdditions: number(entry.source_additions ?? 0),
    testAdditions: number(entry.test_additions ?? 0),
    docAdditions: number(entry.doc_additions ?? 0),
    configAdditions: number(entry.config_additions ?? 0),
    prsReviewed: number(entry.prs_reviewed ?? 0),
    debugHits: number(entry.debug_hits ?? 0),
    todoHits: number(entry.todo_hits ?? 0),
    commitReviews: normalizeCommitReviews(entry.commit_reviews),
    freshness: {
      lastSuccessAt: entry.last_success_at ? string(entry.last_success_at) : undefined,
      degraded: entry.degraded === true,
      error: entry.error ? string(entry.error) : undefined,
    },
  }
}

function normalizeBugReports(value: unknown): BugReportMetrics | undefined {
  if (value === null || typeof value !== 'object') return undefined
  const entry = record(value)
  return {
    total: number(entry.total),
    p0: number(entry.p0),
    p1: number(entry.p1),
    p2: number(entry.p2),
  }
}

function normalizeUIBugs(value: unknown): UIBugMetrics | undefined {
  if (value === null || typeof value !== 'object') return undefined
  const entry = record(value)
  return {
    resolved: number(entry.resolved),
    open: number(entry.open),
    overdue: number(entry.overdue),
  }
}

function normalizeRequirements(value: unknown): RequirementMetrics | undefined {
  if (value === null || typeof value !== 'object') return undefined
  const entry = record(value)
  return {
    total: number(entry.total),
    createdInPeriod: number(entry.created_in_period),
    done: number(entry.done),
    acceptance: number(entry.acceptance),
    development: number(entry.development),
    pending: number(entry.pending),
  }
}

export function normalizeReport(raw: ReportingRaw): MemberReport {
  const body = record(raw)
  const tasks = record(body.tasks)
  const bugs = record(body.bugs)
  const code = body.code === undefined || body.code === null ? undefined : normalizeCode(body.code)
  return {
    members: Array.isArray(body.members)
      ? body.members.map((rollup) => {
        const entry = record(rollup)
        return {
          member: normalizeMember(entry.member),
          tasks: normalizeTasks(entry.tasks),
          bugs: normalizeBugs(entry.bugs),
          code: entry.code === undefined || entry.code === null ? undefined : normalizeCode(entry.code),
          bugReports: normalizeBugReports(entry.bug_reports),
          requirements: normalizeRequirements(entry.requirements),
          uiBugs: normalizeUIBugs(entry.ui_bugs),
        }
      })
      : undefined,
    target: normalizeTarget(body.target),
    period: normalizePeriod(body.period),
    generatedAt: string(body.generated_at),
    tasks: normalizeTasks(tasks),
    bugs: normalizeBugs(bugs),
    code,
    bugReports: normalizeBugReports(body.bug_reports),
    requirements: normalizeRequirements(body.requirements),
    uiBugs: normalizeUIBugs(body.ui_bugs),
    scores: normalizeScores(body.scores),
  }
}

function normalizeTasks(value: unknown): ReportingTaskMetrics {
  const tasks = record(value)
  return {
    completed: number(tasks.completed),
    completedItems: normalizeTaskItems(tasks.completed_items),
    completedOnTime: number(tasks.completed_on_time),
    created: number(tasks.created),
    active: number(tasks.active),
    activeItems: normalizeTaskItems(tasks.active_items),
    overdue: number(tasks.overdue),
    overdueItems: normalizeTaskItems(tasks.overdue_items),
  }
}

function normalizeBugs(value: unknown): ReportingBugMetrics {
  const bugs = record(value)
  return {
    resolved: number(bugs.resolved),
    resolvedItems: normalizeBugItems(bugs.resolved_items),
    resolvedOnTime: number(bugs.resolved_on_time),
    closed: number(bugs.closed),
    open: number(bugs.open),
    openItems: normalizeBugItems(bugs.open_items),
    overdue: number(bugs.overdue),
  }
}

function normalizeScores(value: unknown): ReportScores {
  const scores = record(value)
  const dimensions = Array.isArray(scores.dimensions)
    ? scores.dimensions.map((dimension): ReportScoreDimension => {
      const entry = record(dimension)
      return {
        key: string(entry.key),
        label: string(entry.label),
        score: Math.max(0, Math.min(100, number(entry.score))),
        basis: string(entry.basis),
      }
    })
    : []
  return { overall: number(scores.overall), dimensions }
}

function normalizeTarget(value: ReportingRaw): ReportTarget {
  const target = record(value)
  const kind = string(target.kind, 'member')
  return {
    kind: kind === 'group' || kind === 'department' ? kind : 'member',
    id: number(target.id),
    name: string(target.name),
    subLabel: string(target.sub_label),
    roleNames: stringArray(target.role_names),
    jobTitle: optionalTime(target.job_title),
    avatarUrl: optionalTime(target.avatar_url),
  }
}

export function normalizeExportedDocument(raw: ReportingRaw): ExportedDocument {
  const body = record(raw)
  return {
    documentId: string(body.document_id),
    url: string(body.url),
    title: string(body.title),
  }
}

export function normalizeStartResult(raw: ReportingRaw): StartReportResult {
  const body = record(raw)
  return {
    runId: number(body.run_id),
    status: string(body.status, 'queued'),
    report: normalizeReport(body.report),
  }
}

export function normalizePreview(raw: ReportingRaw): ReportPreview {
  const body = record(raw)
  return {
    report: normalizeReport(body.report),
    latestRun: body.latest_run === undefined || body.latest_run === null
      ? undefined
      : normalizeRun(body.latest_run),
  }
}

function normalizeCommitQuality(value: unknown): CommitQualityAssessment | undefined {
  if (!value || typeof value !== 'object') return undefined
  const entry = record(value)
  const commits = Array.isArray(entry.commits)
    ? entry.commits.flatMap((raw) => {
        if (!raw || typeof raw !== 'object') return []
        const commit = record(raw)
        const sha = typeof commit.sha === 'string' ? commit.sha : ''
        const verdict = typeof commit.verdict === 'string' ? commit.verdict : ''
        if (!sha || !verdict) return []
        const issues = Array.isArray(commit.issues)
          ? commit.issues.filter((issue): issue is string => typeof issue === 'string')
          : []
        return [{
          sha,
          verdict,
          summary: typeof commit.summary === 'string' ? commit.summary : '',
          issues,
        }]
      })
    : []
  return {
    score: number(entry.score ?? 0),
    conclusion: string(entry.conclusion),
    commonIssues: Array.isArray(entry.common_issues)
      ? entry.common_issues.filter((issue): issue is string => typeof issue === 'string')
      : [],
    risks: Array.isArray(entry.risks)
      ? entry.risks.filter((risk): risk is string => typeof risk === 'string')
      : [],
    suggestions: Array.isArray(entry.suggestions)
      ? entry.suggestions.filter((item): item is string => typeof item === 'string')
      : [],
    commits,
  }
}

function normalizeDimensionComments(value: unknown): ReportDimensionComments | undefined {
  if (!value || typeof value !== 'object') return undefined
  const entry = record(value)
  const text = (key: string): string => string(entry[key])
  const comments: ReportDimensionComments = {
    tasks: text('tasks'),
    quality: text('quality'),
    code: text('code'),
  }
  return comments.tasks || comments.quality || comments.code ? comments : undefined
}

export function normalizeRun(raw: ReportingRaw): ReportingRun {
  const body = record(raw)
  return {
    id: number(body.id),
    status: string(body.status, 'unknown'),
    capability: string(body.capability),
    subjectType: string(body.subject_type, 'member'),
    subjectId: number(body.subject_id),
    summary: string(body.summary),
    errorMessage: string(body.error_message),
    completedAt: optionalTime(body.completed_at),
    commitQuality: normalizeCommitQuality(body.commit_quality),
    overallComment: string(body.overall_comment),
    // 缺省（旧运行/模型未输出）保持 undefined 而不是 0，避免误读为最低分。
    overallScore: typeof body.overall_score === 'number' && Number.isFinite(body.overall_score)
      ? Math.min(100, Math.max(0, Math.round(body.overall_score)))
      : undefined,
    dimensionComments: normalizeDimensionComments(body.dimension_comments),
  }
}
