import { useCallback, useMemo, useRef, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'

import { copy } from '@/shared/copy'
import { useWorkspaceStore } from '@/shared/state'
import {
  Button,
  buttonVariants,
  Checkbox,
  ConfirmDialog,
  DataTable,
  AdaptiveFilterBar,
  Input,
  Label,
  Popover,
  PopoverClose,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  UserSelect,
} from '@/shared/ui'
import { useWorkflowMutations, useWorkflowOperationAccess } from '@/features/ai-pm'
import type { PlatformUser, ScrumVersionTaskNew } from '../api'
import { usePlanUnitRequirementsLock } from '../hooks/usePlanUnitRequirementsLock'
import { projectItemTypeLabel } from '../model'
import {
  usePlanUnitColumns,
  usePlanUnitFilters,
  usePlanUnitMembers,
  type PlanUnitColumnKey,
} from '../hooks'
import {
  createProjectItemTableColumns,
  type PlanUnitListActions,
} from './ProjectItemTableColumns'
import { ProjectMembersDialog } from './members/ProjectMembersDialog'
import { useProjectOperationAccess } from '../hooks/useOperationAccess'

export type {
  PlanUnitListActions,
  PlanUnitListItemActions,
} from './ProjectItemTableColumns'

const unitCopy = copy.projectManagement.planUnitList

function ColumnSettings({
  options,
  isVisible,
  onToggle,
}: {
  options: ReadonlyArray<{ key: PlanUnitColumnKey; label: string }>
  isVisible: (key: PlanUnitColumnKey) => boolean
  onToggle: (key: PlanUnitColumnKey) => void
}) {
  return (
    <Popover>
      <PopoverTrigger
        type="button"
        className={buttonVariants({ variant: 'outline', size: 'sm' })}
      >
        {unitCopy.columnSettings}
      </PopoverTrigger>
      <PopoverContent align="end" className="w-72">
        <header className="grid gap-1 border-b border-border p-3">
          <PopoverTitle>{unitCopy.columnSettingsTitle}</PopoverTitle>
          <PopoverDescription>{unitCopy.columnSettingsDescription}</PopoverDescription>
        </header>
        <fieldset className="grid max-h-72 gap-1 overflow-y-auto p-2">
          <legend className="sr-only">{unitCopy.columnSettingsTitle}</legend>
          {options.map((option) => (
            <label
              key={option.key}
              className="flex min-h-8 cursor-pointer items-center gap-2 rounded-md px-2 text-sm hover:bg-muted focus-within:ring-2 focus-within:ring-ring"
            >
              <Checkbox
                checked={isVisible(option.key)}
                onCheckedChange={() => onToggle(option.key)}
              />
              <span>{option.label}</span>
            </label>
          ))}
        </fieldset>
        <footer className="flex justify-end border-t border-border p-2">
          <PopoverClose className={buttonVariants({ size: 'sm' })}>
            {unitCopy.columnSettingsDone}
          </PopoverClose>
        </footer>
      </PopoverContent>
    </Popover>
  )
}

export function PlanUnitList({
  projectId,
  heading,
  rows,
  users,
  actions,
}: {
  projectId: number
  heading?: string
  rows: ScrumVersionTaskNew[]
  users: PlatformUser[]
  actions: PlanUnitListActions
}) {
  const navigate = useNavigate()
  const operationAccess = useProjectOperationAccess()
  const filters = usePlanUnitFilters(rows, users)
  const columnControl = usePlanUnitColumns()
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const workflowAccess = useWorkflowOperationAccess()
  const lockAction = useWorkflowMutations(workspaceId).action

  // 锁版快捷操作，两条路径共用一个确认弹窗：
  // 1. 流程停在需求录入节点的行 → 复合动作（节点完成+锁版原子提交）；
  // 2. 已过录入但仍未锁版的存量行（历史实例/未绑流程）→ 纯锁版，流程不动。
  const [lockTarget, setLockTarget] = useState<{ instanceId: number; nodeId: string; title: string } | null>(null)
  const pureLock = usePlanUnitRequirementsLock()

  const lockableRows = useMemo(() => {
    const map = new Map<number, { instanceId: number; nodeId: string; title: string }>()
    if (!workflowAccess.canActOnInstance()) return map
    for (const row of rows) {
      if (row.requirements_locked || !row.workflow_intake_pending) continue
      if (!row.workflow_instance_id || !row.workflow_intake_node_id) continue
      map.set(row.id, {
        instanceId: row.workflow_instance_id,
        nodeId: row.workflow_intake_node_id,
        title: row.title,
      })
    }
    return map
  }, [rows, workflowAccess])

  const pureLockRows = useMemo(() => {
    const map = new Map<number, { id: number; title: string }>()
    if (!operationAccess.lockPlanUnitRequirements) return map
    for (const row of rows) {
      if (row.requirements_locked || row.workflow_intake_pending) continue
      map.set(row.id, { id: row.id, title: row.title })
    }
    return map
  }, [operationAccess.lockPlanUnitRequirements, rows])

  const listActions = useMemo<PlanUnitListActions>(() => {
    if (lockableRows.size === 0 && pureLockRows.size === 0) return actions
    const items = new Map(actions.items)
    for (const [id, target] of lockableRows) {
      items.set(id, {
        ...(items.get(id) ?? {}),
        lock: () => {
          lockAction.reset()
          pureLock.close()
          setLockTarget(target)
        },
      })
    }
    for (const [id, target] of pureLockRows) {
      items.set(id, {
        ...(items.get(id) ?? {}),
        lock: () => {
          lockAction.reset()
          setLockTarget(null)
          pureLock.open(target)
        },
      })
    }
    return { ...actions, items }
  }, [actions, lockAction, lockableRows, pureLockRows])


  // Participants are version-level facts served per plan unit; load them only
  // while the participants column is visible (the dialog loads its own copy).
  const { membersByUnit, membersPendingUnits, refreshAfterMembersSaved } = usePlanUnitMembers({
    projectId,
    rows,
    enabled: columnControl.isVisible('participants'),
  })

  const [memberUnit, setMemberUnit] = useState<ScrumVersionTaskNew | null>(null)
  // Keep the last edited unit mounted while the panel plays its exit motion.
  const lastMemberUnitRef = useRef<ScrumVersionTaskNew | null>(null)
  if (memberUnit) lastMemberUnitRef.current = memberUnit
  const dialogUnit = memberUnit ?? lastMemberUnitRef.current

  const openMembers = useCallback((row: ScrumVersionTaskNew) => {
    if (!operationAccess.manageParticipants || !(row.can_manage_participants ?? false)) return
    setMemberUnit(row)
  }, [operationAccess.manageParticipants])

  const openDetail = useCallback((row: ScrumVersionTaskNew) => {
    void navigate({
      to: '/workspace/projects/$projectId/items/$projectItemId',
      params: {
        projectId: String(projectId),
        projectItemId: String(row.id),
      },
      search: { tab: 'overview', from_item: undefined },
    })
  }, [navigate, projectId])
  const columns = useMemo(() => createProjectItemTableColumns({
    users,
    actions: listActions,
    isVisible: columnControl.isVisible,
    membersByUnit,
    membersPendingUnits,
    onOpenMembers: operationAccess.manageParticipants ? openMembers : undefined,
  }), [columnControl.isVisible, listActions, membersByUnit, membersPendingUnits, openMembers, users])
  return (
    <section aria-labelledby="pm-plan-unit-list-heading" data-slot="pm-plan-unit-list" className="grid min-w-0 gap-3.5">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <h2 id="pm-plan-unit-list-heading" className="text-base font-semibold tracking-tight">{heading?.trim() || unitCopy.heading}</h2>
          <p className="mt-1 text-xs text-muted-foreground">{unitCopy.description(filters.visibleRows.length, rows.length)}</p>
        </div>
        {actions.create ? (
          <Button type="button" size="sm" onClick={actions.create}>
            {copy.projectManagement.write.actions.createPlanUnit}
          </Button>
        ) : null}
      </div>
      <AdaptiveFilterBar
        controls={[
          { id: 'search', label: unitCopy.searchLabel, placement: 'search', node: <><Label className="sr-only" htmlFor="pm-plan-unit-search">{unitCopy.searchLabel}</Label>
            <Input id="pm-plan-unit-search" value={filters.search.value} placeholder={unitCopy.searchPlaceholder} className="w-full" onChange={(event) => filters.search.onChange(event.target.value)} /></> },
          { id: 'status', label: unitCopy.statusLabel, node: (<Select items={[{ value: 'all', label: unitCopy.allStatuses }, ...(filters.status.options ?? [])]} value={filters.status.value} onValueChange={(value) => filters.status.onChange(String(value))}>
              <SelectTrigger aria-label={unitCopy.statusLabel} className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="all">{unitCopy.allStatuses}</SelectItem>{filters.status.options?.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}</SelectContent>
            </Select>) },
          { id: 'health', label: unitCopy.healthLabel, node: (<Select items={[{ value: 'all', label: unitCopy.allHealth }, ...(filters.health.options ?? [])]} value={filters.health.value} onValueChange={(value) => filters.health.onChange(String(value))}>
              <SelectTrigger aria-label={unitCopy.healthLabel} className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="all">{unitCopy.allHealth}</SelectItem>{filters.health.options?.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}</SelectContent>
            </Select>) },
          { id: 'owner', label: unitCopy.ownerLabel, node: (<UserSelect
              label={unitCopy.ownerLabel}
              value={filters.owner.value}
              options={filters.owner.options ?? []}
              emptyLabel={unitCopy.allOwners}
              className="w-full"
              onValueChange={filters.owner.onChange}
            />) },
        ]}
        endActions={(
          <>
            <Button type="button" variant="outline" size="sm" disabled={!filters.filtered} onClick={filters.reset}>{unitCopy.reset}</Button>
            <ColumnSettings
              options={columnControl.options}
              isVisible={columnControl.isVisible}
              onToggle={columnControl.toggle}
            />
          </>
        )}
      />
      <DataTable
        aria-label={unitCopy.tableLabel}
        className="[&_[data-slot=table]]:min-w-max"
        columns={columns}
        getRowId={(row) => String(row.id)}
        rowAction={{
          label: (row) => unitCopy.openLabel(projectItemTypeLabel(row.type), row.title),
          onActivate: openDetail,
        }}
        state={filters.tableState}
      />
      {dialogUnit ? (
        <ProjectMembersDialog
          key={dialogUnit.id}
          unit={dialogUnit}
          workspaceUsers={users}
          open={memberUnit != null}
          onOpenChange={(open) => { if (!open) setMemberUnit(null) }}
          onSaved={refreshAfterMembersSaved}
        />
      ) : null}
      <ConfirmDialog
        open={lockTarget !== null || pureLock.target !== null}
        onOpenChange={(open) => {
          if (open) return
          setLockTarget(null)
          pureLock.close()
        }}
        onConfirm={() => {
          if (lockTarget) {
            lockAction.mutate(
              {
                id: lockTarget.instanceId,
                action: 'complete',
                node_id: lockTarget.nodeId,
                confirm_requirement_lock: true,
              },
              {
                onSuccess: () => setLockTarget(null),
                // 失败也关闭弹窗：具体原因由全局操作提示（toast）承载。
                onError: () => setLockTarget(null),
              },
            )
            return
          }
          void pureLock.confirm()
        }}
        pending={lockAction.isPending || pureLock.pending}
        cancelLabel={copy.aiPM.workflowSubject.intakeLockConfirmBack}
        confirmLabel={copy.aiPM.workflowSubject.intakeLockConfirmConfirm}
        confirmVariant="default"
        title={pureLock.target
          ? copy.aiPM.workflowSubject.intakeLockOnlyTitle
          : copy.aiPM.workflowSubject.intakeLockConfirmTitle}
        description={pureLock.target
          ? copy.aiPM.workflowSubject.intakeLockOnlyDescription(pureLock.target.title)
          : copy.aiPM.workflowSubject.intakeLockConfirmDescription(lockTarget?.title ?? '')}
      />
    </section>
  )
}
