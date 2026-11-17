export type ReportingPeriodType = 'daily' | 'weekly' | 'monthly'

export const REPORTING_PERIOD_TYPES: readonly ReportingPeriodType[] = [
  'daily',
  'weekly',
  'monthly',
]

export type ReportingMember = {
  id: number
  username: string
  displayName: string
  avatarUrl?: string
  groupName: string
  roleNames: readonly string[]
  // 同步自飞书的职务：roleNames 为空时的兜底展示来源，需加「职务」标注。
  jobTitle?: string
  departmentId?: string
  departmentName?: string
}

export type ReportingGroup = {
  id: number
  code: string
  name: string
  members: readonly ReportingMember[]
}

// 同步自飞书通讯录的组织树节点（展示数据；id 为 PM 内部数值 id）。
export type ReportingDepartment = {
  id: number
  openId: string
  parentId: string
  name: string
  order: number
  memberCount: number
}

export type ReportingDirectory = {
  groups: readonly ReportingGroup[]
  ungrouped: readonly ReportingMember[]
  total: number
  // 飞书组织树；首次同步成功前为空，目录回退到用户组分组。
  departments?: readonly ReportingDepartment[]
}

export type ReportingPeriod = {
  type: ReportingPeriodType
  start: string
  end: string
  anchorDate: string
}

export type ReportTargetKind = 'member' | 'group' | 'department'

export type ReportTarget = {
  kind: ReportTargetKind
  id: number
  name: string
  subLabel?: string
  roleNames?: readonly string[]
  // 成员目标的飞书原始职务：roleNames 为空时的「职务」兜底展示来源；
  // 组/部门目标不携带。
  jobTitle?: string
  avatarUrl?: string
}

export type ReportingTaskItem = {
  id: number
  key: string
  title: string
  status: string
  completedAt?: string
  dueAt?: string
}

export type ReportingBugItem = {
  id: number
  key: string
  title: string
  status: string
  severity: string
  resolvedAt?: string
  dueAt?: string
}

export type ReportingTaskMetrics = {
  completed: number
  completedItems: readonly ReportingTaskItem[]
  completedOnTime: number
  created: number
  active: number
  activeItems: readonly ReportingTaskItem[]
  overdue: number
  overdueItems: readonly ReportingTaskItem[]
}

export type ReportingBugMetrics = {
  resolved: number
  resolvedItems: readonly ReportingBugItem[]
  resolvedOnTime: number
  closed: number
  open: number
  openItems: readonly ReportingBugItem[]
  overdue: number
}

export type ReportingMemberRollup = {
  member: ReportingMember
  tasks: ReportingTaskMetrics
  bugs: ReportingBugMetrics
  // 组/部门排行用的成员级代码活动；采集未接入或无归属时缺省。
  code?: CodeActivity
  // 测试团队成员的「提出缺陷」分档（与 code 互斥，后端保证）。
  bugReports?: BugReportMetrics
  // 产品团队成员的「需求数量」事实（与 code 互斥，后端保证）。
  requirements?: RequirementMetrics
  // 设计团队成员的「UI 缺陷」处理事实（与 code 互斥，后端保证）。
  uiBugs?: UIBugMetrics
}

// 测试岗位的核心事实：本期提出的缺陷，按严重级别分档（P3 不单列）。
export type BugReportMetrics = {
  total: number
  p0: number
  p1: number
  p2: number
}

// 设计岗位的核心事实：UI 域缺陷处理情况（解决按周期，存量/逾期为快照）。
export type UIBugMetrics = {
  resolved: number
  open: number
  overdue: number
}

// 产品岗位的核心事实：负责的需求总数、本期新增与状态分布（快照口径）。
export type RequirementMetrics = {
  total: number
  createdInPeriod: number
  done: number
  acceptance: number
  development: number
  pending: number
}

export type ReportScoreDimension = {
  key: string
  label: string
  score: number
  basis: string
}

export type ReportScores = {
  overall: number
  dimensions: readonly ReportScoreDimension[]
}

export type CodeFreshness = {
  lastSuccessAt?: string
  degraded: boolean
  error?: string
}

// 代码活动是展示事实：LOC 不进评分（net 变化由展示层计算）。
export type CodeActivity = {
  repos: number
  commits: number
  additions: number
  deletions: number
  prsMerged: number
  reviewsCompleted: number
  // 行数构成（四类合计 = additions）与"带评审合入"数；历史聚合行为 0，
  // 此时只展示总数。评审覆盖 = prsReviewed / prsMerged。
  sourceAdditions: number
  testAdditions: number
  docAdditions: number
  configAdditions: number
  prsReviewed: number
  // 逐提交质量证据：报告把每次提交(含截断 diff 样本)交给 AI 点评；
  // 调试残留/TODO 命中同时构成代码卫生扣分的依据。
  debugHits: number
  todoHits: number
  commitReviews: readonly CommitReview[]
  freshness: CodeFreshness
}

export type CommitReview = {
  sha: string
  day: string
  subject: string
  additions: number
  deletions: number
  files: number
  debugHits: number
  todoHits: number
  diffSample?: string
}

export type MemberReport = {
  target: ReportTarget
  period: ReportingPeriod
  generatedAt: string
  tasks: ReportingTaskMetrics
  bugs: ReportingBugMetrics
  code?: CodeActivity
  // 岗位焦点事实：出现其一即隐藏代码活动（后端保证互斥）。
  bugReports?: BugReportMetrics
  requirements?: RequirementMetrics
  uiBugs?: UIBugMetrics
  scores: ReportScores
  members?: readonly ReportingMemberRollup[]
}

export type StartReportResult = {
  runId: number
  status: string
  report: MemberReport
}

// 预览响应：工作事实 + 该视图最近一次 run（定时生成没有前端回执，页面靠
// 它把评语与正文自动带出来；该视图尚未生成时缺省）。
export type ReportPreview = {
  report: MemberReport
  latestRun?: ReportingRun
}

export type ExportedDocument = {
  documentId: string
  url: string
  title: string
}

export type CommitQualityVerdict = {
  sha: string
  verdict: string
  summary: string
  issues?: readonly string[]
}

export type CommitQualityAssessment = {
  score: number
  conclusion: string
  commonIssues?: readonly string[]
  risks?: readonly string[]
  suggestions?: readonly string[]
  commits?: readonly CommitQualityVerdict[]
}

// 三个成员排行维度各自的 AI 综合短评（团队汇报非空，个人汇报为空）。
export type ReportDimensionComments = {
  tasks: string
  quality: string
  code: string
}

export type ReportingRun = {
  id: number
  status: string
  capability: string
  subjectType: string
  subjectId: number
  summary: string
  errorMessage: string
  completedAt?: string
  commitQuality?: CommitQualityAssessment
  overallComment?: string
  // AI 综合评分（0-100）：随综合评语一起输出，旧运行/生成失败时缺省。
  overallScore?: number
  dimensionComments?: ReportDimensionComments
}
