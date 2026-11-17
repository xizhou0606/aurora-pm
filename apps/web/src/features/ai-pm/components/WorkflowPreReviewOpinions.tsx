import { useState, type ReactNode } from 'react'
import { PhCircleNotch, PhPencil, PhPaperPlaneTilt, PhUserPlus } from '@/shared/ui/icons'
import { useSessionStore } from '@/shared/state'
import { Button, Skeleton, UserDirectoryMultiSelect, UserPreview } from '@/shared/ui'
import { copy } from '@/shared/copy'
import { platformUserFactId } from '@/shared/runtime'
import { useWorkflowAssignees, useWorkflowMutations, useWorkflowNodeOpinions } from '../api/api'
import { WorkflowRichTextBody, WorkflowRichTextField, workflowRichTextEmpty } from './WorkflowRichTextField'
import { workflowPeopleDirectory } from './WorkflowTaskPlanOwners'
import type { WorkflowNodeOpinion } from '../model/types'
import { useWorkflowOperationAccess } from '../hooks/useOperationAccess'

const subjectCopy = copy.aiPM.workflowSubject

function formatOpinionTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  }).format(date)
}

function opinionAuthorName(opinion: WorkflowNodeOpinion) {
  return opinion.author_display_name?.trim() || opinion.author_username?.trim() || subjectCopy.opinionAuthorUnknown
}

export function WorkflowPreReviewOpinions({
  workspaceId,
  instanceId,
  nodeId,
  readOnly = false,
  layout = 'stack',
  preview,
  composeFooter,
}: {
  workspaceId: number
  instanceId: number
  nodeId: string
  readOnly?: boolean
  layout?: 'stack' | 'split'
  preview?: ReactNode
  composeFooter?: ReactNode
}) {
  'use no memo'
  const sessionUser = useSessionStore((state) => state.user)
  const currentUserId = platformUserFactId(sessionUser)
  const opinions = useWorkflowNodeOpinions(workspaceId, instanceId, nodeId)
  const people = workflowPeopleDirectory(useWorkflowAssignees(workspaceId))
  const mutations = useWorkflowMutations(workspaceId)
  const access = useWorkflowOperationAccess()
  const canCreateOpinion = access.canCreateOpinion(!readOnly)
  const canUpdateOpinion = access.canUpdateOpinion(!readOnly)
  const [draft, setDraft] = useState('')
  const [inviteeIds, setInviteeIds] = useState<number[]>([])
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editDraft, setEditDraft] = useState('')

  const pending = mutations.createNodeOpinion.isPending || mutations.updateNodeOpinion.isPending || mutations.invitePreReview.isPending
  const rows = opinions.data ?? []

  async function submitCreate() {
    const body = draft.trim()
    if (workflowRichTextEmpty(body) || !access.canCreateOpinion(!readOnly) || pending) return
    await mutations.createNodeOpinion.mutateAsync({ instanceId, nodeId, body, invite_user_ids: inviteeIds })
    setDraft('')
  }

  async function submitInvite() {
    if (!access.canCreateOpinion(!readOnly) || pending || inviteeIds.length === 0) return
    await mutations.invitePreReview.mutateAsync({ instanceId, nodeId, invite_user_ids: inviteeIds })
  }

  async function submitEdit() {
    if (editingId == null || !access.canUpdateOpinion(!readOnly) || pending) return
    const body = editDraft.trim()
    if (workflowRichTextEmpty(body)) return
    await mutations.updateNodeOpinion.mutateAsync({ instanceId, nodeId, opinionId: editingId, body })
    setEditingId(null)
    setEditDraft('')
  }

  const split = layout === 'split'
  const thread = (
    <div className="grid min-w-0 gap-2.5">
      {preview}
      <header className="flex items-baseline justify-between gap-3">
        <h3 className="m-0 text-sm font-semibold text-foreground">{subjectCopy.opinionTitle}</h3>
        <span className="shrink-0 text-xs text-muted-foreground">{subjectCopy.historyCount(rows.length)}</span>
      </header>
      {canCreateOpinion ? <p className="m-0 text-xs leading-4 text-muted-foreground">{subjectCopy.opinionDescription}</p> : null}

      {opinions.isLoading ? <Skeleton className="h-16 w-full" /> : null}
      {opinions.isError ? <p role="alert" className="m-0 text-xs text-destructive">{subjectCopy.opinionLoadFailed}</p> : null}

      {!opinions.isLoading && !opinions.isError ? (
        <ul className="m-0 grid list-none gap-0 divide-y divide-border p-0">
          {rows.length === 0 ? (
            <li className="px-0 py-2 text-xs leading-5 text-muted-foreground">{subjectCopy.opinionEmpty}</li>
          ) : null}
          {rows.map((opinion) => {
            const editing = editingId === opinion.id
            const canEdit = canUpdateOpinion && currentUserId > 0 && opinion.author_user_id === currentUserId
            return (
              <li
                key={opinion.id}
                data-testid={`workflow-opinion-${opinion.id}`}
                className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-2 gap-y-1 py-2"
              >
                <UserPreview
                  name={opinionAuthorName(opinion)}
                  avatarUrl={opinion.author_avatar_url}
                  size="sm"
                  className="mt-0.5 self-start [&>span:last-child]:sr-only"
                />
                <div className="grid min-w-0 gap-1">
                  <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5 text-xs leading-4">
                    <strong className="truncate font-medium text-foreground">{opinionAuthorName(opinion)}</strong>
                    <time className="text-muted-foreground" dateTime={opinion.updated_at || opinion.created_at}>{formatOpinionTime(opinion.updated_at || opinion.created_at)}</time>
                    {opinion.edited ? <span className="text-muted-foreground">{subjectCopy.opinionEdited}</span> : null}
                    {canEdit && !editing ? (
                      <Button size="xs" variant="ghost" className="h-5 px-1 text-xs text-muted-foreground" onClick={() => { setEditingId(opinion.id); setEditDraft(opinion.body) }}>
                        <PhPencil className="size-3" />{subjectCopy.opinionEdit}
                      </Button>
                    ) : null}
                  </div>
                  {editing ? (
                    <div className="grid gap-1.5">
                      <WorkflowRichTextField
                        workspaceId={workspaceId}
                        instanceId={instanceId}
                        nodeId={nodeId}
                        label={subjectCopy.opinionEdit}
                        value={editDraft}
                        documentId={`edit-${opinion.id}`}
                        disabled={pending}
                        minHeight={88}
                        onChange={setEditDraft}
                      />
                      <div className="flex flex-wrap justify-end gap-1.5">
                        <Button size="xs" variant="outline" disabled={pending} onClick={() => { setEditingId(null); setEditDraft('') }}>{subjectCopy.opinionCancel}</Button>
                        <Button size="xs" disabled={pending || workflowRichTextEmpty(editDraft)} onClick={() => void submitEdit()}>{pending ? <PhCircleNotch className="animate-spin" /> : null}{subjectCopy.opinionSave}</Button>
                      </div>
                    </div>
                  ) : (
                    <WorkflowRichTextBody value={opinion.body} />
                  )}
                </div>
              </li>
            )
          })}
        </ul>
      ) : null}
    </div>
  )
  const composer = canCreateOpinion ? (
    <div className={split ? 'grid min-w-0 content-start gap-3' : 'grid gap-2 rounded-lg border border-border bg-muted/20 p-2.5'}>
      <UserDirectoryMultiSelect
        id={`pre-review-invite-${nodeId}`}
        label={subjectCopy.opinionInvite}
        description={inviteeIds.length
          ? `${subjectCopy.opinionInvited(inviteeIds.length)}。${subjectCopy.opinionInviteHint}`
          : subjectCopy.opinionInviteHint}
        placeholder={subjectCopy.opinionInvitePlaceholder}
        value={inviteeIds}
        state={people}
        disabled={pending}
        onValueChange={setInviteeIds}
      />
      <div className="flex justify-end">
        <Button size="xs" variant="outline" disabled={pending || inviteeIds.length === 0} onClick={() => void submitInvite()}>
          {mutations.invitePreReview.isPending ? <PhCircleNotch className="animate-spin" /> : <PhUserPlus className="size-3.5" />}
          {subjectCopy.opinionInviteSubmit}
        </Button>
      </div>
      <div className="grid min-w-0 gap-1.5">
        <WorkflowRichTextField
          workspaceId={workspaceId}
          instanceId={instanceId}
          nodeId={nodeId}
          label={subjectCopy.opinionTitle}
          value={draft}
          documentId={`draft-${nodeId}`}
          disabled={pending}
          placeholder={subjectCopy.opinionPlaceholder}
          minHeight={split ? 168 : 88}
          onChange={setDraft}
        />
        <div className="flex flex-wrap items-center justify-between gap-2">
          {(mutations.createNodeOpinion.isError || mutations.updateNodeOpinion.isError || mutations.invitePreReview.isError) ? (
            <p role="alert" className="m-0 text-xs text-destructive">
              {mutations.invitePreReview.isError ? subjectCopy.opinionInviteFailed : subjectCopy.opinionSubmitFailed}
            </p>
          ) : <span />}
          <Button size="sm" disabled={pending || workflowRichTextEmpty(draft)} onClick={() => void submitCreate()}>
            {pending ? <PhCircleNotch className="animate-spin" /> : <PhPaperPlaneTilt className="size-3.5" />}
            {subjectCopy.opinionSubmit}
          </Button>
        </div>
      </div>
      {composeFooter}
    </div>
  ) : composeFooter ?? null

  return (
    <section
      data-testid="workflow-pre-review-opinions"
      data-layout={layout}
      aria-label={subjectCopy.opinionTitle}
      className={split ? 'grid gap-6 lg:grid-cols-2 lg:gap-0' : 'grid gap-2.5'}
    >
      <div className={split ? 'min-w-0 lg:border-r lg:border-border lg:pr-6' : 'contents'}>
        {thread}
      </div>
      {composer ? (
        <div className={split ? 'min-w-0 lg:pl-6' : 'contents'}>
          {composer}
        </div>
      ) : null}
    </section>
  )
}
