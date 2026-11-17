package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/cover"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
)

const requirementCoverMaxBytes = 2 * 1024 * 1024

var errInvalidRequirementCover = errors.New("invalid requirement cover")

func (s *Server) getRequirementCover(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	requirement, _, allowed := s.requirementByID(c, id)
	if !allowed {
		return
	}
	normalizedURL, eligible := requirementcover.EligibleURL(requirement.ProductDocURL, s.cfg)
	store, available := s.requirementCoverStore()
	if !eligible || !available {
		failRequirementCover(c, http.StatusNotFound, apiErrorRequirementCoverUnavailable)
		return
	}
	var cover requirementgorm.RequirementCover
	if err := s.db.Where(
		"workspace_id = ? AND requirement_id = ? AND source_hash = ? AND status = ?",
		requirement.WorkspaceID, requirement.ID, requirementcover.SourceHash(normalizedURL), requirementcover.StatusReady,
	).First(&cover).Error; err != nil {
		failRequirementCover(c, http.StatusNotFound, apiErrorRequirementCoverUnavailable)
		return
	}
	if !validRequirementCoverContentType(cover.ContentType) {
		failRequirementCover(c, http.StatusBadGateway, apiErrorFeishuCoverUnavailable)
		return
	}
	reader, err := store.GetObject(c.Request.Context(), cover.ObjectKey)
	if err != nil {
		failRequirementCover(c, http.StatusBadGateway, apiErrorFeishuCoverUnavailable)
		return
	}
	defer reader.Close()
	content, err := readRequirementCover(reader)
	if err != nil {
		failRequirementCover(c, http.StatusBadGateway, apiErrorFeishuCoverUnavailable)
		return
	}
	c.Header("Cache-Control", "private, max-age=600")
	c.Header("Vary", "Origin, Authorization, X-Workspace-ID")
	c.Header("ETag", `"`+cover.SourceHash+`-`+strconv.FormatUint(uint64(cover.Generation), 10)+`"`)
	c.Header("Content-Disposition", `inline; filename="requirement-cover.jpg"`)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "image/jpeg", content)
}

func failRequirementCover(c *gin.Context, status int, code string) {
	c.Header("Cache-Control", "private, no-store")
	failCode(c, status, code, "requirement cover unavailable")
}

func validRequirementCoverContentType(contentType string) bool {
	return strings.EqualFold(strings.TrimSpace(contentType), "image/jpeg")
}

func (s *Server) requirementCoverStore() (taskObjectStore, bool) {
	store, ok := s.materialStore.(taskObjectStore)
	return store, ok && store != nil && s.materialStoreErr == nil && s.cfg.ObjectStorage.Enabled
}

func readRequirementCover(reader io.Reader) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(reader, requirementCoverMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) == 0 || len(content) > requirementCoverMaxBytes ||
		!validRequirementCoverContentType(http.DetectContentType(content)) {
		return nil, errInvalidRequirementCover
	}
	return content, nil
}
