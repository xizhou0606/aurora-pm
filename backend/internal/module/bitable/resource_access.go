package bitable

import (
	"context"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
)

var bitableActionOperations = []struct {
	operation documentlibrary.Operation
	actions   []accesscatalog.ActionCode
}{
	{documentlibrary.OperationView, []accesscatalog.ActionCode{
		accesscatalog.ActionBitablesResourcesView, accesscatalog.ActionBitablesResourcesPin,
		accesscatalog.ActionBitablesResourcesFavorite, accesscatalog.ActionBitablesResourcesMove,
	}},
	{documentlibrary.OperationEdit, []accesscatalog.ActionCode{
		accesscatalog.ActionBitablesRecordsWrite, accesscatalog.ActionBitablesTransferImport,
	}},
	{documentlibrary.OperationStructure, []accesscatalog.ActionCode{
		accesscatalog.ActionBitablesFieldsManage, accesscatalog.ActionBitablesViewsManage,
		accesscatalog.ActionBitablesResourcesRename,
	}},
	{documentlibrary.OperationComment, []accesscatalog.ActionCode{accesscatalog.ActionBitablesCommentsCreate}},
	{documentlibrary.OperationAssetRead, []accesscatalog.ActionCode{accesscatalog.ActionBitablesAssetsDownload}},
	{documentlibrary.OperationAssetWrite, []accesscatalog.ActionCode{accesscatalog.ActionBitablesAssetsUpload}},
	{documentlibrary.OperationExport, []accesscatalog.ActionCode{accesscatalog.ActionBitablesTransferExport}},
	{documentlibrary.OperationCopy, []accesscatalog.ActionCode{
		accesscatalog.ActionBitablesResourcesDuplicate, accesscatalog.ActionBitablesShortcutsCreate,
	}},
	{documentlibrary.OperationDelete, []accesscatalog.ActionCode{accesscatalog.ActionBitablesResourcesDelete}},
	{documentlibrary.OperationManageAccess, []accesscatalog.ActionCode{accesscatalog.ActionBitablesOwnershipTransfer}},
}

func (s *Service) requireResource(
	ctx context.Context, p Principal, resourceID string, operation documentlibrary.Operation,
) (ResourceBinding, error) {
	if s.access == nil {
		return ResourceBinding{}, ErrUnavailable
	}
	normalizedID := strings.TrimSpace(resourceID)
	if p.UserID == 0 || p.WorkspaceID == 0 {
		return ResourceBinding{}, ErrForbidden
	}
	if normalizedID == "" || normalizedID != resourceID || len(resourceID) > 200 {
		return ResourceBinding{}, ErrInvalidInput
	}
	binding, err := s.access.RequireBitableResource(
		ctx, p, documentlibrary.ResourceID(resourceID), operation,
	)
	if err != nil {
		return ResourceBinding{}, err
	}
	if binding.ResourceID != documentlibrary.ResourceID(resourceID) {
		return ResourceBinding{}, ErrUnavailable
	}
	if err := validateResourceBinding(binding, binding.NativeResourceID); err != nil {
		return ResourceBinding{}, err
	}
	return binding, nil
}

func validateResourceBinding(binding ResourceBinding, nativeResourceID string) error {
	if strings.TrimSpace(string(binding.ResourceID)) == "" || binding.NativeResourceID != nativeResourceID ||
		binding.OwnerUserID == 0 {
		return ErrUnavailable
	}
	parsed, err := uuid.Parse(binding.NativeResourceID)
	if err != nil || parsed == uuid.Nil || parsed.String() != binding.NativeResourceID {
		return ErrUnavailable
	}
	return nil
}

func projectResource(resource Resource, binding ResourceBinding) Resource {
	resource.NativeResourceID = binding.NativeResourceID
	resource.ResourceID = string(binding.ResourceID)
	resource.ID = string(binding.ResourceID)
	resource.OwnerUserID = binding.OwnerUserID
	if resource.AllowedActions == nil {
		resource.AllowedActions = []string{}
	}
	return resource
}

func operationForLegacyBitableAction(action accesscatalog.ActionCode) (documentlibrary.Operation, bool) {
	for _, candidate := range bitableActionOperations {
		for _, expected := range candidate.actions {
			if action == expected {
				return candidate.operation, true
			}
		}
	}
	return "", false
}

func allBitableOperations() []documentlibrary.Operation {
	result := make([]documentlibrary.Operation, 0, len(bitableActionOperations))
	for _, candidate := range bitableActionOperations {
		result = append(result, candidate.operation)
	}
	return result
}

func allowedActionsFromDecisions(decisions map[documentlibrary.Operation]bool) []string {
	result := []string{}
	for _, candidate := range bitableActionOperations {
		if decisions[candidate.operation] {
			for _, action := range candidate.actions {
				result = append(result, string(action))
			}
		}
	}
	sort.Strings(result)
	return result
}
