import { useNavigate } from '@tanstack/react-router'
import { PhArrowsLeftRight } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
  Button,
} from '@/shared/ui'
import type { ProjectListItem } from '../model'

export function ProjectSwitcher({
  projects,
  currentProjectId,
}: {
  projects: readonly ProjectListItem[]
  currentProjectId: number
}) {
  const navigate = useNavigate()
  const projectCopy = copy.projectManagement.projectList

  if (projects.length <= 1) return null

  return (
    <Popover>
      <PopoverTrigger
        type="button"
        aria-label={projectCopy.switcherLabel ?? '切换项目集'}
        data-test="project-switcher-trigger"
        className="inline-flex size-8 items-center justify-center rounded-md border border-border bg-card text-muted-foreground hover:bg-muted hover:text-foreground"
      >
        <PhArrowsLeftRight aria-hidden="true" className="size-4" />
      </PopoverTrigger>
      <PopoverContent align="end" className="w-64 p-1.5">
        <ul className="grid max-h-72 gap-0.5 overflow-y-auto">
          {projects.map((project) => {
            const selected = project.id === currentProjectId
            return (
              <li key={project.id}>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  data-selected={selected || undefined}
                  className="flex w-full min-w-0 items-center justify-start gap-2 rounded-md px-2.5 py-2 text-left text-sm h-auto"
                  onClick={() => {
                    if (selected) return
                    void navigate({
                      to: '/workspace/projects/$projectId',
                      params: { projectId: String(project.id) },
                      search: { tab: 'overview', from_item: undefined },
                    })
                  }}
                >
                  <span className="min-w-0 flex-1 truncate">{project.name}</span>
                  {selected ? (
                    <span className="shrink-0 text-xs text-primary">{projectCopy.switcherCurrentLabel ?? '当前'}</span>
                  ) : null}
                </Button>
              </li>
            )
          })}
        </ul>
      </PopoverContent>
    </Popover>
  )
}