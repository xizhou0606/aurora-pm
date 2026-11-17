import { QUALITY_COPY } from '@/shared/copy'
import {
  ActivityTimeline,
  Button,
  DetailSection,
  EmptyState,
  EntityDetailLayout,
  LazyRichTextField,
  MetadataItem,
  MetadataList,
  MetadataRail,
  UserPreview,
} from '@/shared/ui'

import type { QualityActivitiesState, QualityCommentsState } from '../../hooks'
import type {
  QualityBug,
  QualityComment,
  QualityLifecycleAction,
  QualityLifecycleModel,
  QualityUsersState,
  QualityMutationState,
} from '../../model/types'
import { formatDateOnly } from '@/shared/lib/date-boundary'
import type { PMScopeOptions } from '@/shared/api'
import { LifecycleBar } from './LifecycleBar'
import { Materials } from './Materials'
import { UserDirectoryStatus } from '../list/UserDirectoryStatus'
import { Collaboration } from './Collaboration'
import { activityTimelineState } from './activityTimelineState'

function Metadata({
  bug,
  users,
  onOpenRequirement,
}: {
  bug: QualityBug
  users: QualityUsersState
  onOpenRequirement: (requirementId: number) => void
}) {
  const directory = new Map(users.rows.map((user) => [user.id, user]))
  const identity = (userId: number | undefined, fallback: string) => {
    const user = userId ? directory.get(userId) : undefined
    return (
      <UserPreview
        name={user?.name || fallback}
        secondary={user?.username}
        avatarUrl={user?.avatarUrl}
      />
    )
  }
  const hierarchyEntries = [
    [QUALITY_COPY.detail.status, bug.status ? QUALITY_COPY.enums.status[bug.status] || bug.status : ''],
    [QUALITY_COPY.detail.severity, bug.severity ? QUALITY_COPY.enums.severity[bug.severity] || bug.severity : ''],
    [QUALITY_COPY.detail.projectSet, bug.projectSetName],
    [QUALITY_COPY.detail.project, bug.projectName],
  ]
  const detailEntries = [
    [QUALITY_COPY.detail.type, bug.bugTypeLabel || (bug.bugType ? QUALITY_COPY.enums.type[bug.bugType] : '')],
    [QUALITY_COPY.detail.defectDomain, QUALITY_COPY.enums.defectDomain[bug.defectDomain === 'ui' ? 'ui' : 'functional']],
    [QUALITY_COPY.detail.dueDate, formatDateOnly(bug.dueDate)],
    [QUALITY_COPY.detail.environment, bug.foundEnvironment],
    [QUALITY_COPY.detail.operatingSystem, bug.operatingSystem],
    [QUALITY_COPY.detail.browser, bug.browser],
  ]
  return (
    <MetadataRail title={QUALITY_COPY.detail.properties}>
      <UserDirectoryStatus state={users} />
      <MetadataList>
        {hierarchyEntries.map(([label, value]) => (
          <MetadataItem key={label} label={label}>
            <span className="block truncate">{value || QUALITY_COPY.list.notSet}</span>
          </MetadataItem>
        ))}
        <MetadataItem label={QUALITY_COPY.detail.requirement}>
          {bug.requirementId ? (
            <Button
              type="button"
              variant="link"
              size="sm"
              className="h-auto min-w-0 max-w-full justify-start p-0 text-left font-normal text-primary no-underline hover:underline"
              aria-label={QUALITY_COPY.list.openRequirement(
                bug.requirementTitle || QUALITY_COPY.list.fallbackRequirement(bug.requirementId),
              )}
              onClick={() => onOpenRequirement(bug.requirementId as number)}
            >
              <span className="block min-w-0 truncate">
                {bug.requirementTitle || QUALITY_COPY.list.fallbackRequirement(bug.requirementId)}
              </span>
            </Button>
          ) : QUALITY_COPY.list.notSet}
        </MetadataItem>
        {detailEntries.map(([label, value]) => (
          <MetadataItem key={label} label={label}>
            <span className="block truncate">{value || QUALITY_COPY.list.notSet}</span>
          </MetadataItem>
        ))}
        <MetadataItem label={QUALITY_COPY.detail.assignee}>{identity(bug.assigneeUserId, bug.assigneeName || QUALITY_COPY.list.unassigned)}</MetadataItem>
        <MetadataItem label={QUALITY_COPY.detail.reporter}>{identity(bug.reporterUserId, bug.reporterName || QUALITY_COPY.list.unknown)}</MetadataItem>
        <MetadataItem label={QUALITY_COPY.detail.reviewer}>{identity(bug.reviewerUserId, bug.reviewerName || QUALITY_COPY.list.unassigned)}</MetadataItem>
      </MetadataList>
    </MetadataRail>
  )
}

export function ReadView({
  bug,
  users,
  scopeOptions,
  activities,
  comments,
  currentUserId,
  commentMutation,
  canViewComments,
  canCreateComment,
  canDeleteComment,
  canUpload,
  onComment,
  onDeleteComment,
  onUploadCommentImage,
  lifecycle,
  onLifecycleAction,
  onPreview,
  onOpenRequirement,
}: {
  bug: QualityBug
  users: QualityUsersState
  scopeOptions: PMScopeOptions | null
  activities: QualityActivitiesState
  comments: QualityCommentsState
  currentUserId: number
  commentMutation: QualityMutationState
  canViewComments: boolean
  canCreateComment: boolean
  canDeleteComment: (comment: QualityComment) => boolean
  canUpload: boolean
  onComment: (body: string) => Promise<boolean>
  onDeleteComment: (id: number) => Promise<boolean>
  onUploadCommentImage: (file: File) => Promise<string | null>
  lifecycle: QualityLifecycleModel
  onLifecycleAction: (action: QualityLifecycleAction) => void
  onPreview: (url: string, trigger: HTMLElement) => void
  onOpenRequirement: (requirementId: number) => void
}) {
  const hasResolution = Boolean(
    bug.resolutionReason || bug.resolutionReasonLabel || bug.resolutionNote || bug.resolutionMaterials.length,
  )
  const expectedResult = bug.expectedResult?.trim()
  const actualResult = bug.actualResult?.trim()
  const hasLegacyResults = Boolean(expectedResult || actualResult)

  return (
    <div className="relative min-w-0 pb-3">
      <EntityDetailLayout
        ariaLabel={bug.title}
        main={(
          <>
            <DetailSection title={QUALITY_COPY.detail.reproduction}>
              {bug.reproductionSteps ? (
                <LazyRichTextField
                  label={QUALITY_COPY.detail.reproduction}
                  value={bug.reproductionSteps}
                  serialization="json"
                  visuallyHiddenLabel
                  readOnly
                  minHeight={96}
                  onPreviewImage={onPreview}
                />
              ) : <EmptyState title={QUALITY_COPY.detail.noReproduction} />}
            </DetailSection>
            {hasLegacyResults ? (
              <DetailSection title={QUALITY_COPY.form.results} contentClassName="grid gap-4 sm:grid-cols-2">
                {expectedResult ? <div><h3 className="text-sm font-semibold">{QUALITY_COPY.detail.expected}</h3><p className="mt-2 whitespace-pre-wrap text-sm text-muted-foreground">{expectedResult}</p></div> : null}
                {actualResult ? <div><h3 className="text-sm font-semibold">{QUALITY_COPY.detail.actual}</h3><p className="mt-2 whitespace-pre-wrap text-sm text-muted-foreground">{actualResult}</p></div> : null}
              </DetailSection>
            ) : null}
            <DetailSection title={QUALITY_COPY.detail.materials}>
              <Materials items={bug.reproductionMaterials} onPreview={onPreview} />
            </DetailSection>
            {hasResolution ? (
              <DetailSection title={QUALITY_COPY.lifecycle.resolveTitle} contentClassName="grid gap-3">
                <dl className="grid grid-cols-[minmax(7rem,auto)_minmax(0,1fr)] gap-x-3 gap-y-2 text-sm">
                  <div className="contents"><dt className="text-muted-foreground">{QUALITY_COPY.lifecycle.resolutionReason}</dt><dd>{bug.resolutionReasonLabel || bug.resolutionReason || QUALITY_COPY.list.notSet}</dd></div>
                  <div className="contents">
                    <dt className="text-muted-foreground">{QUALITY_COPY.lifecycle.resolutionNote}</dt>
                    <dd className="min-w-0">
                      {bug.resolutionNote ? (
                        <LazyRichTextField
                          label={QUALITY_COPY.lifecycle.resolutionNote}
                          value={bug.resolutionNote}
                          serialization="json"
                          visuallyHiddenLabel
                          readOnly
                          minHeight={72}
                          onPreviewImage={onPreview}
                        />
                      ) : QUALITY_COPY.list.notSet}
                    </dd>
                  </div>
                </dl>
                <Materials items={bug.resolutionMaterials} onPreview={onPreview} />
              </DetailSection>
            ) : null}
            <Collaboration
              bugId={bug.id}
              currentUserId={currentUserId}
              comments={comments}
              users={users}
              commentMutation={commentMutation}
              canViewComments={canViewComments}
              canCreateComment={canCreateComment}
              canDeleteComment={canDeleteComment}
              canUpload={canUpload}
              onComment={onComment}
              onDeleteComment={onDeleteComment}
              onUploadImage={onUploadCommentImage}
              activity={(
                <ActivityTimeline
                  state={activityTimelineState(activities, users, scopeOptions, onPreview)}
                />
              )}
            />
          </>
        )}
        rail={<Metadata bug={bug} users={users} onOpenRequirement={onOpenRequirement} />}
      />
      <LifecycleBar model={lifecycle} onAction={onLifecycleAction} />
    </div>
  )
}
