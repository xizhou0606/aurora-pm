import { useMemo, useState } from 'react'

import { REPORTING_COPY } from '@/shared/copy'
import { Button, EmptyState, LoadingSkeleton, StatusPill } from '@/shared/ui'

import type { ReportingDepartment, ReportingDirectory, ReportingMember } from '../model/types'
import type { ReportingListState } from '../hooks'

export type ReportSelection =
  | { kind: 'member'; id: number }
  | { kind: 'group'; id: number }
  | { kind: 'department'; id: number }

type MemberDirectoryProps = {
  state: ReportingListState<ReportingDirectory>
  selected: ReportSelection | null
  onSelectMember: (member: ReportingMember) => void
  onSelectGroup: (groupId: number) => void
  onSelectDepartment: (departmentId: number) => void
}

function MemberAvatar({ member }: { member: ReportingMember }) {
  if (member.avatarUrl) {
    return (
      <img
        src={member.avatarUrl}
        alt=""
        aria-hidden="true"
        className="size-7 shrink-0 rounded-full object-cover"
      />
    )
  }
  return (
    <span
      aria-hidden="true"
      className="flex size-7 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-medium text-muted-foreground"
    >
      {member.displayName.slice(0, 1)}
    </span>
  )
}

function MemberRow({
  member,
  selected,
  onSelect,
}: {
  member: ReportingMember
  selected: boolean
  onSelect: (member: ReportingMember) => void
}) {
  return (
    <button
      type="button"
      onClick={() => onSelect(member)}
      aria-pressed={selected}
      data-testid="reporting-member-row"
      className={`flex w-full items-center gap-3 rounded-lg py-1.5 pl-3 pr-2 text-left transition-control outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background ${
        selected
          ? 'bg-accent text-accent-foreground'
          : 'hover:bg-muted focus-visible:bg-muted'
      }`}
    >
      <MemberAvatar member={member} />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-medium text-foreground">
          {member.displayName}
        </span>
        <span className="block truncate text-xs text-muted-foreground">
          {member.roleNames.length > 0
            ? member.roleNames.join('、')
            : member.jobTitle
              ? (
                <>
                  <span>{member.jobTitle}</span>
                  <StatusPill tone="neutral" className="ml-1 min-w-0 px-1 py-0 align-middle">
                    {REPORTING_COPY.directory.jobTitleBadge}
                  </StatusPill>
                </>
              )
              : REPORTING_COPY.directory.noRoles}
        </span>
      </span>
    </button>
  )
}

function DirectoryToggle({ name, expanded, onToggle, testId }: {
  name: string
  expanded: boolean
  onToggle: () => void
  testId: string
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon-xs"
      className="rounded-lg text-muted-foreground"
      onClick={onToggle}
      aria-expanded={expanded}
      aria-label={expanded
        ? REPORTING_COPY.directory.collapseGroup(name)
        : REPORTING_COPY.directory.expandGroup(name)}
      data-testid={testId}
    >
      <svg
        aria-hidden="true"
        viewBox="0 0 16 16"
        className={`size-3.5 transition-transform duration-150 ${expanded ? 'rotate-90' : ''}`}
      >
        <path d="M6 3.5 L11 8 L6 12.5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    </Button>
  )
}

function GroupSection({
  groupId,
  name,
  members,
  selected,
  onSelectMember,
  onSelectGroup,
}: {
  groupId: number
  name: string
  members: readonly ReportingMember[]
  selected: ReportSelection | null
  onSelectMember: (member: ReportingMember) => void
  onSelectGroup: (groupId: number) => void
}) {
  // The tree keeps every role group visible — including empty ones — so the
  // organisational hierarchy never silently disappears.
  const groupSelected = selected?.kind === 'group' && selected.id === groupId
  const [expanded, setExpanded] = useState(true)
  const open = expanded
  return (
    <section className="pb-1">
      <div
        className={`flex items-center gap-1 rounded-lg pr-1 transition-colors ${
          groupSelected ? 'bg-accent text-accent-foreground' : 'hover:bg-muted'
        }`}
      >
        <DirectoryToggle
          name={name}
          expanded={open}
          onToggle={() => setExpanded((current) => !current)}
          testId={`reporting-group-toggle-${groupId}`}
        />
        <button
          type="button"
          onClick={groupId > 0
            ? () => {
              setExpanded(true)
              onSelectGroup(groupId)
            }
            : undefined}
          aria-pressed={groupSelected}
          disabled={groupId === 0}
          data-testid={`reporting-group-row-${groupId}`}
          className="flex min-w-0 flex-1 items-center justify-between gap-2 rounded-lg py-2 text-left transition-control outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background disabled:cursor-default"
        >
          <span className="min-w-0 truncate text-sm font-semibold text-foreground">{name}</span>
          <span className="flex shrink-0 items-center gap-1.5">
            <StatusPill tone="neutral">{REPORTING_COPY.directory.memberCount(members.length)}</StatusPill>
          </span>
        </button>
      </div>
      {open && (
        <div className="ml-3 border-l border-border pl-2">
          {members.length === 0 ? (
            <div className="px-2 py-1">
              <EmptyState title={REPORTING_COPY.directory.emptyGroup} name="hugeicons:user-multiple" />
            </div>
          ) : (
            members.map((member) => (
              <MemberRow
                key={member.id}
                member={member}
                selected={selected?.kind === 'member' && selected.id === member.id}
                onSelect={onSelectMember}
              />
            ))
          )}
        </div>
      )}
    </section>
  )
}

export function MemberDirectory({
  state,
  selected,
  onSelectMember,
  onSelectGroup,
  onSelectDepartment,
}: MemberDirectoryProps) {
  if (state.status === 'loading' || state.status === 'unavailable') {
    return (
      <div className="space-y-2 p-3" role="status" aria-busy="true">
        <LoadingSkeleton rows={6} />
      </div>
    )
  }
  if (state.status === 'error') {
    return null
  }
  const directory = state.data
  const departments = directory.departments ?? []
  return (
    <nav aria-label={REPORTING_COPY.directory.title} className="pb-4">
      {departments.length > 0 ? (
        <DepartmentTree
          departments={departments}
          directory={directory}
          selected={selected}
          onSelectMember={onSelectMember}
          onSelectDepartment={onSelectDepartment}
        />
      ) : (
        <>
          {directory.groups.map((group) => (
            <GroupSection
              key={group.id}
              groupId={group.id}
              name={group.name}
              members={group.members}
              selected={selected}
              onSelectMember={onSelectMember}
              onSelectGroup={onSelectGroup}
            />
          ))}
          <GroupSection
            groupId={0}
            name={REPORTING_COPY.directory.ungrouped}
            members={directory.ungrouped}
            selected={selected}
            onSelectMember={onSelectMember}
            onSelectGroup={onSelectGroup}
          />
        </>
      )}
    </nav>
  )
}

type DepartmentTreeProps = {
  departments: readonly ReportingDepartment[]
  directory: ReportingDirectory
  selected: ReportSelection | null
  onSelectMember: (member: ReportingMember) => void
  onSelectDepartment: (departmentId: number) => void
}

// DepartmentTree renders the synced Feishu org structure. Department headers
// are display-only — 组报 targets stay 用户组-based — so only member rows
// select. Members without a mapped department surface in the explicit
// 未分组 bucket so the list stays exhaustive.
function DepartmentTree({ departments, directory, selected, onSelectMember, onSelectDepartment }: DepartmentTreeProps) {
  const childrenByParent = useMemo(() => {
    const map = new Map<string, ReportingDepartment[]>()
    for (const department of departments) {
      const list = map.get(department.parentId) ?? []
      list.push(department)
      map.set(department.parentId, list)
    }
    for (const list of map.values()) {
      list.sort((left, right) =>
        left.order !== right.order ? left.order - right.order : left.name.localeCompare(right.name, 'zh-Hans-CN'))
    }
    return map
  }, [departments])
  const membersByDepartment = useMemo(() => {
    const map = new Map<string, ReportingMember[]>()
    for (const group of directory.groups) {
      for (const member of group.members) {
        const key = member.departmentId ?? ''
        const list = map.get(key) ?? []
        list.push(member)
        map.set(key, list)
      }
    }
    for (const member of directory.ungrouped) {
      const key = member.departmentId ?? ''
      const list = map.get(key) ?? []
      list.push(member)
      map.set(key, list)
    }
    return map
  }, [directory])
  const roots = childrenByParent.get('0') ?? departments.filter(
    (department) => !departments.some((parent) => parent.openId === department.parentId),
  )
  // 部门徽标显示递归总人数（直属成员 + 全部子部门成员），父节点不再显示 0。
  const memberTotals = useMemo(() => {
    const totals = new Map<string, number>()
    const compute = (department: ReportingDepartment): number => {
      let total = (membersByDepartment.get(department.openId) ?? []).length
      for (const child of childrenByParent.get(department.openId) ?? []) {
        total += compute(child)
      }
      totals.set(department.openId, total)
      return total
    }
    for (const department of departments) compute(department)
    return totals
  }, [childrenByParent, departments, membersByDepartment])
  // 部门视图不渲染“未分组”：组织架构里找不到的人（离职未停用、重名、
  // 姓名不一致）不该出现在可选汇报对象里；用户组兜底视图仍保留未分组。
  return (
    <>
      {roots.map((department) => (
        <DepartmentSection
          key={department.id}
          department={department}
          childrenByParent={childrenByParent}
          membersByDepartment={membersByDepartment}
          memberTotals={memberTotals}
          selected={selected}
          onSelectMember={onSelectMember}
          onSelectDepartment={onSelectDepartment}
        />
      ))}
    </>
  )
}

type DepartmentSectionProps = {
  department: ReportingDepartment
  childrenByParent: Map<string, ReportingDepartment[]>
  membersByDepartment: Map<string, ReportingMember[]>
  memberTotals: Map<string, number>
  selected: ReportSelection | null
  onSelectMember: (member: ReportingMember) => void
  onSelectDepartment: (departmentId: number) => void
}

function DepartmentSection({
  department,
  childrenByParent,
  membersByDepartment,
  memberTotals,
  selected,
  onSelectMember,
  onSelectDepartment,
}: DepartmentSectionProps) {
  // The tree keeps every department visible — including empty ones — so the
  // organisational hierarchy never silently disappears.
  const [expanded, setExpanded] = useState(true)
  const children = childrenByParent.get(department.openId) ?? []
  const members = membersByDepartment.get(department.openId) ?? []
  const isEmpty = children.length === 0 && members.length === 0
  return (
    <section className="pb-1">
      <div className="flex items-center gap-1 rounded-lg pr-1 transition-colors hover:bg-muted">
        <DirectoryToggle
          name={department.name}
          expanded={expanded}
          onToggle={() => setExpanded((current) => !current)}
          testId={`reporting-dept-toggle-${department.openId}`}
        />
        <button
          type="button"
          onClick={() => {
            setExpanded(true)
            onSelectDepartment(department.id)
          }}
          aria-pressed={selected?.kind === 'department' && selected.id === department.id}
          data-testid={`reporting-dept-row-${department.openId}`}
          className="flex min-w-0 flex-1 items-center justify-between gap-2 rounded-lg py-2 text-left transition-control outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background aria-pressed:bg-accent aria-pressed:text-accent-foreground"
        >
          <span className="min-w-0 truncate text-sm font-semibold text-foreground">
            {department.name}
          </span>
          <span className="flex shrink-0 items-center gap-1.5">
            <StatusPill tone="neutral">
              {REPORTING_COPY.directory.memberCount(memberTotals.get(department.openId) ?? members.length)}
            </StatusPill>
          </span>
        </button>
      </div>
      {expanded && (
        <div className="ml-3 border-l border-border pl-2">
          {children.map((child) => (
            <DepartmentSection
              key={child.id}
              department={child}
              childrenByParent={childrenByParent}
              membersByDepartment={membersByDepartment}
              memberTotals={memberTotals}
              selected={selected}
              onSelectMember={onSelectMember}
              onSelectDepartment={onSelectDepartment}
            />
          ))}
          {members.map((member) => (
            <MemberRow
              key={member.id}
              member={member}
              selected={selected?.kind === 'member' && selected.id === member.id}
              onSelect={onSelectMember}
            />
          ))}
          {isEmpty && (
            <div className="px-2 py-1">
              <EmptyState title={REPORTING_COPY.directory.emptyGroup} name="hugeicons:user-multiple" />
            </div>
          )}
        </div>
      )}
    </section>
  )
}
