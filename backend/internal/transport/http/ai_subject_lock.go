package httpapi

import (
	"errors"
	"fmt"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type aiRunSubjectSnapshot struct {
	requirement *requirementgorm.Requirement
	planUnit    *projectgorm.PlanUnit
}

func lockAIRunSubjectTx(
	tx *gorm.DB,
	workspaceID uint,
	subjectType string,
	subjectID uint,
	expectedProjectID uint,
) (aiRunSubjectSnapshot, error) {
	switch subjectType {
	case "requirement":
		var requirement requirementgorm.Requirement
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND workspace_id = ?", subjectID, workspaceID).
			First(&requirement).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return aiRunSubjectSnapshot{}, fmt.Errorf("requirement not found: %w", err)
			}
			return aiRunSubjectSnapshot{}, err
		}
		if requirement.ProjectSetID != expectedProjectID {
			return aiRunSubjectSnapshot{}, errAIRunSubjectProjectChanged
		}
		return aiRunSubjectSnapshot{requirement: &requirement}, nil
	case "plan_unit":
		var unit projectgorm.PlanUnit
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND workspace_id = ?", subjectID, workspaceID).
			First(&unit).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return aiRunSubjectSnapshot{}, fmt.Errorf("plan unit not found: %w", err)
			}
			return aiRunSubjectSnapshot{}, err
		}
		if unit.ProjectSetID != expectedProjectID {
			return aiRunSubjectSnapshot{}, errAIRunSubjectProjectChanged
		}
		return aiRunSubjectSnapshot{planUnit: &unit}, nil
	default:
		return aiRunSubjectSnapshot{}, fmt.Errorf("unsupported AI run subject lock type %q", subjectType)
	}
}
