import type {
  BatchMutationResult,
  DetailNavigation,
  DetailResourceEntry,
  MutationState,
  ResourceState,
  UserOptionsState,
  WorkItem,
  WorkItemAttachment,
  WorkItemChildCreatePayload,
  WorkItemChildResource,
  WorkItemDetailPatch,
  WorkItemPatch,
  WorkItemRelated,
  WorkItemsDownload,
  WorkItemStatusOption,
  WorkItemStatusTransition,
  WorkItemClientOption,
} from '../../model/types'
import { EntityDetailLayout } from '@/shared/ui'
import { AttachmentSection } from './AttachmentSection'
import { Children } from './Children'
import { Collaboration, DetailDescription, DetailHeader } from './index'
import { DetailAside } from './DetailAside'
import type { WorkItemDetailDraftControl } from '../../hooks'

export type WorkItemDetailView = {
  state: { status: string; data?: WorkItem; retry?: () => void; refresh?: string }
  resources: readonly DetailResourceEntry[]
  children: WorkItemChildResource
  attachments: ResourceState<WorkItemAttachment[]>
  commentUsers: UserOptionsState
  transitions?: ResourceState<WorkItemStatusTransition[]>
  workflow?: ResourceState<readonly WorkItemStatusOption[]>
  navigation?: DetailNavigation
  parent?: WorkItemRelated
  currentUserId: number
  clients?: readonly WorkItemClientOption[]
}

export type WorkItemDetailActionState = {
  update: MutationState
  delete: MutationState
  comment: MutationState
  attachment: MutationState
  watch: MutationState
  child: MutationState
  title?: MutationState
  description?: MutationState
  field?: MutationState
}

export type WorkItemDetailCapabilities = {
  uploadAttachments: boolean
  downloadAttachments: boolean
  deleteAttachments: boolean
  createChild: boolean
}

export type WorkItemDetailActions = {
  update: (row: WorkItem, patch: WorkItemPatch) => Promise<boolean>
  delete: (row: WorkItem, options?: { cascade?: boolean }) => Promise<boolean>
  comment: (body: string) => Promise<boolean>
  uploadAttachment: (file: File) => Promise<boolean>
  uploadCommentImage: (file: File) => Promise<string | null>
  uploadRichTextImage: (file: File) => Promise<string | null>
  deleteAttachment: (id: number) => Promise<boolean>
  downloadAttachment: (id: number) => Promise<WorkItemsDownload | null>
  toggleWatch: () => Promise<boolean>
  createChildren: (
    drafts: readonly WorkItemChildCreatePayload[],
  ) => Promise<BatchMutationResult>
  saveTitle?: (title: string) => Promise<boolean>
  saveDescription?: (description: string) => Promise<boolean>
  saveField?: (patch: WorkItemDetailPatch) => Promise<boolean>
}

type DetailBodyProps = {
  resources: readonly DetailResourceEntry[]
  children: WorkItemChildResource
  commentState: MutationState
  attachments: ResourceState<WorkItemAttachment[]>
  attachmentState: MutationState
  childState: MutationState
  canUploadAttachments: boolean
  canDownloadAttachments: boolean
  canDeleteAttachments: boolean
  canCreateChild: boolean
  currentUserId: number
  clients?: readonly WorkItemClientOption[]
  draft: WorkItemDetailDraftControl
  editing: boolean
  statusEditing: boolean
  onComment: (body: string) => Promise<boolean>
  onUploadAttachment: (file: File) => Promise<boolean>
  onUploadCommentImage: (file: File) => Promise<string | null>
  onUploadRichTextImage: (file: File) => Promise<string | null>
  onDeleteAttachment: (id: number) => Promise<boolean>
  onDownloadAttachment: (id: number) => Promise<WorkItemsDownload | null>
  onCreateChildren: (
    drafts: readonly WorkItemChildCreatePayload[],
  ) => Promise<BatchMutationResult>
  commentUsers: UserOptionsState
  transitions?: ResourceState<WorkItemStatusTransition[]>
  workflow?: ResourceState<readonly WorkItemStatusOption[]>
  navigation?: DetailNavigation
  parent?: WorkItemRelated
}

function requiredResource(
  resources: readonly DetailResourceEntry[],
  id: DetailResourceEntry['id'],
): DetailResourceEntry {
  const entry = resources.find((candidate) => candidate.id === id)
  if (!entry) throw new Error(`missing_work_item_detail_resource:${id}`)
  return entry
}

export function DetailBody({
  item,
  resources,
  children,
  draft,
  editing,
  statusEditing,
  commentState,
  attachments,
  attachmentState,
  childState,
  transitions,
  workflow,
  navigation,
  parent,
  canUploadAttachments,
  canDownloadAttachments,
  canDeleteAttachments,
  canCreateChild,
  currentUserId,
  onComment,
  onUploadAttachment,
  onUploadCommentImage,
  onUploadRichTextImage,
  onDeleteAttachment,
  onDownloadAttachment,
  onCreateChildren,
  commentUsers,
  clients = [],
}: DetailBodyProps & { item: WorkItem }) {
  const activities = requiredResource(resources, 'activities')
  const comments = requiredResource(resources, 'comments')
  return (
    <EntityDetailLayout
      ariaLabelledBy="work-items-detail-title"
      header={(
        <DetailHeader
          item={item}
          value={draft.value.title}
          editable={editing}
          saveState={draft.saveState}
          onChange={(value) => draft.change('title', value)}
        />
      )}
      main={(
        <>
          <DetailDescription
            item={item}
            value={draft.value.description}
            editable={editing}
            saveState={draft.saveState}
            canUploadBinary={canUploadAttachments}
            onChange={(value) => draft.change('description', value)}
            onUploadImage={onUploadRichTextImage}
            onUploadAttachment={onUploadAttachment}
          />
          <Children
            resource={children}
            users={commentUsers}
            workflow={workflow?.status === 'ready' ? workflow.data : []}
            mutation={childState}
            canCreate={canCreateChild}
            create={onCreateChildren}
          />
          <AttachmentSection
            state={attachments}
            canUpload={item.capabilities.canAttach}
            canDelete={canDeleteAttachments}
            canUploadBinary={canUploadAttachments}
            canDownloadBinary={canDownloadAttachments}
            mutation={attachmentState}
            onUpload={onUploadAttachment}
            onDelete={onDeleteAttachment}
            onDownload={onDownloadAttachment}
          />
          <Collaboration
            taskId={item.id}
            activities={activities}
            comments={comments}
            transitions={transitions || { status: 'idle' }}
            commentState={commentState}
            commentUsers={commentUsers}
            currentUserId={currentUserId}
            canComment={item.capabilities.canComment}
            canAttach={item.capabilities.canAttach}
            canUploadBinary={canUploadAttachments}
            onComment={onComment}
            onUploadCommentImage={onUploadCommentImage}
            onDownloadAttachment={onDownloadAttachment}
          />
        </>
      )}
      rail={(
        <DetailAside
          item={item}
          draft={draft.value}
          editable={editing}
          statusEditable={statusEditing}
          saveState={draft.saveState}
          users={commentUsers}
          clients={clients}
          workflow={workflow || { status: 'idle' }}
          onOpenRequirement={navigation?.openRequirement}
          parent={parent}
          onOpenParent={navigation?.openTask}
          onChange={draft.change}
        />
      )}
    />
  )
}
