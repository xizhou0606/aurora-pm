import { useMemo } from 'react'
import { copy } from '@/shared/copy'
import { UserPreview } from '@/shared/ui'
import type { PlatformUser, ScrumProjectNew } from '../../api'

/**
 * Read-only project set creator card. Project sets carry a single membership
 * fact — their creator; owner/participant management lives on the plan units,
 * so this card replaces the retired set-level member editor.
 */
export function ProjectCreatorCard({
  project,
  workspaceUsers,
  title,
  className,
}: {
  project: ScrumProjectNew
  workspaceUsers: readonly PlatformUser[]
  title?: string
  className?: string
}) {
  const membersCopy = copy.projectManagement.members
  const usersById = useMemo(
    () => new Map(workspaceUsers.map((u) => [Number(u.id), u])),
    [workspaceUsers],
  )
  const creatorId = project.created_by_user_id ?? null
  const creator = creatorId != null ? usersById.get(creatorId) : undefined
  const creatorName = project.created_by_name?.trim() || (creator
    ? (creator.display_name || creator.username)
    : creatorId != null
      ? membersCopy.userFallback(creatorId)
      : membersCopy.noOwner)

  return (
    <div
      data-testid="pm-project-creator-card"
      className={
        className ??
        'rounded-lg border border-border bg-card p-4 sm:p-5'
      }
    >
      <h3 className="text-sm font-semibold text-foreground">
        {title ?? membersCopy.deliveryHeading}
      </h3>
      <dl className="mt-3 grid gap-2 text-sm">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <dt className="shrink-0 text-muted-foreground">{membersCopy.creatorLabel}</dt>
          <dd className="min-w-0">
            <UserPreview
              name={creatorName}
              avatarUrl={creator?.avatar_url}
              size="sm"
            />
          </dd>
        </div>
      </dl>
      <p className="mt-3 text-xs text-muted-foreground">{membersCopy.creatorHint}</p>
    </div>
  )
}
