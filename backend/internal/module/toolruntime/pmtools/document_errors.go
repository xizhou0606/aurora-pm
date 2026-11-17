package pmtools

import (
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

func documentToolError(err error) error {
	code := ""
	switch {
	case errors.Is(err, document.ErrReplacementResultUnknown):
		code = "DOCUMENT_REPLACE_RESULT_UNKNOWN"
	case errors.Is(err, document.ErrEditingInProgress):
		code = "DOCUMENT_EDITING_IN_PROGRESS"
	case errors.Is(err, document.ErrEditingStateUnavailable):
		code = "DOCUMENT_EDITING_STATE_UNAVAILABLE"
	case errors.Is(err, document.ErrReplaceInProgress):
		code = "DOCUMENT_REPLACE_IN_PROGRESS"
	case errors.Is(err, document.ErrForbidden):
		code = "DOCUMENT_FORBIDDEN"
	case errors.Is(err, document.ErrDocumentNotFound), errors.Is(err, document.ErrNodeNotFound), errors.Is(err, document.ErrSpaceNotFound), errors.Is(err, document.ErrBindingNotFound), errors.Is(err, document.ErrAssetNotFound), errors.Is(err, document.ErrCommentNotFound), errors.Is(err, document.ErrCommentThreadNotFound):
		code = "DOCUMENT_NOT_FOUND"
	case errors.Is(err, document.ErrDirectoryConflict):
		code = "DOCUMENT_DIRECTORY_CONFLICT"
	case errors.Is(err, document.ErrRevisionConflict):
		code = "DOCUMENT_VERSION_CONFLICT"
	case errors.Is(err, document.ErrArchiveImpactChanged), errors.Is(err, document.ErrArchiveConfirmationRequired):
		code = "DOCUMENT_ARCHIVE_IMPACT_CHANGED"
	case errors.Is(err, document.ErrBindingConflict), errors.Is(err, document.ErrBindingConfirmationRequired):
		code = "DOCUMENT_BINDING_CONFLICT"
	case errors.Is(err, document.ErrCommentThreadResolved), errors.Is(err, document.ErrCommentTransitionConflict):
		code = "DOCUMENT_COMMENT_CONFLICT"
	case errors.Is(err, document.ErrAssetStorageUnavailable), errors.Is(err, document.ErrCommentsUnavailable), errors.Is(err, document.ErrContextActionsUnavailable):
		code = "DOCUMENT_SERVICE_UNAVAILABLE"
	case errors.Is(err, document.ErrLegacyContentWriteDisabled):
		code = "DOCUMENT_COLLABORATION_CUTOVER"
	case errors.Is(err, document.ErrInvalidInput), errors.Is(err, document.ErrParentNotFolder), errors.Is(err, document.ErrHierarchyCycle), errors.Is(err, document.ErrRequirementUnavailable), errors.Is(err, document.ErrOwnerUserInvalid), errors.Is(err, document.ErrShortcutTargetInvalid), errors.Is(err, document.ErrCommentMentionInvalid), errors.Is(err, document.ErrSystemManagedNode):
		code = "DOCUMENT_INPUT_INVALID"
	}
	if failure := toolruntime.LookupBusinessError(code); failure != nil {
		return failure
	}
	return err
}
