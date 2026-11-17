// Package designreviewhttp adapts authenticated PM HTTP requests to the
// design-review domain service.
package designreviewhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type Handler struct{ service *designreview.Service }

func NewHandler(service *designreview.Service) *Handler { return &Handler{service: service} }

func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	reviews := group.Group("/design-reviews")
	reviews.GET("/capabilities", handler.capabilities)
	reviews.GET("/requirements", handler.requirements)
	reviews.GET("", handler.listReviews)
	reviews.POST("", handler.createReview)
	reviews.POST("/figma-preview", handler.figmaPreview)
	reviews.POST("/dom-normalize", handler.domNormalize)
	reviews.GET("/:id", handler.getReview)
	reviews.GET("/:id/rounds", handler.rounds)
	reviews.POST("/:id/rechecks", handler.createRecheck)
	reviews.POST("/:id/assets", handler.uploadAsset)
	reviews.GET("/:id/assets/:asset_id", handler.downloadAsset)
	reviews.GET("/:id/export", handler.exportReview)
	reviews.POST("/:id/analysis", handler.queueAnalysis)
	reviews.POST("/:id/analysis/retry", handler.retryAnalysis)
	reviews.POST("/:id/analysis/abandon", handler.abandonAnalysis)
	reviews.DELETE("/:id", handler.deleteReview)

	differences := group.Group("/design-review-differences")
	differences.POST("", handler.createDifference)
	differences.PATCH("/recheck-matches/:id", handler.resolveRecheckMatch)
	differences.PATCH("/recheck-verifications/:id", handler.overrideRecheckVerification)
	differences.PATCH("/:id", handler.updateDifference)
	differences.DELETE("/:id", handler.deleteDifference)
	differences.POST("/:id/verify", handler.verifyDifference)
	differences.POST("/:id/resolve", handler.resolveDifference)
	differences.POST("/:id/reopen", handler.reopenDifference)

	deep := group.Group("/design-review-deep-reviews")
	deep.POST("", handler.createDeepReview)
	deep.POST("/:id/retry", handler.retryDeepReview)
	deep.POST("/:id/cancel", handler.cancelDeepReview)

	collectors := group.Group("/design-review-mobile-collectors")
	collectors.GET("", handler.listCollectors)
	collectors.POST("/pairing-codes", handler.createPairingCode)
	collectors.DELETE("/:id", handler.revokeCollector)

	captures := group.Group("/design-review-mobile-captures")
	captures.GET("", handler.listCaptures)
	captures.POST("", handler.createCapture)
	captures.GET("/:id/asset", handler.downloadCaptureAsset)
	captures.POST("/:id/consume", handler.consumeCapture)
	captures.DELETE("/:id", handler.deleteCapture)
}

func (handler *Handler) capabilities(c *gin.Context) {
	_, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	respond(c, http.StatusOK, handler.service.Capabilities(c.Request.Context(), userID))
}

func (handler *Handler) requirements(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	items, err := handler.service.ListRequirements(c.Request.Context(), workspaceID, userID)
	respondResult(c, items, err)
}

func (handler *Handler) listReviews(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	filter := designreview.ReviewListFilter{Keyword: c.Query("keyword"), ProjectSetID: queryUint(c, "project_set_id"), PlanUnitID: queryUint(c, "plan_unit_id"), RequirementID: queryUint(c, "requirement_id"), Status: c.Query("status"), Conclusion: c.Query("conclusion"), Page: queryInt(c, "page", 1), PageSize: queryInt(c, "page_size", 20)}
	page, err := handler.service.ListReviews(c.Request.Context(), workspaceID, userID, filter)
	respondResult(c, page, err)
}

func (handler *Handler) getReview(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	item, err := handler.service.GetReview(c.Request.Context(), workspaceID, userID, c.Param("id"))
	respondResult(c, item, err)
}

func (handler *Handler) createReview(c *gin.Context) {
	workspaceID, userID, name, ok := requestIdentity(c)
	if !ok {
		return
	}
	var input designreview.CreateReviewInput
	if decodeStrictJSON(c, &input) != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	item, err := handler.service.CreateReview(c.Request.Context(), workspaceID, userID, name, input)
	respondResultStatus(c, http.StatusCreated, item, err)
}

func (handler *Handler) createRecheck(c *gin.Context) {
	workspaceID, userID, name, ok := requestIdentity(c)
	if !ok {
		return
	}
	var input designreview.CreateReviewInput
	if decodeStrictJSON(c, &input) != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	input.SourceReviewID = stringPointer(c.Param("id"))
	item, err := handler.service.CreateReview(c.Request.Context(), workspaceID, userID, name, input)
	respondResultStatus(c, http.StatusCreated, item, err)
}

func (handler *Handler) rounds(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	item, err := handler.service.GetReview(c.Request.Context(), workspaceID, userID, c.Param("id"))
	if err != nil {
		failDomain(c, err)
		return
	}
	respond(c, http.StatusOK, item.Rounds)
}

func (handler *Handler) figmaPreview(c *gin.Context) {
	_, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	var input struct {
		URL string `json:"figma_url"`
	}
	if decodeStrictJSON(c, &input) != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	preview, err := handler.service.PreviewFigma(c.Request.Context(), userID, input.URL)
	respondResult(c, preview, err)
}

func (handler *Handler) domNormalize(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	var input struct {
		FigmaURL  string          `json:"figma_url"`
		Inventory json.RawMessage `json:"inventory"`
	}
	if decodeStrictJSON(c, &input) != nil || len(input.Inventory) == 0 {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	plan, err := handler.service.NormalizeDOM(c.Request.Context(), workspaceID, userID, input.FigmaURL, input.Inventory)
	respondResult(c, plan, err)
}

func (handler *Handler) uploadAsset(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	opened, err := file.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	defer opened.Close()
	asset, err := handler.service.UploadAsset(c.Request.Context(), workspaceID, userID, c.Param("id"), c.PostForm("kind"), file.Header.Get("Content-Type"), opened)
	respondResultStatus(c, http.StatusCreated, asset, err)
}

func (handler *Handler) downloadAsset(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	asset, reader, err := handler.service.Asset(c.Request.Context(), workspaceID, userID, c.Param("id"), c.Param("asset_id"))
	if err != nil {
		failDomain(c, err)
		return
	}
	defer reader.Close()
	c.Header("Content-Type", asset.ContentType)
	c.Header("Content-Length", strconv.FormatInt(asset.ByteSize, 10))
	c.Header("Cache-Control", "private, no-store")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, reader)
}

func (handler *Handler) queueAnalysis(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	err := handler.service.QueueAnalysis(c.Request.Context(), workspaceID, userID, c.Param("id"))
	respondResult(c, gin.H{"queued": err == nil}, err)
}

func (handler *Handler) retryAnalysis(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	err := handler.service.RetryAnalysis(c.Request.Context(), workspaceID, userID, c.Param("id"))
	respondResult(c, gin.H{"queued": err == nil}, err)
}

func (handler *Handler) abandonAnalysis(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	err := handler.service.AbandonAnalysis(c.Request.Context(), workspaceID, userID, c.Param("id"))
	respondResult(c, gin.H{"abandoned": err == nil}, err)
}

func (handler *Handler) deleteReview(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	err := handler.service.DeleteReview(c.Request.Context(), workspaceID, userID, c.Param("id"))
	respondResult(c, gin.H{"deleting": err == nil}, err)
}

type differenceRequest struct {
	ReviewID string `json:"review_id"`
	designreview.DifferenceInput
}

func (handler *Handler) createDifference(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	var input differenceRequest
	if decodeStrictJSON(c, &input) != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	item, err := handler.service.CreateDifference(c.Request.Context(), workspaceID, userID, input.ReviewID, input.DifferenceInput)
	respondResultStatus(c, http.StatusCreated, item, err)
}

func (handler *Handler) updateDifference(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	var input differenceRequest
	if decodeStrictJSON(c, &input) != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	item, err := handler.service.UpdateDifference(c.Request.Context(), workspaceID, userID, input.ReviewID, c.Param("id"), input.DifferenceInput)
	respondResult(c, item, err)
}

func (handler *Handler) deleteDifference(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	err := handler.service.DeleteDifference(c.Request.Context(), workspaceID, userID, c.Query("review_id"), c.Param("id"))
	respondResult(c, gin.H{"deleted": err == nil}, err)
}

func (handler *Handler) verifyDifference(c *gin.Context) {
	var input struct {
		ReviewID string `json:"review_id"`
		Status   string `json:"status"`
	}
	if decodeStrictJSON(c, &input) != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	handler.transitionDifference(c, input.ReviewID, "verify", input.Status)
}

func (handler *Handler) resolveDifference(c *gin.Context) {
	handler.transitionDifferenceBody(c, "resolve")
}
func (handler *Handler) reopenDifference(c *gin.Context) {
	handler.transitionDifferenceBody(c, "reopen")
}

func (handler *Handler) transitionDifferenceBody(c *gin.Context, operation string) {
	var input struct {
		ReviewID string `json:"review_id"`
	}
	if decodeStrictJSON(c, &input) != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	handler.transitionDifference(c, input.ReviewID, operation, "")
}

func (handler *Handler) transitionDifference(c *gin.Context, reviewID, operation, status string) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	item, err := handler.service.TransitionDifference(c.Request.Context(), workspaceID, userID, reviewID, c.Param("id"), operation, status)
	respondResult(c, item, err)
}

func (handler *Handler) resolveRecheckMatch(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	var input struct {
		ReviewID   string `json:"review_id"`
		Resolution string `json:"resolution"`
	}
	if decodeStrictJSON(c, &input) != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	err := handler.service.ResolveRecheckMatch(c.Request.Context(), workspaceID, userID, input.ReviewID, c.Param("id"), input.Resolution)
	respondResult(c, gin.H{"resolved": err == nil}, err)
}

func (handler *Handler) overrideRecheckVerification(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	var input struct {
		ReviewID string  `json:"review_id"`
		Status   *string `json:"status"`
	}
	if decodeStrictJSON(c, &input) != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	err := handler.service.OverrideRecheckVerification(c.Request.Context(), workspaceID, userID, input.ReviewID, c.Param("id"), input.Status)
	respondResult(c, gin.H{"updated": err == nil}, err)
}

type deepRequest struct {
	ReviewID             string              `json:"review_id"`
	DesignRegion         designreview.Region `json:"design_region"`
	ImplementationRegion designreview.Region `json:"implementation_region"`
	Focus                string              `json:"focus"`
}

func (handler *Handler) createDeepReview(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	var input deepRequest
	if decodeStrictJSON(c, &input) != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	item, err := handler.service.CreateDeepReview(c.Request.Context(), workspaceID, userID, input.ReviewID, input.DesignRegion, input.ImplementationRegion, input.Focus, nil)
	respondResultStatus(c, http.StatusCreated, item, err)
}

func (handler *Handler) retryDeepReview(c *gin.Context)  { handler.transitionDeep(c, "retry") }
func (handler *Handler) cancelDeepReview(c *gin.Context) { handler.transitionDeep(c, "cancel") }

func (handler *Handler) transitionDeep(c *gin.Context, operation string) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	var input struct {
		ReviewID string `json:"review_id"`
	}
	if decodeStrictJSON(c, &input) != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	err := handler.service.TransitionDeepReview(c.Request.Context(), workspaceID, userID, input.ReviewID, c.Param("id"), operation)
	respondResult(c, gin.H{"status": operation}, err)
}

func (handler *Handler) listCollectors(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	items, err := handler.service.ListCollectors(c.Request.Context(), workspaceID, userID)
	respondResult(c, items, err)
}
func (handler *Handler) createPairingCode(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	item, err := handler.service.CreatePairingCode(c.Request.Context(), workspaceID, userID)
	respondResultStatus(c, http.StatusCreated, item, err)
}
func (handler *Handler) revokeCollector(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	err := handler.service.RevokeCollector(c.Request.Context(), workspaceID, userID, c.Param("id"))
	respondResult(c, gin.H{"revoked": err == nil}, err)
}
func (handler *Handler) listCaptures(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	items, err := handler.service.ListCaptures(c.Request.Context(), workspaceID, userID)
	respondResult(c, items, err)
}
func (handler *Handler) deleteCapture(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	err := handler.service.DeleteCapture(c.Request.Context(), workspaceID, userID, c.Param("id"))
	respondResult(c, gin.H{"deleted": err == nil}, err)
}

func requestIdentity(c *gin.Context) (uint, designreview.ActorIdentity, string, bool) {
	workspaceValue, workspaceOK := c.Get(pmauth.WorkspaceContextKey)
	workspace, validWorkspace := workspaceValue.(identitygorm.Workspace)
	factValue, factOK := c.Get(pmauth.ActorFactUserIDContextKey)
	factUserID, validFact := factValue.(uint)
	principalValue, principalOK := c.Get(pmauth.PrincipalContextKey)
	principal, validPrincipal := principalValue.(pmauth.Principal)
	actor := designreview.ActorIdentity{PrincipalUserID: principal.UserID, FactUserID: factUserID}
	if !workspaceOK || !validWorkspace || workspace.ID == 0 || !factOK || !validFact || !principalOK || !validPrincipal || !actor.Valid() {
		fail(c, http.StatusUnauthorized, errors.New("authenticated workspace is required"))
		return 0, designreview.ActorIdentity{}, "", false
	}
	name := strings.TrimSpace(principal.DisplayName)
	if name == "" {
		name = principal.Username
	}
	return workspace.ID, actor, name, true
}

func queryUint(c *gin.Context, key string) uint {
	value, _ := strconv.ParseUint(c.Query(key), 10, 64)
	return uint(value)
}
func queryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
func stringPointer(value string) *string           { result := strings.TrimSpace(value); return &result }
func respond(c *gin.Context, status int, data any) { c.JSON(status, gin.H{"data": data}) }
func respondResult(c *gin.Context, data any, err error) {
	respondResultStatus(c, http.StatusOK, data, err)
}
func respondResultStatus(c *gin.Context, status int, data any, err error) {
	if err != nil {
		failDomain(c, err)
		return
	}
	respond(c, status, data)
}
func fail(c *gin.Context, status int, err error) {
	c.AbortWithStatusJSON(status, gin.H{"error": err.Error()})
}
func failDomain(c *gin.Context, err error) {
	switch {
	case errors.Is(err, designreview.ErrInvalid), errors.Is(err, designreview.ErrRequirement), errors.Is(err, designreview.ErrEvidenceRequired):
		fail(c, http.StatusBadRequest, err)
	case errors.Is(err, designreview.ErrNotFound):
		fail(c, http.StatusNotFound, err)
	case errors.Is(err, designreview.ErrForbidden), errors.Is(err, designreview.ErrCreatorOnly):
		fail(c, http.StatusForbidden, err)
	case errors.Is(err, designreview.ErrConflict):
		fail(c, http.StatusConflict, err)
	case errors.Is(err, designreview.ErrUnavailable):
		fail(c, http.StatusServiceUnavailable, err)
	default:
		fail(c, http.StatusInternalServerError, fmt.Errorf("design review operation failed"))
	}
}
