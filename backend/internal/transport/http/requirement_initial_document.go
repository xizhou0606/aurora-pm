package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"gorm.io/gorm"
)

type initialDocumentError struct{ cause error }

func (e initialDocumentError) Error() string { return e.cause.Error() }
func (e initialDocumentError) Unwrap() error { return e.cause }

// The association and policy change share the creation transaction. The workflow
// cannot enqueue an AI run before the selected document is part of its inputs.
func (s *Server) attachInitialRequirementDocument(tx *gorm.DB, c *gin.Context, row *requirementgorm.Requirement, selection *initialRequirementDocument) error {
	if selection == nil {
		return nil
	}
	if selection.ResourceID == "" || selection.ExpectedPolicyRevision == 0 || selection.OperationID == "" || row.ProductDocURL != "" {
		return initialDocumentError{requirement.ErrResourceLinkInvalid}
	}
	principal, ok := currentPrincipal(c)
	if !ok || s.requirementResourceLinks == nil {
		return initialDocumentError{resourceaccess.ErrForbidden}
	}
	service := s.requirementResourceLinks(tx)
	if service == nil {
		return initialDocumentError{resourceaccess.ErrForbidden}
	}
	_, err := service.Apply(c.Request.Context(), requirement.ResourceLinkCommand{
		WorkspaceID: row.WorkspaceID, UserID: principal.UserID, RequirementID: row.ID,
		ResourceID: selection.ResourceID, ExpectedVersion: row.Version,
		ExpectedPolicyRevision: selection.ExpectedPolicyRevision, OperationID: selection.OperationID,
	})
	if err != nil {
		return initialDocumentError{err}
	}
	return tx.Where("workspace_id = ? AND id = ?", row.WorkspaceID, row.ID).First(row).Error
}

func respondInitialDocumentError(c *gin.Context, err error) bool {
	var association initialDocumentError
	if !errors.As(err, &association) {
		return false
	}
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, requirement.ErrResourceLinkInvalid), errors.Is(err, resourceaccess.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, requirement.ErrResourceLinkConflict), errors.Is(err, resourceaccess.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, requirement.ErrResourceLinkNotFound), errors.Is(err, documentlibrary.ErrNotFound), errors.Is(err, resourceaccess.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, resourceaccess.ErrForbidden):
		status = http.StatusForbidden
	}
	fail(c, status, "initial document association failed")
	return true
}
