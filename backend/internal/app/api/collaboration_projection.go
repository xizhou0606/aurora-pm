package api

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	collaborationhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/http"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
)

type collaborationProjectionValidator struct {
	documents collaborationAssetReferenceValidator
	resources documentlibrary.Resolver
	policies  *resourceaccess.PolicyService
}

type collaborationAssetReferenceValidator interface {
	ValidateCollaborationAssetReferences(context.Context, document.Principal, uint, []uint) error
}

type collaborationProjectionReferences struct {
	AssetIDs           []string
	BitableResourceIDs []string
}

func (validator collaborationProjectionValidator) ValidateCollaborationProjection(
	ctx context.Context,
	principal collaboration.Principal,
	room collaboration.Room,
	projection json.RawMessage,
	reported collaborationhttp.AuthorityReferenceSet,
) error {
	extracted, _, err := validator.validateProjectionShape(ctx, principal, room, projection)
	if err != nil {
		return err
	}
	if !equalReferenceIDs(extracted.AssetIDs, sortedReferenceIDs(reported.AssetIDs)) ||
		!equalReferenceIDs(extracted.BitableResourceIDs, sortedReferenceIDs(reported.BitableResourceIDs)) {
		return collaboration.ErrInvalidInput
	}
	return validator.ValidateDocumentProjection(ctx, principal, room, projection)
}

func (validator collaborationProjectionValidator) ValidateDocumentProjection(
	ctx context.Context,
	principal collaboration.Principal,
	room collaboration.Room,
	projection []byte,
) error {
	references, documentID, err := validator.validateProjectionShape(ctx, principal, room, projection)
	if err != nil {
		return err
	}
	assetIDs := make([]uint, 0, len(references.AssetIDs))
	for _, value := range references.AssetIDs {
		assetID, parseErr := strconv.ParseUint(value, 10, 64)
		if parseErr != nil || assetID == 0 || uint64(uint(assetID)) != assetID {
			return collaboration.ErrInvalidInput
		}
		assetIDs = append(assetIDs, uint(assetID))
	}
	documentPrincipal := document.Principal{
		WorkspaceID: room.WorkspaceID, UserID: principal.UserID, LinkAccessSecret: principal.LinkAccessSecret,
	}
	if err = validator.documents.ValidateCollaborationAssetReferences(ctx, documentPrincipal, documentID, assetIDs); err != nil {
		return mapDocumentCollaborationValidationError(err)
	}
	for _, resourceID := range references.BitableResourceIDs {
		resource, binding, resolveErr := validator.resources.RequireTypeOperation(
			ctx, room.WorkspaceID, documentlibrary.ResourceID(resourceID), documentlibrary.OperationView,
		)
		if resolveErr != nil || resource.TypeKey != documentlibrary.TypeBitable || binding.TypeKey != documentlibrary.TypeBitable {
			return collaboration.ErrForbidden
		}
		decision, evaluateErr := validator.policies.Evaluate(ctx, collaborationResourcePrincipal(principal), resourceaccess.EvaluationRequest{
			ResourceID: resource.ID, Operation: documentlibrary.OperationView,
		})
		if evaluateErr != nil || !decision.Allowed || decision.PolicyRevision == 0 {
			return collaboration.ErrForbidden
		}
	}
	return nil
}

func (validator collaborationProjectionValidator) validateProjectionShape(
	ctx context.Context,
	principal collaboration.Principal,
	room collaboration.Room,
	projection []byte,
) (collaborationProjectionReferences, uint, error) {
	if validator.documents == nil || validator.resources == nil || validator.policies == nil ||
		principal.WorkspaceID == 0 || principal.UserID == 0 || principal.WorkspaceID != room.WorkspaceID {
		return collaborationProjectionReferences{}, 0, collaboration.ErrForbidden
	}
	resource, binding, err := validator.resources.ResolveResource(
		ctx, room.WorkspaceID, documentlibrary.ResourceID(room.ResourceID),
	)
	if err != nil || resource.TypeKey != documentlibrary.TypeRichText || binding.TypeKey != documentlibrary.TypeRichText ||
		resource.Lifecycle != "active" || resource.ArchivedAt != nil {
		return collaborationProjectionReferences{}, 0, collaboration.ErrForbidden
	}
	documentID, err := strconv.ParseUint(binding.PayloadRef, 10, 64)
	if err != nil || documentID == 0 || uint64(uint(documentID)) != documentID {
		return collaborationProjectionReferences{}, 0, collaboration.ErrDataCorrupt
	}
	references, err := extractCollaborationProjectionReferences(projection)
	if err != nil {
		return collaborationProjectionReferences{}, 0, err
	}
	return references, uint(documentID), nil
}

func extractCollaborationProjectionReferences(projection []byte) (collaborationProjectionReferences, error) {
	normalized, _, _, err := document.NormalizeContentForSchema(json.RawMessage(projection), document.SchemaVersionV3)
	if err != nil {
		return collaborationProjectionReferences{}, collaboration.ErrInvalidInput
	}
	var root map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(normalized)))
	decoder.UseNumber()
	if err = decoder.Decode(&root); err != nil {
		return collaborationProjectionReferences{}, collaboration.ErrInvalidInput
	}
	references := collaborationProjectionReferences{}
	if err = collectCollaborationProjectionReferences(root, &references); err != nil {
		return collaborationProjectionReferences{}, err
	}
	references.AssetIDs = canonicalReferenceIDs(references.AssetIDs)
	references.BitableResourceIDs = canonicalReferenceIDs(references.BitableResourceIDs)
	return references, nil
}

func collectCollaborationProjectionReferences(node map[string]any, references *collaborationProjectionReferences) error {
	nodeType, _ := node["type"].(string)
	attrs, _ := node["attrs"].(map[string]any)
	switch nodeType {
	case "image":
		if rawAssetID, exists := attrs["assetId"]; exists && rawAssetID != nil {
			assetID, ok := rawAssetID.(json.Number)
			if !ok {
				return collaboration.ErrInvalidInput
			}
			value := assetID.String()
			parsed, err := strconv.ParseUint(value, 10, 64)
			if err != nil || parsed == 0 || strconv.FormatUint(parsed, 10) != value {
				return collaboration.ErrInvalidInput
			}
			references.AssetIDs = append(references.AssetIDs, value)
		}
	case "bitable":
		resourceID, ok := attrs["resourceId"].(string)
		if !ok || !collaborationResourceIDPattern.MatchString(resourceID) {
			return collaboration.ErrInvalidInput
		}
		references.BitableResourceIDs = append(references.BitableResourceIDs, resourceID)
	}
	children, _ := node["content"].([]any)
	for _, rawChild := range children {
		child, ok := rawChild.(map[string]any)
		if !ok {
			return collaboration.ErrInvalidInput
		}
		if err := collectCollaborationProjectionReferences(child, references); err != nil {
			return err
		}
	}
	return nil
}

func canonicalReferenceIDs(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	result = slicesCompact(result)
	return result
}

func sortedReferenceIDs(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func equalReferenceIDs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func slicesCompact(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func mapDocumentCollaborationValidationError(err error) error {
	switch {
	case errors.Is(err, document.ErrInvalidInput):
		return collaboration.ErrInvalidInput
	case errors.Is(err, document.ErrDocumentNotFound), errors.Is(err, document.ErrAssetNotFound),
		errors.Is(err, document.ErrForbidden):
		return collaboration.ErrForbidden
	default:
		return collaboration.ErrDataCorrupt
	}
}

var _ collaboration.DocumentProjectionValidator = collaborationProjectionValidator{}
var _ collaborationAuthorityProjectionValidator = collaborationProjectionValidator{}
