import { useParams, useSearch } from '@tanstack/react-router'

import { ProjectWorkspace } from '../components'
import { resolvePlanUnitTab, resolveProjectDetailTab } from '../model'

export function ProjectsPage() {
  const search = useSearch({ from: '/workspace/projects' })
  return <ProjectWorkspace route={{ kind: 'list', projectId: search.project_id }} />
}

export function ProjectDetailPage() {
  const params = useParams({ from: '/workspace/projects/$projectId' })
  const search = useSearch({ from: '/workspace/projects/$projectId' })
  return (
    <ProjectWorkspace
      route={{
        kind: 'project',
        projectId: Number(params.projectId),
        tab: resolveProjectDetailTab(search.tab),
        fromItemId: search.from_item,
      }}
    />
  )
}

export function ProjectItemDetailPage() {
  const params = useParams({
    from: '/workspace/projects/$projectId/items/$projectItemId',
  })
  const search = useSearch({
    from: '/workspace/projects/$projectId/items/$projectItemId',
  })
  return (
    <ProjectWorkspace
      route={{
        kind: 'plan-unit',
        projectId: Number(params.projectId),
        planUnitId: Number(params.projectItemId),
        tab: resolvePlanUnitTab(search.tab),
        search: {
          requirement: {
            q: search.req_q ?? '',
            status: search.req_status,
            priority: search.req_priority,
            page: search.req_page ?? 1,
            pageSize: search.req_page_size ?? 50,
          },
          task: {
            requirementId: search.task_requirement_id,
            taskId: search.task_id,
            page: search.task_page ?? 1,
            pageSize: search.task_page_size ?? 50,
          },
        },
      }}
    />
  )
}
