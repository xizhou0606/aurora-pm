import { copy } from '@/shared/copy'
import {
  Button,
  DateTimePicker,
  EmptyState,
  LoadingSkeleton,
  ResponsivePanel,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  UserDirectorySelect,
} from '@/shared/ui'
import {
  projectDeliveryNodeIdentity,
  projectDeliveryNodeLabel,
  projectDeliveryNodeLabelIdentity,
} from '@/features/project-management/utils/projectDeliveryNodes'
import type {
  ScrumProjectDeliveryNodeLabel,
} from '../../api'
import type {
  DeliveryDraft,
  DeliveryEditorState,
  OwnerDirectoryState,
} from '../../model'
import { draftDateValue } from '../../model/form-utils'
import { WriteFormField } from '../WriteFormField'

const deliveryCopy = copy.projectManagement.delivery

type LabelCatalogState =
  | { status: 'loading'; writable: false }
  | { status: 'error'; retry: () => void; writable: false }
  | { status: 'empty'; refreshing: boolean; staleError?: { retry: () => void }; writable: boolean }
  | {
      status: 'ready'
      items: readonly ScrumProjectDeliveryNodeLabel[]
      refreshing: boolean
      staleError?: { retry: () => void }
      writable: boolean
    }

export type NodeEditorControl = {
  labels: LabelCatalogState
  available: readonly ScrumProjectDeliveryNodeLabel[]
  owners: OwnerDirectoryState
  onAdd: (label: ScrumProjectDeliveryNodeLabel) => void
  onRemove: (nodeKey: string) => void
  onNodeChange: (
    nodeKey: string,
    patch: {
      due_at?: string | null
      actual_completed_at?: string | null
      owner_user_id?: number | null
    },
  ) => void
  onManageLabels?: () => void
  onClose: () => void
  onSubmit: () => void
}

function catalogItems(state: LabelCatalogState): readonly ScrumProjectDeliveryNodeLabel[] {
  return state.status === 'ready' ? state.items : []
}

function NodeRows({
  draft,
  labels,
  owners,
  locked,
  onRemove,
  onNodeChange,
}: {
  draft: DeliveryDraft
  labels: readonly ScrumProjectDeliveryNodeLabel[]
  owners: OwnerDirectoryState
  locked: boolean
  onRemove: NodeEditorControl['onRemove']
  onNodeChange: NodeEditorControl['onNodeChange']
}) {
  if (draft.nodes.length === 0) {
    return (
      <EmptyState
        title={deliveryCopy.noSelectedTitle}
        className="border border-dashed border-border"
      />
    )
  }
  return (
    <div className="grid gap-3">
      {draft.nodes.map((node, index) => {
        const nodeKey = projectDeliveryNodeIdentity(node)
        const idBase = `pm-delivery-node-${index}`
        return (
          <article key={nodeKey || idBase} className="rounded-lg border border-border bg-card p-3">
            <header className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <h3 className="truncate text-sm font-semibold text-foreground">
                  {projectDeliveryNodeLabel(node, labels)}
                </h3>
                {node.label_deleted ? (
                  <p className="mt-1 text-xs text-muted-foreground">{deliveryCopy.deletedLabel}</p>
                ) : null}
              </div>
              <Button
                type="button"
                size="xs"
                variant="destructive"
                disabled={locked}
                onClick={() => onRemove(nodeKey)}
              >
                {deliveryCopy.removeNode}
              </Button>
            </header>
            <div className="mt-3 grid gap-3 sm:grid-cols-3">
              <WriteFormField id={`${idBase}-due`} label={deliveryCopy.dueAt}>
                <DateTimePicker
                  id={`${idBase}-due`}
                  value={draftDateValue(node.due_at, true)}
                  disabled={locked}
                  onChange={(value) => onNodeChange(nodeKey, {
                    due_at: value || null,
                  })}
                />
              </WriteFormField>
              <WriteFormField id={`${idBase}-actual`} label={deliveryCopy.actualCompletedAt}>
                <DateTimePicker
                  id={`${idBase}-actual`}
                  value={draftDateValue(node.actual_completed_at, true)}
                  defaultMonth={draftDateValue(node.due_at, true)}
                  disabled={locked}
                  onChange={(value) => onNodeChange(nodeKey, {
                    actual_completed_at: value || null,
                  })}
                />
              </WriteFormField>
              <UserDirectorySelect
                id={`${idBase}-owner`}
                label={deliveryCopy.owner}
                value={node.owner_user_id || null}
                state={owners}
                disabled={locked}
                onValueChange={(ownerUserId) => onNodeChange(nodeKey, {
                  owner_user_id: ownerUserId ?? null,
                })}
              />
            </div>
          </article>
        )
      })}
    </div>
  )
}

export function NodeEditor({
  state,
  targetTitle,
  control,
}: {
  state: DeliveryEditorState
  targetTitle?: string
  control: NodeEditorControl
}) {
  const editing = state.status === 'editing' ? state : undefined
  const locked = editing?.submit.status === 'pending'
  const labels = catalogItems(control.labels)
  const submitError = editing?.submit.status === 'error'
    ? editing.submit.errorCode === 'RESOURCE_CHANGED'
      ? deliveryCopy.feedback.contextChanged
      : deliveryCopy.feedback.saveFailed
    : undefined
  const selectLabel = (value: unknown) => {
    const label = control.available.find((item) => (
      projectDeliveryNodeLabelIdentity(item) === String(value || '')
    ))
    if (label) control.onAdd(label)
  }

  return (
    <ResponsivePanel
      open={Boolean(editing)}
      onOpenChange={(open) => { if (!open) control.onClose() }}
      title={(
        <span className="flex flex-wrap items-baseline gap-2">
          <span>{deliveryCopy.editNodes}</span>
          {targetTitle ? <span className="text-sm font-normal text-muted-foreground">{targetTitle}</span> : null}
        </span>
      )}
      description={deliveryCopy.description}
      closeLabel={deliveryCopy.cancel}
      pending={locked}
      className="max-w-3xl"
      footer={editing ? (
        <Button
          type="submit"
          form="pm-delivery-node-editor-form"
          disabled={locked}
        >
          {locked ? deliveryCopy.saving : deliveryCopy.save}
        </Button>
      ) : undefined}
    >
      {editing ? (
        <form
          id="pm-delivery-node-editor-form"
          className="grid gap-4"
          onSubmit={(event) => {
            event.preventDefault()
            if (!locked) control.onSubmit()
          }}
        >
          <div className="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
            <WriteFormField id="pm-delivery-add-node" label={deliveryCopy.nodeLabel}>
              <Select disabled={locked || control.available.length === 0} onValueChange={selectLabel}>
                <SelectTrigger id="pm-delivery-add-node" className="min-w-52">
                  <SelectValue placeholder={deliveryCopy.addNode} />
                </SelectTrigger>
                <SelectContent>
                  {control.available.map((label) => (
                    <SelectItem
                      key={projectDeliveryNodeLabelIdentity(label)}
                      value={projectDeliveryNodeLabelIdentity(label)}
                    >
                      {label.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </WriteFormField>
            {control.onManageLabels ? (
              <Button type="button" variant="outline" disabled={locked} onClick={control.onManageLabels}>
                {deliveryCopy.manageLabels}
              </Button>
            ) : null}
          </div>

          {control.labels.status === 'loading' ? (
            <LoadingSkeleton rows={2} label={deliveryCopy.labels.loading} />
          ) : null}
          {control.labels.status === 'error' ? (
            <div role="alert" className="flex flex-wrap items-center gap-2 rounded-md border border-destructive/30 bg-destructive/8 px-3 py-2 text-sm text-destructive">
              <span>{deliveryCopy.labels.loadFailed}</span>
              <Button type="button" size="xs" variant="outline" onClick={control.labels.retry}>
                {copy.projectManagement.common.retry}
              </Button>
            </div>
          ) : null}
          {(control.labels.status === 'ready' || control.labels.status === 'empty') && control.labels.staleError ? (
            <div role="alert" className="flex flex-wrap items-center gap-2 rounded-md border border-destructive/30 bg-destructive/8 px-3 py-2 text-sm text-destructive">
              <span>{deliveryCopy.labels.loadFailed}</span>
              <Button type="button" size="xs" variant="outline" onClick={control.labels.staleError.retry}>
                {copy.projectManagement.common.retry}
              </Button>
            </div>
          ) : null}
          {control.labels.status === 'empty' ? (
            <p role="status" className="rounded-md border border-border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
              {deliveryCopy.noCatalogDescription}
            </p>
          ) : null}

          <NodeRows
            draft={editing.draft}
            labels={labels}
            owners={control.owners}
            locked={Boolean(locked)}
            onRemove={control.onRemove}
            onNodeChange={control.onNodeChange}
          />
          {submitError ? (
            <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/8 px-3 py-2 text-sm text-destructive">
              {submitError}
            </p>
          ) : null}
        </form>
      ) : null}
    </ResponsivePanel>
  )
}
