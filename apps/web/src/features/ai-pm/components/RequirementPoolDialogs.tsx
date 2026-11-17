import { isSafeApiError, PM_API_ERROR_CODES, type PMScopeOptions } from '@/shared/api'
import { copy } from '@/shared/copy'
import {
  Button,
  ConfirmDialog,
  Label,
  ResponsivePanel,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/shared/ui'
import type { RequirementTableRow } from './RequirementPoolTable'
import { ScheduleMutationNotice } from './WorkflowSchedule'

const poolCopy = copy.aiPM.requirementPool
const commonCopy = copy.aiPM.common
const contentCopy = copy.projectManagement.content

export interface MoveDraft {
  requirement: RequirementTableRow
  targetProjectId: number
  targetPlanUnitId: number
}

export interface RequirementPoolDialogsProps {
  move: MoveDraft | null
  removing: RequirementTableRow | null
  closeMove: () => void
  closeDelete: () => void
  submitMove: (confirmSubjectSchedule?: boolean) => void
  submitDelete: () => void
  movePending: boolean
  removePending: boolean
  moveError: unknown
  deleteError: string | undefined
  scope?: PMScopeOptions
  onProjectChange: (projectId: number) => void
  onUnitChange: (unitId: number) => void
}

export function RequirementPoolDialogs({
  move,
  removing,
  closeMove,
  closeDelete,
  submitMove,
  submitDelete,
  movePending,
  removePending,
  moveError: rawMoveError,
  deleteError,
  scope,
  onProjectChange,
  onUnitChange,
}: RequirementPoolDialogsProps) {
  const projectItems = (scope?.projectSets ?? []).map((project) => ({
    value: project.id,
    label: project.label,
  }))
  const unitItems = (scope?.projects ?? [])
    .filter((unit) => unit.parentId === move?.targetProjectId)
    .map((unit) => ({ value: unit.id, label: unit.label }))
  const sameTarget = Boolean(move && move.targetPlanUnitId && move.targetPlanUnitId === (move.requirement.version_task_new_id ?? 0))
  const scheduleError = isSafeApiError(rawMoveError) && (
    rawMoveError.code === PM_API_ERROR_CODES.WORKFLOW_SCHEDULE_CONFIRMATION_REQUIRED ||
    rawMoveError.code === PM_API_ERROR_CODES.WORKFLOW_SCHEDULE_CONFLICT
  )
  const staleMove = isSafeApiError(rawMoveError) && rawMoveError.code === PM_API_ERROR_CODES.REQUIREMENT_STALE
  const moveError = rawMoveError && !scheduleError
    ? staleMove
      ? contentCopy.requirement.moveStale
      : contentCopy.common.moveFailed
    : undefined

  return (
    <>
      {move ? (
      <ResponsivePanel
        open
        onOpenChange={(open) => { if (!open) closeMove() }}
        title={contentCopy.requirement.moveTitle}
        description={contentCopy.requirement.moveDescription(move.requirement.title)}
        closeLabel={commonCopy.cancel}
        pending={movePending}
        className="max-w-lg"
        footer={(
          <Button
            type="button"
            disabled={movePending || staleMove || !move.targetPlanUnitId || sameTarget}
            onClick={() => submitMove()}
          >
            {movePending ? contentCopy.common.saving : contentCopy.common.move}
          </Button>
        )}
      >
        <div className="grid min-w-0 gap-4">
            <div className="grid gap-1.5">
              <Label htmlFor="requirement-pool-move-project">{poolCopy.project}</Label>
              <Select
                items={projectItems}
                value={move.targetProjectId || null}
                disabled={movePending || staleMove}
                onValueChange={(value) => onProjectChange(Number(value) || 0)}
              >
                <SelectTrigger id="requirement-pool-move-project">
                  <SelectValue placeholder={poolCopy.projectPlaceholder} />
                </SelectTrigger>
                <SelectContent>
                  {projectItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="requirement-pool-move-unit">{contentCopy.requirement.moveTarget}</Label>
              {unitItems.length > 0 ? (
                <Select
                  items={unitItems}
                  value={move.targetPlanUnitId || null}
                  disabled={movePending || staleMove || !move.targetProjectId}
                  onValueChange={(value) => onUnitChange(Number(value) || 0)}
                >
                  <SelectTrigger id="requirement-pool-move-unit">
                    <SelectValue placeholder={contentCopy.requirement.moveTarget} />
                  </SelectTrigger>
                  <SelectContent>
                    {unitItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : (
                <p className="m-0 rounded-md border border-dashed border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
                  {contentCopy.requirement.moveTargetUnavailable}
                </p>
              )}
            </div>
            {scheduleError ? (
              <ScheduleMutationNotice
                error={rawMoveError}
                pending={movePending}
                onAdjust={() => {
                  // handled by parent via reset
                }}
                onConfirm={() => submitMove(true)}
                resolveSubjectName={(type, id) => {
                  if (type === 'requirement' && id === move.requirement.id) return move.requirement.title
                  if (type === 'project_set') return projectItems.find((item) => item.value === id)?.label
                  return unitItems.find((item) => item.value === id)?.label
                }}
              />
            ) : null}
            {moveError ? <p role="alert" className="m-0 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{moveError}</p> : null}
        </div>
      </ResponsivePanel>
      ) : null}
      <ConfirmDialog
        open={Boolean(removing)}
        onOpenChange={(open) => { if (!open) closeDelete() }}
        onConfirm={submitDelete}
        title={contentCopy.requirement.deleteTitle}
        description={contentCopy.requirement.deleteDescription(removing?.title ?? '')}
        cancelLabel={commonCopy.cancel}
        confirmLabel={removePending ? contentCopy.common.deleting : contentCopy.requirement.confirmDelete}
        pending={removePending}
        error={deleteError}
      />
    </>
  )
}