package documentread

import (
	"context"

	accesscatalog "github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
)

type effectiveActionCodes interface {
	EffectiveActionCodes(context.Context, uint) ([]string, error)
}

type workspaceMembershipVerifier interface {
	VerifyWorkspaceMembership(context.Context, uint, uint) error
}

// GlobalAuthorizer is the single type-bounded mapping from
// unified resource operations to existing canonical Actions. It does not
// create cross-type action implications: candidates are selected only after
// resolving the server-owned resource type.
type GlobalAuthorizer struct {
	Actions     effectiveActionCodes
	Memberships workspaceMembershipVerifier
}

func (authorizer GlobalAuthorizer) AuthorizeDocumentResourceOperation(
	ctx context.Context,
	principal resourceaccess.Principal,
	resource documentlibrary.Resource,
	operation documentlibrary.Operation,
) (bool, error) {
	if authorizer.Actions == nil || authorizer.Memberships == nil || principal.UserID == 0 ||
		principal.WorkspaceID == 0 || resource.WorkspaceID != principal.WorkspaceID {
		return false, nil
	}
	if err := authorizer.Memberships.VerifyWorkspaceMembership(ctx, principal.UserID, principal.WorkspaceID); err != nil {
		return false, nil
	}
	requirements := documentResourceOperationActionRequirements(resource.TypeKey, operation)
	if len(requirements) == 0 {
		return false, nil
	}
	codes, err := authorizer.Actions.EffectiveActionCodes(ctx, principal.UserID)
	if err != nil {
		return false, err
	}
	effective := make(map[accesscatalog.ActionCode]struct{}, len(codes))
	for _, code := range codes {
		effective[accesscatalog.ActionCode(code)] = struct{}{}
	}
	for _, alternatives := range requirements {
		matched := false
		for _, candidate := range alternatives {
			if _, allowed := effective[candidate]; allowed {
				matched = true
				break
			}
		}
		if !matched {
			return false, nil
		}
	}
	return true, nil
}

// Every outer slice is required (AND); codes within one inner slice are
// alternatives (OR). Realtime transport is governed by its shared canonical
// Action, intersected with the type's content capability so the transport
// cannot become a read/write bypass. These Actions govern distinct transport
// and content powers; this is not a new-vs-legacy compatibility double gate.
func documentResourceOperationActionRequirements(
	typeKey documentlibrary.TypeKey,
	operation documentlibrary.Operation,
) [][]accesscatalog.ActionCode {
	if operation == documentlibrary.OperationCollaborate {
		switch typeKey {
		case documentlibrary.TypeRichText:
			return [][]accesscatalog.ActionCode{
				{accesscatalog.ActionDocumentsCollaborationView},
				richTextOperationActions(documentlibrary.OperationView),
			}
		case documentlibrary.TypeBitable:
			return [][]accesscatalog.ActionCode{
				{accesscatalog.ActionDocumentsCollaborationView},
				bitableOperationActions(documentlibrary.OperationView),
			}
		default:
			return nil
		}
	}
	if operation == documentlibrary.OperationCellLease {
		if typeKey != documentlibrary.TypeBitable {
			return nil
		}
		return [][]accesscatalog.ActionCode{
			{accesscatalog.ActionDocumentsCollaborationEdit},
			bitableOperationActions(documentlibrary.OperationEdit),
		}
	}
	candidates := documentResourceOperationActions(typeKey, operation)
	if len(candidates) == 0 {
		return nil
	}
	return [][]accesscatalog.ActionCode{candidates}
}

func documentResourceOperationActions(
	typeKey documentlibrary.TypeKey,
	operation documentlibrary.Operation,
) []accesscatalog.ActionCode {
	switch operation {
	case documentlibrary.OperationViewAccess:
		return []accesscatalog.ActionCode{accesscatalog.ActionDocumentsAccessView}
	case documentlibrary.OperationManageAccess:
		return []accesscatalog.ActionCode{accesscatalog.ActionDocumentsAccessManage}
	case documentlibrary.OperationViewLinks:
		return []accesscatalog.ActionCode{accesscatalog.ActionDocumentsShareLinksView}
	case documentlibrary.OperationManageLinks:
		return []accesscatalog.ActionCode{accesscatalog.ActionDocumentsShareLinksManage}
	case documentlibrary.OperationExchangeLink:
		return []accesscatalog.ActionCode{accesscatalog.ActionDocumentsShareLinksExchange}
	}
	if typeKey == documentlibrary.TypeRichText {
		return richTextOperationActions(operation)
	}
	if typeKey == documentlibrary.TypeBitable {
		return bitableOperationActions(operation)
	}
	return nil
}

func richTextOperationActions(operation documentlibrary.Operation) []accesscatalog.ActionCode {
	switch operation {
	case documentlibrary.OperationView:
		return []accesscatalog.ActionCode{accesscatalog.ActionDocumentsContentView, accesscatalog.ActionDocumentsNodesView}
	case documentlibrary.OperationCollaborate:
		return nil
	case documentlibrary.OperationEdit:
		return []accesscatalog.ActionCode{accesscatalog.ActionDocumentsContentUpdate}
	case documentlibrary.OperationMove:
		return []accesscatalog.ActionCode{accesscatalog.ActionDocumentsNodesMove}
	case documentlibrary.OperationCopy:
		return []accesscatalog.ActionCode{accesscatalog.ActionDocumentsNodesCopy}
	case documentlibrary.OperationDelete:
		return []accesscatalog.ActionCode{accesscatalog.ActionDocumentsNodesArchive}
	case documentlibrary.OperationExport:
		return []accesscatalog.ActionCode{accesscatalog.ActionDocumentsContentExport}
	case documentlibrary.OperationAssetRead:
		return []accesscatalog.ActionCode{accesscatalog.ActionDocumentsAssetsView, accesscatalog.ActionDocumentsAssetsDownload}
	case documentlibrary.OperationAssetWrite:
		return []accesscatalog.ActionCode{accesscatalog.ActionDocumentsAssetsUpload, accesscatalog.ActionDocumentsAssetsDelete}
	case documentlibrary.OperationComment:
		return []accesscatalog.ActionCode{
			accesscatalog.ActionDocumentsCommentsCreate, accesscatalog.ActionDocumentsCommentsReply,
			accesscatalog.ActionDocumentsCommentsEditOwn, accesscatalog.ActionDocumentsCommentsDeleteOwn,
			accesscatalog.ActionDocumentsCommentsResolve, accesscatalog.ActionDocumentsCommentsReopen,
		}
	case documentlibrary.OperationStructure:
		return []accesscatalog.ActionCode{accesscatalog.ActionDocumentsBindingsUpdate}
	default:
		return nil
	}
}

func bitableOperationActions(operation documentlibrary.Operation) []accesscatalog.ActionCode {
	switch operation {
	case documentlibrary.OperationView:
		return []accesscatalog.ActionCode{accesscatalog.ActionBitablesResourcesView}
	case documentlibrary.OperationCollaborate:
		return nil
	case documentlibrary.OperationEdit:
		return []accesscatalog.ActionCode{accesscatalog.ActionBitablesRecordsWrite}
	case documentlibrary.OperationCellLease:
		return nil
	case documentlibrary.OperationMove:
		return []accesscatalog.ActionCode{accesscatalog.ActionBitablesResourcesMove}
	case documentlibrary.OperationCopy:
		return []accesscatalog.ActionCode{accesscatalog.ActionBitablesResourcesDuplicate}
	case documentlibrary.OperationDelete:
		return []accesscatalog.ActionCode{accesscatalog.ActionBitablesResourcesDelete}
	case documentlibrary.OperationExport:
		return []accesscatalog.ActionCode{accesscatalog.ActionBitablesTransferExport}
	case documentlibrary.OperationAssetRead:
		return []accesscatalog.ActionCode{accesscatalog.ActionBitablesAssetsDownload}
	case documentlibrary.OperationAssetWrite:
		return []accesscatalog.ActionCode{accesscatalog.ActionBitablesAssetsUpload}
	case documentlibrary.OperationComment:
		return []accesscatalog.ActionCode{accesscatalog.ActionBitablesCommentsCreate}
	case documentlibrary.OperationStructure:
		return []accesscatalog.ActionCode{
			accesscatalog.ActionBitablesFieldsManage, accesscatalog.ActionBitablesViewsManage,
			accesscatalog.ActionBitablesTransferImport,
		}
	default:
		return nil
	}
}
