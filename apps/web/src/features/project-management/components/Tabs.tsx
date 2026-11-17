import { Link } from '@tanstack/react-router'

import { parsePlanUnitSearch, type ProjectDetailTab } from '@/shared/router'
import { useSessionStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import type { PlanUnitTab, ProjectWorkspaceRoute } from '../model'
import type { ProjectOperationAccess } from '../hooks'

type TabDefinition = { label: string }

const projectTabs = {
  overview: { label: copy.projectManagement.tabs.overview },
  items: { label: copy.projectManagement.tabs.planUnits },
  activity: { label: copy.projectManagement.tabs.activity },
} satisfies Record<ProjectDetailTab, TabDefinition>

const projectTabEntries = Object.entries(projectTabs) as [
  ProjectDetailTab,
  TabDefinition,
][]

const planUnitTabs = {
  overview: { label: copy.projectManagement.tabs.overview },
  requirements: { label: copy.projectManagement.tabs.requirements },
  tasks: { label: copy.projectManagement.tabs.tasks },
  delivery: { label: copy.projectManagement.delivery.assignments.heading },
  activity: { label: copy.projectManagement.tabs.activity },
} satisfies Record<PlanUnitTab, TabDefinition>

const planUnitTabEntries = Object.entries(planUnitTabs) as [
  PlanUnitTab,
  TabDefinition,
][]

export function DetailTabs({
  active,
  projectId,
  operationAccess,
}: {
  active: ProjectDetailTab
  projectId: number
  operationAccess?: ProjectOperationAccess
}) {
  const entries = projectTabEntries.filter(([tab]) => {
    if (!operationAccess) return true
    if (tab === 'overview') return operationAccess.viewBoard
    if (tab === 'items') return operationAccess.viewPlanUnits
    return operationAccess.viewActivity
  })
  return (
    <nav
      aria-label={copy.projectManagement.title}
      data-slot="pm-detail-tabs"
      className="flex min-w-0 gap-1 overflow-x-auto rounded-lg bg-muted/60 p-1"
    >
      {entries.map(([tab, definition]) => (
        <Link
          key={tab}
          to="/workspace/projects/$projectId"
          params={{ projectId: String(projectId) }}
          search={{
            tab,
            from_item: undefined,
          }}
          aria-current={active === tab ? 'page' : undefined}
          className="shrink-0 rounded-lg border border-transparent px-3 py-1.5 text-sm text-muted-foreground no-underline outline-none transition-control hover:text-foreground aria-[current=page]:border-border/60 aria-[current=page]:bg-card aria-[current=page]:font-medium aria-[current=page]:text-foreground aria-[current=page]:shadow-[var(--design-shadow-card)] focus-visible:ring-2 focus-visible:ring-ring"
        >
          {definition.label}
        </Link>
      ))}
    </nav>
  )
}

export function PlanUnitTabs({
  active,
  route,
  operationAccess,
}: {
  active: PlanUnitTab
  route: Extract<ProjectWorkspaceRoute, { kind: 'plan-unit' }>
  operationAccess?: ProjectOperationAccess
}) {
  const embedded = useSessionStore((state) => state.mode === 'embedded')
  const visibleEntries = planUnitTabEntries.filter(([tab]) => {
    if (!operationAccess) return true
    if (tab === 'overview') return operationAccess.viewPlanUnits
    if (tab === 'requirements') return operationAccess.viewPlanRequirements
    if (tab === 'tasks') return operationAccess.viewTaskExecution
    if (tab === 'delivery') return operationAccess.viewDeliveryLabels
      && operationAccess.viewPlanRequirements
      && operationAccess.viewTaskExecution
    return operationAccess.viewActivity
      && operationAccess.viewPlanRequirements
      && operationAccess.viewTaskExecution
  })
  const entries = embedded
    ? visibleEntries.filter(([tab]) => tab !== 'tasks')
    : visibleEntries
  const routeSearch = route.search
  return (
    <nav
      aria-label={copy.projectManagement.planUnitDetail.heading}
      data-slot="pm-detail-tabs"
      className="sticky top-0 z-20 flex min-w-0 gap-1 overflow-x-auto rounded-lg bg-muted/60 p-1"
    >
      {entries.map(([tab, definition]) => (
        <Link
          key={tab}
          to="/workspace/projects/$projectId/items/$projectItemId"
          params={{ projectId: String(route.projectId), projectItemId: String(route.planUnitId) }}
          search={parsePlanUnitSearch({
            tab,
            req_q: routeSearch?.requirement.q,
            req_status: routeSearch?.requirement.status,
            req_priority: routeSearch?.requirement.priority,
            req_page: routeSearch && routeSearch.requirement.page > 1
              ? routeSearch.requirement.page
              : undefined,
            req_page_size: routeSearch && routeSearch.requirement.pageSize !== 50
              ? routeSearch.requirement.pageSize
              : undefined,
            task_requirement_id: routeSearch?.task.requirementId,
            task_id: routeSearch?.task.taskId,
            task_page: routeSearch && routeSearch.task.page > 1
              ? routeSearch.task.page
              : undefined,
            task_page_size: routeSearch && routeSearch.task.pageSize !== 50
              ? routeSearch.task.pageSize
              : undefined,
          })}
          aria-current={active === tab ? 'page' : undefined}
          className="shrink-0 rounded-lg border border-transparent px-3 py-1.5 text-sm text-muted-foreground no-underline outline-none transition-control hover:text-foreground aria-[current=page]:border-border/60 aria-[current=page]:bg-card aria-[current=page]:font-medium aria-[current=page]:text-foreground aria-[current=page]:shadow-[var(--design-shadow-card)] focus-visible:ring-2 focus-visible:ring-ring"
        >
          {definition.label}
        </Link>
      ))}
    </nav>
  )
}
