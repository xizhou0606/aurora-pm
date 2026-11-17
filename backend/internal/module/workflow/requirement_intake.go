package workflow

import (
	"context"
	"strings"
	"time"
)

// RequirementIntakeLockStore persists the plan-unit requirement-intake lock
// inside the caller's transaction. The application composition root implements
// it over the project module store; the workflow module never touches project
// tables directly. Requirement creation serializes on the same plan-unit row,
// so a lock written here is immediately visible to any concurrent creation.
type RequirementIntakeLockStore interface {
	// RequirementIntakeLocked reports whether requirement intake is already
	// locked for the plan unit.
	RequirementIntakeLocked(ctx context.Context, workspaceID, planUnitID uint) (bool, error)
	// MarkRequirementIntakeLocked writes the first lock record. It returns
	// false when the plan unit was locked before, leaving the original locker
	// and timestamp intact (rollbacks and repeated completions never rewrite
	// lock history).
	MarkRequirementIntakeLocked(ctx context.Context, workspaceID, planUnitID, lockedByUserID uint, lockedAt time.Time) (bool, error)
}

// RequirementIntakeEffect is the plan-unit lock audit written next to the node
// action audit, in the same transaction.
type RequirementIntakeEffect struct {
	WorkspaceID  uint
	PrincipalID  uint
	ProjectSetID uint
	PlanUnitID   uint
	InstanceID   uint
	NodeID       string
	NodeName     string
	LockedAt     time.Time
}

type RequirementIntakeEffectWriter interface {
	RecordRequirementIntakeLocked(context.Context, RequirementIntakeEffect) error
}

// MutationNodeRequiresRequirementIntakeLock reports whether completing this
// node is the project requirement-intake lock confirmation. Tri-state per the
// intake-lock design (§4.2): explicit requirement_intake_lock wins (true or
// false); when unset, historical name compatibility applies — plan-unit
// human task/form nodes that collect requirement input (requirement_input or
// a 需求录入/需求收集 name) default to requiring the confirmation.
func MutationNodeRequiresRequirementIntakeLock(subjectType string, node DefinitionNode, action string) bool {
	if subjectType != "plan_unit" || action != "complete" {
		return false
	}
	if node.Type != "human_task" && node.Type != "form" {
		return false
	}
	if raw, exists := node.Config["requirement_intake_lock"]; exists {
		required, _ := raw.(bool)
		return required
	}
	if requirementInput, _ := node.Config["requirement_input"].(bool); requirementInput {
		return true
	}
	return strings.Contains(node.Name, "需求录入") || strings.Contains(node.Name, "需求收集")
}
