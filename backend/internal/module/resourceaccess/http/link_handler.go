package resourceaccesshttp

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
)

func (handler *Handler) listLinks(c *gin.Context) {
	principal, resourceID, ok := handler.requireDocumentResourcePolicy(c, documentlibrary.OperationViewLinks)
	if !ok {
		return
	}
	links, err := handler.links.List(c.Request.Context(), principal, resourceID)
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, links)
}

func (handler *Handler) createLink(c *gin.Context) {
	principal, resourceID, ok := handler.requireDocumentResourcePolicy(c, documentlibrary.OperationManageLinks)
	if !ok {
		return
	}
	var request struct {
		Level     string `json:"level"`
		ExpiresIn string `json:"expires_in"`
	}
	if !bindJSON(c, &request) {
		return
	}
	link, err := handler.links.Create(c.Request.Context(), principal, resourceID, resourceaccess.CreateShareLinkInput{
		Level: resourceaccess.Level(request.Level), Duration: resourceaccess.LinkDuration(request.ExpiresIn),
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusCreated, link)
}

func (handler *Handler) revokeLink(c *gin.Context) {
	principal, resourceID, ok := handler.requireDocumentResourcePolicy(c, documentlibrary.OperationManageLinks)
	if !ok {
		return
	}
	if err := handler.links.Revoke(
		c.Request.Context(), principal, resourceID, strings.TrimSpace(c.Param("linkId")),
	); err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"revoked_id": c.Param("linkId")})
}

func (handler *Handler) rotateLink(c *gin.Context) {
	principal, resourceID, ok := handler.requireDocumentResourcePolicy(c, documentlibrary.OperationManageLinks)
	if !ok {
		return
	}
	link, err := handler.links.Rotate(
		c.Request.Context(), principal, resourceID, strings.TrimSpace(c.Param("linkId")),
	)
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, link)
}

func (handler *Handler) exchangeLink(c *gin.Context) {
	principal, ok := requestPrincipal(c)
	if !ok {
		return
	}
	var request struct {
		Secret string `json:"secret"`
	}
	if !bindJSON(c, &request) {
		return
	}
	result, err := handler.links.Exchange(c.Request.Context(), principal, request.Secret)
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, result)
}
