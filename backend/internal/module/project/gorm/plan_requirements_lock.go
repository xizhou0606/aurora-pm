package projectgorm

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrPlanUnitRequirementsLocked is returned when requirement intake is locked
// for a plan unit and a caller tried to create a new requirement there.
var ErrPlanUnitRequirementsLocked = errors.New("plan unit requirements are locked")

// ErrPlanUnitMutationInProgress mirrors the child-scope saga guard: the plan
// unit row is mid-mutation and cannot accept requirement intake decisions.
var ErrPlanUnitMutationInProgress = errors.New("plan unit mutation is in progress")

// LockPlanUnitForRequirementsTx locks the plan unit row FOR UPDATE so that
// requirement creation and requirement-intake locking serialize on the same
// row. Callers must read RequirementsLockedAt only from the returned row.
func LockPlanUnitForRequirementsTx(tx *gorm.DB, workspaceID, planUnitID uint) (PlanUnit, error) {
	var unit PlanUnit
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("workspace_id = ? AND id = ?", workspaceID, planUnitID).First(&unit).Error
	return unit, err
}

// EnsurePlanUnitAcceptsNewRequirements verifies inside the caller's
// transaction that the locked plan unit still accepts brand-new requirements.
// Moving, rescheduling, or editing existing requirements must NOT call this.
func EnsurePlanUnitAcceptsNewRequirementsTx(tx *gorm.DB, workspaceID, planUnitID uint) error {
	unit, err := LockPlanUnitForRequirementsTx(tx, workspaceID, planUnitID)
	if err != nil {
		return err
	}
	return planUnitRequirementIntakeError(unit)
}

func planUnitRequirementIntakeError(unit PlanUnit) error {
	if unit.RequirementsLockedAt != nil {
		return ErrPlanUnitRequirementsLocked
	}
	return nil
}

// BumpPlanUnitDeliveryScopeRevisionTx increments the plan unit's delivery
// scope revision inside the caller's transaction. Call it whenever the
// current requirement set changes (create / move / schedule / delete) so the
// revision stays a monotonic fact for traceability.
func BumpPlanUnitDeliveryScopeRevisionTx(tx *gorm.DB, workspaceID, planUnitID uint) error {
	return tx.Model(&PlanUnit{}).
		Where("workspace_id = ? AND id = ?", workspaceID, planUnitID).
		Update("delivery_scope_revision", gorm.Expr("delivery_scope_revision + 1")).Error
}

// MarkPlanUnitRequirementsLockedTx writes the first lock record. Repeated
// confirmations are no-ops: the original locker and timestamp stay intact so
// workflow rollbacks and re-completions cannot rewrite the lock history.
func MarkPlanUnitRequirementsLockedTx(tx *gorm.DB, workspaceID, planUnitID, lockedByUserID uint, lockedAt time.Time) (bool, error) {
	unit, err := LockPlanUnitForRequirementsTx(tx, workspaceID, planUnitID)
	if err != nil {
		return false, err
	}
	if unit.MutationState != "" {
		return false, ErrPlanUnitMutationInProgress
	}
	if unit.RequirementsLockedAt != nil {
		return false, nil
	}
	updated := tx.Model(&PlanUnit{}).
		Where("workspace_id = ? AND id = ? AND requirements_locked_at IS NULL", workspaceID, planUnitID).
		Updates(map[string]any{"requirements_locked_at": lockedAt, "requirements_locked_by_user_id": lockedByUserID})
	if updated.Error != nil {
		return false, updated.Error
	}
	if updated.RowsAffected != 1 {
		return false, ErrPlanUnitMutationInProgress
	}
	return true, nil
}
