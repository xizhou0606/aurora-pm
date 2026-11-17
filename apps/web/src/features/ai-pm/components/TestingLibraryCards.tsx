import { copy } from '@/shared/copy'
import { Button, EmptyState, Skeleton, UserPreview } from '@/shared/ui'
import { testingLibraryCoverSrc } from '../utils/testing-library-covers'
import type { TestingLibraryItem } from '../model/types'
import { saveWorkflowNodeArtifact } from '../utils/workflow-artifact-file'

const libraryCopy = copy.aiPM.testingLibrary

function formatDate(value: string) {
  const parsed = Date.parse(value)
  if (!Number.isFinite(parsed)) return ''
  return new Date(parsed).toLocaleDateString('zh-CN')
}

function projectPath(item: TestingLibraryItem) {
  const project = item.project_name || libraryCopy.unknownProject
  return item.plan_unit_title ? `${project} / ${item.plan_unit_title}` : project
}

function groupTestingItems(rows: TestingLibraryItem[]) {
  const groups = new Map<string, { projectId: number; projectName: string; items: TestingLibraryItem[] }>()
  for (const item of rows) {
    const projectId = item.project_id ?? 0
    const projectName = item.project_name || libraryCopy.unknownProject
    const key = `${projectId}:${projectName}`
    const current = groups.get(key) ?? { projectId, projectName, items: [] }
    current.items.push(item)
    groups.set(key, current)
  }
  return [...groups.values()]
}

export function TestingLibraryCards({
  rows,
  pending,
  error,
  kind,
  canOpen,
  onRetry,
}: {
  rows: TestingLibraryItem[]
  pending: boolean
  error: boolean
  kind: 'test_case' | 'test_report'
  canOpen: boolean
  onRetry: () => void
}) {
  if (pending) {
    return (
      <ul className="m-0 grid list-none grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-4 p-0">
        {Array.from({ length: 6 }, (_, index) => (
          <li key={index} className="overflow-hidden rounded-lg border border-border bg-card">
            <Skeleton className="aspect-[2/1] rounded-none" />
            <div className="grid gap-2 p-3">
              <Skeleton className="h-4 w-12" />
              <Skeleton className="h-5 w-4/5" />
              <Skeleton className="h-4 w-3/5" />
            </div>
          </li>
        ))}
      </ul>
    )
  }
  if (error) {
    return (
      <EmptyState
        title={libraryCopy.loadFailedTitle}
        action={<Button type="button" variant="outline" onClick={onRetry}>{copy.aiPM.common.retry}</Button>}
      />
    )
  }
  if (!rows.length) {
    return <EmptyState title={libraryCopy.emptyTitle} />
  }
  const groups = groupTestingItems(rows)
  return (
    <div className="grid gap-6" aria-label={libraryCopy.cards}>
      {groups.map((group) => (
        <section key={`${group.projectId}-${group.projectName}`} className="grid gap-3">
          <h2 className="m-0 text-sm font-semibold text-foreground">
            {group.projectName}
            <span className="ml-2 font-normal text-muted-foreground"> {libraryCopy.itemCount(group.items.length)}</span>
          </h2>
          <ul className="m-0 grid list-none grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-4 p-0">
            {group.items.map((item) => (
              <TestingLibraryCard key={item.id} item={item} kind={kind} canOpen={canOpen} />
            ))}
          </ul>
        </section>
      ))}
    </div>
  )
}

function TestingLibraryCard({ item, kind, canOpen }: { item: TestingLibraryItem; kind: 'test_case' | 'test_report'; canOpen: boolean }) {
  const open = () => {
    void saveWorkflowNodeArtifact(item)
  }
  return (
    <li>
      <Button
        type="button"
        variant="outline"
        className="grid h-auto w-full overflow-hidden whitespace-normal rounded-lg border-border bg-card p-0 text-left transition-colors hover:border-primary/40 hover:bg-card"
        disabled={!canOpen}
        onClick={open}
        aria-label={libraryCopy.openItem(item.name)}
      >
        <img
          src={testingLibraryCoverSrc(kind, item.id)}
          alt=""
          className="aspect-[2/1] w-full object-cover"
        />
        <span className="grid gap-1 p-3">
          <span className="text-xs text-muted-foreground">{kind === 'test_report' ? libraryCopy.typeReport : libraryCopy.typeCase}</span>
          <strong className="text-sm leading-5 text-foreground">{item.subject_title || item.name}</strong>
          <span className="text-xs text-muted-foreground">{projectPath(item)}</span>
          <span className="mt-1 flex min-w-0 items-center justify-between gap-2 border-t border-border pt-2">
            {item.uploader_name ? (
              <UserPreview
                size="sm"
                name={item.uploader_name}
                avatarUrl={item.uploader_avatar_url}
                className="min-w-0"
              />
            ) : <span />}
            <span className="shrink-0 text-xs text-muted-foreground">{formatDate(item.created_at)}</span>
          </span>
        </span>
      </Button>
    </li>
  )
}
