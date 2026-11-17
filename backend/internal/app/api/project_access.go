package api

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"gorm.io/gorm"
)

// newProjectAccessService constructs the access service wired to the GORM
// plan-unit access repository and identity AdminRoleChecker. It returns the
// concrete service because callers need all access facets: AccessChecker and
// PlanUnitAccessChecker point gates plus AccessFilterBuilder SQL scope
// filters.
func newProjectAccessService(db *gorm.DB) (*project.AccessService, error) {
	adminRoleChecker := projectgorm.NewAdminRoleChecker(db)
	unitAccessRepository := projectgorm.NewPlanUnitAccessRepository(db)

	// AdminRoleChecker implements AdminRoleChecker and UserActiveChecker.
	// PlanUnitAccessRepository implements SetCreatorReader,
	// PlanUnitAccessReader, and AccessFilterBuilder.
	return project.NewAccessService(
		adminRoleChecker,     // AdminRoleChecker
		adminRoleChecker,     // UserActiveChecker
		unitAccessRepository, // SetCreatorReader
		unitAccessRepository, // PlanUnitAccessReader
		unitAccessRepository, // AccessFilterBuilder
	)
}
