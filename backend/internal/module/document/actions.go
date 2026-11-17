package document

import accesscatalog "github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"

const (
	ActionScopeAll          = accesscatalog.ActionDocumentsScopeAll
	ActionCollaborationView = accesscatalog.ActionDocumentsCollaborationView
	ActionCollaborationEdit = accesscatalog.ActionDocumentsCollaborationEdit
	ActionNodesView         = accesscatalog.ActionDocumentsNodesView
	ActionNodesCreate       = accesscatalog.ActionDocumentsNodesCreate
	ActionNodesCopy         = accesscatalog.ActionDocumentsNodesCopy
	ActionNodesUpdate       = accesscatalog.ActionDocumentsNodesUpdate
	ActionNodesMove         = accesscatalog.ActionDocumentsNodesMove
	ActionNodesArchive      = accesscatalog.ActionDocumentsNodesArchive
	ActionContentView       = accesscatalog.ActionDocumentsContentView
	ActionContentUpdate     = accesscatalog.ActionDocumentsContentUpdate
	ActionContentExport     = accesscatalog.ActionDocumentsContentExport
	ActionBindingsUpdate    = accesscatalog.ActionDocumentsBindingsUpdate
	ActionAssetsView        = accesscatalog.ActionDocumentsAssetsView
	ActionAssetsUpload      = accesscatalog.ActionDocumentsAssetsUpload
	ActionAssetsDownload    = accesscatalog.ActionDocumentsAssetsDownload
	ActionAssetsDelete      = accesscatalog.ActionDocumentsAssetsDelete
	ActionCommentsView      = accesscatalog.ActionDocumentsCommentsView
	ActionCommentsCreate    = accesscatalog.ActionDocumentsCommentsCreate
	ActionCommentsReply     = accesscatalog.ActionDocumentsCommentsReply
	ActionCommentsEditOwn   = accesscatalog.ActionDocumentsCommentsEditOwn
	ActionCommentsDeleteOwn = accesscatalog.ActionDocumentsCommentsDeleteOwn
	ActionCommentsResolve   = accesscatalog.ActionDocumentsCommentsResolve
	ActionCommentsReopen    = accesscatalog.ActionDocumentsCommentsReopen
	ActionShortcutsCreate   = accesscatalog.ActionDocumentsShortcutsCreate
	ActionPreferencesUpdate = accesscatalog.ActionDocumentsPreferencesUpdate
	ActionOwnershipTransfer = accesscatalog.ActionDocumentsOwnershipTransfer
	ActionRequirementView   = accesscatalog.ActionRequirementsPoolView
)
