import { copy } from '@/shared/copy'
import {
  Button,
  DateTimePicker,
  EmptyState,
  LoadingSkeleton,
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
import type { ScrumProjectDeliveryNodeLabel } from '../api'
import {
  availablePlanUnitDeliveryLabels,
  type OwnerDirectoryState,
  type PlanUnitDraft,
  type PlanUnitDeliveryCatalogState,
} from '../model'
import { draftDateValue } from '../model/form-utils'
import { WriteFormField } from './WriteFormField'

const deliveryCopy = copy.projectManagement.delivery
const formCopy = copy.projectManagement.write.planUnitForm

export type PlanUnitDeliveryControl = {
  owners: OwnerDirectoryState
  onAdd: (label: ScrumProjectDeliveryNodeLabel) => void
  onRemove: (identity: string) => void
  onNodeChange: (
    identity: string,
    patch: {
      due_at?: string | null
      actual_completed_at?: string | null
      owner_user_id?: number | null
    },
  ) => void
  onManageLabels: () => void
  onRetry: () => void
}

function NodeRows({
  draft,
  owners,
  locked,
  control,
}: {
  draft: PlanUnitDraft
  owners: OwnerDirectoryState
  locked: boolean
  control: Pick<PlanUnitDeliveryControl, 'onRemove' | 'onNodeChange'>
}) {
  if (draft.deliveryNodes.length === 0) {
    return (
      <EmptyState
        title={deliveryCopy.noSelectedTitle}
        className="border border-dashed border-border"
      />
    )
  }
  return (
    <div className="grid gap-3">
      {draft.deliveryNodes.map((node, index) => {
        const identity = projectDeliveryNodeIdentity(node)
        const idBase = `pm-plan-unit-delivery-${index}`
        return (
          <article key={identity || idBase} className="rounded-lg border border-border bg-card p-3">
            <header className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <h4 className="truncate text-sm font-semibold text-foreground">
                  {projectDeliveryNodeLabel(node, draft.deliveryNodeLabels)}
                </h4>
                {node.label_deleted ? (
                  <p className="mt-1 text-xs text-muted-foreground">{deliveryCopy.deletedLabel}</p>
                ) : null}
              </div>
              <Button
                type="button"
                size="xs"
                variant="destructive"
                disabled={locked}
                onClick={() => control.onRemove(identity)}
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
                  onChange={(value) => control.onNodeChange(identity, {
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
                  onChange={(value) => control.onNodeChange(identity, {
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
                onValueChange={(ownerUserId) => control.onNodeChange(identity, { owner_user_id: ownerUserId ?? null })}
              />
            </div>
          </article>
        )
      })}
    </div>
  )
}

function CatalogStatus({
  state,
  onRetry,
}: {
  state: PlanUnitDeliveryCatalogState
  onRetry: () => void
}) {
  if (state.status === 'loading') {
    return <LoadingSkeleton rows={2} label={deliveryCopy.labels.loading} />
  }
  if (state.status === 'error') {
    return (
      <div role="alert" className="flex flex-wrap items-center gap-2 rounded-md border border-destructive/30 bg-destructive/8 px-3 py-2 text-sm text-destructive">
        <span>{deliveryCopy.labels.loadFailed}</span>
        <Button type="button" size="xs" variant="outline" onClick={onRetry}>
          {copy.projectManagement.common.retry}
        </Button>
      </div>
    )
  }
  return null
}

export function PlanUnitDeliveryFields({
  draft,
  catalog,
  locked: submitLocked,
  control,
}: {
  draft: PlanUnitDraft
  catalog: PlanUnitDeliveryCatalogState
  locked: boolean
  control: PlanUnitDeliveryControl
}) {
  const available = availablePlanUnitDeliveryLabels(draft)
  const locked = submitLocked || catalog.status !== 'ready'
  const selectLabel = (value: unknown) => {
    const label = available.find((item) => (
      projectDeliveryNodeLabelIdentity(item) === String(value || '')
    ))
    if (label) control.onAdd(label)
  }
  return (
    <section className="grid gap-3 rounded-lg border border-border bg-muted/20 p-3" aria-labelledby="pm-plan-unit-delivery-heading">
      <header className="grid gap-1">
        <h3 id="pm-plan-unit-delivery-heading" className="text-sm font-semibold text-foreground">
          {formCopy.deliveryTitle}
        </h3>
        <p className="text-xs text-muted-foreground">{formCopy.deliveryDescription}</p>
      </header>
      <CatalogStatus state={catalog} onRetry={control.onRetry} />
      <div className="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
        <WriteFormField id="pm-plan-unit-add-delivery" label={deliveryCopy.nodeLabel}>
          <Select disabled={locked || available.length === 0} onValueChange={selectLabel}>
            <SelectTrigger id="pm-plan-unit-add-delivery" className="min-w-52">
              <SelectValue placeholder={deliveryCopy.addNode} />
            </SelectTrigger>
            <SelectContent>
              {available.map((label) => (
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
        <Button
          type="button"
          variant="outline"
          disabled={locked}
          onClick={control.onManageLabels}
        >
          {deliveryCopy.manageLabels}
        </Button>
      </div>
      {catalog.status === 'ready' && draft.deliveryNodeLabels.length === 0 ? (
        <p role="status" className="rounded-md border border-border bg-background px-3 py-2 text-sm text-muted-foreground">
          {deliveryCopy.noCatalogDescription}
        </p>
      ) : null}
      <NodeRows draft={draft} owners={control.owners} locked={locked} control={control} />
    </section>
  )
}
