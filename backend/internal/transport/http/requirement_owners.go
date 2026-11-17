package httpapi

import (
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
)

func ownerListDTO(ids requirementgorm.UintList) requirementgorm.UintList {
	if ids == nil {
		return requirementgorm.UintList{}
	}
	return ids
}

func ownerIDsFromBody(ids *[]uint, scalar optionalValue[uint]) requirementgorm.UintList {
	if ids != nil {
		return requirementgorm.UniqueOwnerIDs(requirementgorm.UintList(*ids))
	}
	return requirementgorm.CoalesceOwnerIDs(nil, scalar.Value)
}

func appendOwnerIDs(userIDs []uint, ids *[]uint, scalar optionalValue[uint]) []uint {
	if ids != nil {
		return append(userIDs, (*ids)...)
	}
	if scalar.Value != nil {
		return append(userIDs, *scalar.Value)
	}
	return userIDs
}

func applyRequirementOwnerFields(requirement *requirementgorm.Requirement, body requirementBody) {
	requirement.ProductOwnerUserIDs = ownerIDsFromBody(body.ProductOwnerUserIDs, body.ProductOwnerUserID)
	requirement.DesignOwnerUserIDs = ownerIDsFromBody(body.DesignOwnerUserIDs, body.DesignOwnerUserID)
	requirement.FrontendOwnerUserIDs = ownerIDsFromBody(body.FrontendOwnerUserIDs, body.FrontendOwnerUserID)
	requirement.BackendOwnerUserIDs = ownerIDsFromBody(body.BackendOwnerUserIDs, body.BackendOwnerUserID)
	requirement.TesterUserIDs = requirementgorm.UniqueOwnerIDs(uintListValue(body.TesterUserIDs))
	requirement.SyncRoleOwners()
}

func applyRequirementOwnerDefaults(requirement *requirementgorm.Requirement, creatorID uint) {
	requirement.SyncRoleOwners()
	if len(requirement.ProductOwnerUserIDs) == 0 && creatorID > 0 {
		requirement.ProductOwnerUserIDs = requirementgorm.UintList{creatorID}
	}
	requirement.SyncRoleOwners()
}

func applyRequirementOwnerUpdates(updates map[string]any, body requirementBody) {
	applyOwnerUpdate(updates, body.ProductOwnerUserIDs, body.ProductOwnerUserID, "product_owner_user_ids", "product_owner_user_id")
	applyOwnerUpdate(updates, body.DesignOwnerUserIDs, body.DesignOwnerUserID, "design_owner_user_ids", "design_owner_user_id")
	applyOwnerUpdate(updates, body.FrontendOwnerUserIDs, body.FrontendOwnerUserID, "frontend_owner_user_ids", "frontend_owner_user_id")
	applyOwnerUpdate(updates, body.BackendOwnerUserIDs, body.BackendOwnerUserID, "backend_owner_user_ids", "backend_owner_user_id")
	if body.TesterUserIDs != nil {
		updates["tester_user_ids"] = requirementgorm.UniqueOwnerIDs(requirementgorm.UintList(*body.TesterUserIDs))
	}
}

func applyOwnerUpdate(
	updates map[string]any,
	ids *[]uint,
	scalar optionalValue[uint],
	idsKey string,
	scalarKey string,
) {
	if ids == nil && !scalar.Set {
		return
	}
	list := ownerIDsFromBody(ids, scalar)
	updates[idsKey] = list
	updates[scalarKey] = requirementgorm.FirstOwnerID(list, nil)
}
