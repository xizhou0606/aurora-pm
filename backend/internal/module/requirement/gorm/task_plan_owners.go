package requirementgorm

import (
	"errors"

	"gorm.io/gorm"
)

func HasTaskPlanOwners(requirement Requirement) bool {
	return positiveOwnerID(requirement.FrontendOwnerUserID) != nil &&
		positiveOwnerID(requirement.BackendOwnerUserID) != nil
}

func TaskPlanLaneAssignee(requirement Requirement, lane string) *uint {
	switch lane {
	case "frontend":
		return positiveOwnerID(requirement.FrontendOwnerUserID)
	case "backend":
		return positiveOwnerID(requirement.BackendOwnerUserID)
	default:
		return nil
	}
}

func TaskPlanReadyTx(
	tx *gorm.DB,
	workspaceID uint,
	subjectType string,
	subjectID uint,
) (bool, error) {
	if subjectType != "requirement" || subjectID == 0 {
		return false, nil
	}
	var requirement Requirement
	if err := tx.Where("id = ? AND workspace_id = ?", subjectID, workspaceID).First(&requirement).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return HasTaskPlanOwners(requirement), nil
}

func positiveOwnerID(value *uint) *uint {
	if value == nil || *value == 0 {
		return nil
	}
	return value
}
