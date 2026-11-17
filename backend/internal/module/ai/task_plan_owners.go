package ai

import (
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task/plan"
)

var ErrTaskPlanOwnersRequired = errors.New("frontend and backend owners must be assigned before task plan AI can run")

// RequireTaskPlanOwners enforces the task-plan capability's lane-owner
// precondition after the requirement module has supplied its owner state.
func RequireTaskPlanOwners(capability string, hasOwners bool) error {
	if capability != taskplan.Capability || hasOwners {
		return nil
	}
	return ErrTaskPlanOwnersRequired
}
