import { copy } from '@/shared/copy'
import {
  Button,
  EmptyState,
  Input,
  Label,
  LoadingSkeleton,
  Pagination,
  StatusPill,
  UserPreview,
} from '@/shared/ui'
import { projectStatusTone, type ProjectListState } from '../model'

type Props = {
  state: ProjectListState
  search: {
    value: string
    onChange: (value: string) => void
  }
  pagination: {
    page: number
    pageSize: number
    onChange: (page: number) => void
  }
  actions: ProjectListActions
}

export type ProjectListItemActions = {
  edit?: () => void
  delete?: () => void
}

export type ProjectListActions = {
  create?: () => void
  select: (projectId: number) => void
  items: ReadonlyMap<number, ProjectListItemActions>
}

const projectCopy = copy.projectManagement.projectList

function healthTone(value: string) {
  if (value === '正常') return 'success' as const
  if (value === '关注') return 'warning' as const
  if (value === '高风险') return 'danger' as const
  return 'neutral' as const
}

function assertNever(value: never): never {
  throw new Error(`Unhandled project list state: ${JSON.stringify(value)}`)
}

function ProjectListBody({
  state,
  actions,
}: {
  state: ProjectListState
  actions: ProjectListActions
}) {
  switch (state.status) {
    case 'loading':
      return <LoadingSkeleton rows={3} label={projectCopy.loading} className="p-3" />
    case 'error':
      return (
        <div role="alert" className="p-3">
          <EmptyState
            title={projectCopy.errorTitle}
            action={(
              <Button type="button" variant="outline" size="sm" onClick={state.retry}>
                {copy.projectManagement.common.retry}
              </Button>
            )}
          />
        </div>
      )
    case 'empty':
      return (
        <EmptyState
          role="status"
          title={state.reason === 'filtered' ? projectCopy.filteredEmptyTitle : projectCopy.emptyTitle}
          className="m-auto"
        />
      )
    case 'ready':
      return (
        <ul className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-1.5" data-test="project-set-list">
          {state.items.map((project) => (
            <li
              key={project.id}
              data-selected={project.selected}
              className="pm-project-card relative min-w-0 overflow-hidden rounded-lg border border-transparent bg-card last:mb-0 hover:border-border"
            >
              <span
                aria-hidden="true"
                className="pm-project-card__indicator absolute left-0 bg-transparent transition-colors"
              />
              <Button
                type="button"
                variant="ghost"
                aria-pressed={project.selected}
                data-test="project-set-item"
                className="h-auto w-full min-w-0 flex-col items-stretch gap-2.5 whitespace-normal rounded-lg border border-transparent px-3 py-2.5 text-left text-foreground"
                data-selected={project.selected}
                onClick={() => actions.select(project.id)}
              >
                <span className="min-w-0">
                  <strong className="block truncate text-sm font-semibold">
                    {project.name}
                  </strong>
                  <span className="mt-0.5 block truncate text-xs leading-4 text-muted-foreground">
                    {project.description}
                  </span>
                </span>
                <span className="flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                  <UserPreview
                    name={project.owner.name}
                    secondary={project.owner.secondary}
                    avatarUrl={project.owner.avatarUrl}
                    size="sm"
                    className="mr-0.5 max-w-[7rem] shrink"
                  />
                  <StatusPill tone={projectStatusTone(project.statusKey)} className="min-w-0 px-1.5 py-0 text-xs">{project.status}</StatusPill>
                  <StatusPill tone={healthTone(project.health)} className="min-w-0 px-1.5 py-0 text-xs">{project.health}</StatusPill>
                  {project.healthReason ? <span className="sr-only">{project.healthReason}</span> : null}
                </span>
                <span
                  className="grid min-w-0 grid-cols-3 gap-1.5 text-xs text-muted-foreground"
                  data-test="project-set-stats"
                >
                  <span className="flex min-w-0 flex-col whitespace-nowrap">
                    <strong className="truncate text-sm text-foreground">{project.planUnitCount}</strong>
                    <small className="truncate text-xs">{projectCopy.planUnitLabel}</small>
                  </span>
                  <span className="flex min-w-0 flex-col whitespace-nowrap">
                    <strong className="truncate text-sm text-foreground">{project.requirementCount}</strong>
                    <small className="truncate text-xs">{projectCopy.requirementLabel}</small>
                  </span>
                  <span className="flex min-w-0 flex-col whitespace-nowrap">
                    <strong className="truncate text-sm text-foreground">
                      {projectCopy.progressValue(project.progress)}
                    </strong>
                    <small className="truncate text-xs">{projectCopy.progressLabel}</small>
                  </span>
                </span>
                <span aria-hidden="true" className="h-1 overflow-hidden rounded-full bg-muted">
                  <span
                    className="block h-full w-(--project-progress-width) rounded-full bg-primary transition-[width]"
                    style={{ '--project-progress-width': `${project.progress}%` }}
                  />
                </span>
              </Button>
              {actions.items.get(project.id) ? (
                <div className="flex flex-wrap justify-end gap-1 border-t border-border/60 px-2 py-1.5">
                  {actions.items.get(project.id)?.edit ? (
                    <Button
                      type="button"
                      size="xs"
                      variant="ghost"
                      aria-label={copy.projectManagement.write.actions.editProjectLabel(project.name)}
                      onClick={actions.items.get(project.id)?.edit}
                    >
                      {copy.projectManagement.write.actions.editProject}
                    </Button>
                  ) : null}
                  {actions.items.get(project.id)?.delete ? (
                    <Button
                      type="button"
                      size="xs"
                      variant="ghost"
                      aria-label={copy.projectManagement.write.actions.deleteProjectLabel(project.name)}
                      onClick={actions.items.get(project.id)?.delete}
                    >
                      {copy.projectManagement.write.actions.deleteProject}
                    </Button>
                  ) : null}
                </div>
              ) : null}
            </li>
          ))}
        </ul>
      )
    default:
      return assertNever(state)
  }
}

export function ProjectSetNav({ state, search, pagination, actions }: Props) {
  return (
    <aside
      aria-labelledby="pm-project-list-heading"
      data-test="pm-project-sidebar"
      className="flex h-full min-h-0 min-w-0 flex-col overflow-hidden rounded-lg border border-border bg-card"
    >
      <header className="pm-project-list__header flex items-center justify-between border-b border-border px-3.5 py-3">
        <div>
          <h2 id="pm-project-list-heading" className="text-sm font-semibold">
            {projectCopy.heading}
          </h2>
          <p className="mt-0.5 text-xs text-muted-foreground" aria-live="polite">
            {state.summary}
          </p>
        </div>
        {actions.create ? (
          <Button type="button" size="xs" onClick={actions.create}>
            {copy.projectManagement.write.actions.createProject}
          </Button>
        ) : null}
      </header>

      <div className="border-b border-border p-2">
        <Label htmlFor="pm-project-search" className="sr-only">{projectCopy.searchLabel}</Label>
        <Input
          id="pm-project-search"
          value={search.value}
          placeholder={projectCopy.searchPlaceholder}
          onChange={(event) => search.onChange(event.target.value)}
        />
      </div>

      <ProjectListBody state={state} actions={actions} />
      {state.status === 'ready' && state.total > 0 ? (
        <Pagination
          aria-label={projectCopy.paginationLabel}
          className="shrink-0 border-t border-border p-2"
          page={pagination.page}
          pageSize={pagination.pageSize}
          pageSizes={[pagination.pageSize]}
          showPageSize={false}
          total={state.total}
          onPageChange={pagination.onChange}
          onPageSizeChange={() => undefined}
        />
      ) : null}
    </aside>
  )
}
