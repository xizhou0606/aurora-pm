package document

import "errors"

var (
	ErrInvalidInput                = errors.New("document input is invalid")
	ErrForbidden                   = errors.New("document operation is forbidden")
	ErrSpaceNotFound               = errors.New("document space not found")
	ErrNodeNotFound                = errors.New("document node not found")
	ErrDocumentNotFound            = errors.New("document not found")
	ErrBindingNotFound             = errors.New("document binding not found")
	ErrParentNotFolder             = errors.New("document parent is not a folder")
	ErrHierarchyCycle              = errors.New("document hierarchy cycle")
	ErrArchiveConfirmationRequired = errors.New("document archive confirmation required")
	ErrArchiveImpactChanged        = errors.New("document archive impact changed")
	ErrRevisionConflict            = errors.New("document revision conflict")
	ErrBindingConfirmationRequired = errors.New("document binding confirmation required")
	ErrRequirementUnavailable      = errors.New("bound requirement is unavailable")
	ErrBindingConflict             = errors.New("requirement primary document is already bound")
	ErrAssetNotFound               = errors.New("document asset not found")
	ErrAssetStorageUnavailable     = errors.New("document asset storage is unavailable")
	ErrCommentsUnavailable         = errors.New("document comments are unavailable")
	ErrCommentThreadNotFound       = errors.New("document comment thread not found")
	ErrCommentNotFound             = errors.New("document comment not found")
	ErrCommentThreadResolved       = errors.New("document comment thread is resolved")
	ErrCommentTransitionConflict   = errors.New("document comment thread transition conflicts with current state")
	ErrCommentMentionInvalid       = errors.New("document comment mention is invalid")
	ErrContextActionsUnavailable   = errors.New("document context actions are unavailable")
	ErrSystemManagedNode           = errors.New("system-managed document node cannot be changed by this operation")
	ErrShortcutTargetInvalid       = errors.New("document shortcut target is invalid")
	ErrOwnerUserInvalid            = errors.New("document owner user is invalid")
	ErrLegacyContentWriteDisabled  = errors.New("legacy document content write is disabled after collaboration cutover")
)

type RevisionConflictError struct{ LatestRevision uint }

func (err RevisionConflictError) Error() string { return ErrRevisionConflict.Error() }
func (err RevisionConflictError) Unwrap() error { return ErrRevisionConflict }

type ArchiveImpactChangedError struct{ Latest ArchiveImpact }

func (err ArchiveImpactChangedError) Error() string { return ErrArchiveImpactChanged.Error() }
func (err ArchiveImpactChangedError) Unwrap() error { return ErrArchiveImpactChanged }
