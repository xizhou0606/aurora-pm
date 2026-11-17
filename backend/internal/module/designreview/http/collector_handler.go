package designreviewhttp

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func (handler *Handler) RegisterCollectorRoutes(group *gin.RouterGroup) {
	group.POST("/pair", handler.pairCollector)
	group.POST("/collector/heartbeat", handler.withCollector(handler.heartbeatCollector))
	group.POST("/collector/claim", handler.withCollector(handler.claimCollectorCapture))
	group.POST("/collector/drafts/:id/complete", handler.withCollector(handler.completeCollectorCapture))
	group.POST("/collector/drafts/:id/fail", handler.withCollector(handler.failCollectorCapture))
}

func (handler *Handler) pairCollector(c *gin.Context) {
	var input designreview.PairCollectorInput
	if decodeStrictJSON(c, &input) != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	result, err := handler.service.PairCollector(c.Request.Context(), input)
	respondResultStatus(c, http.StatusCreated, result, err)
}

func (handler *Handler) heartbeatCollector(c *gin.Context, collector designreview.MobileCollector) {
	var input designreview.CollectorHeartbeatInput
	if decodeStrictJSON(c, &input) != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	err := handler.service.HeartbeatCollector(c.Request.Context(), collector, input)
	respondResult(c, gin.H{"accepted": err == nil}, err)
}

type collectorHandler func(*gin.Context, designreview.MobileCollector)

func (handler *Handler) withCollector(next collectorHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(strings.ToLower(header), "collector ") {
			fail(c, http.StatusUnauthorized, designreview.ErrForbidden)
			return
		}
		collector, err := handler.service.AuthenticateCollector(c.Request.Context(), strings.TrimSpace(header[len("collector "):]))
		if err != nil {
			failDomain(c, err)
			return
		}
		next(c, collector)
	}
}

func (handler *Handler) claimCollectorCapture(c *gin.Context, collector designreview.MobileCollector) {
	draft, err := handler.service.ClaimCapture(c.Request.Context(), collector)
	if err == nil && draft == nil {
		c.Status(http.StatusNoContent)
		return
	}
	respondResult(c, draft, err)
}

func (handler *Handler) completeCollectorCapture(c *gin.Context, collector designreview.MobileCollector) {
	file, err := c.FormFile("screenshot")
	if err != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	opened, err := file.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	defer opened.Close()
	var input designreview.MobileCaptureResult
	decoder := json.NewDecoder(strings.NewReader(c.PostForm("metadata")))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	err = handler.service.CompleteCapture(c.Request.Context(), collector, c.Param("id"), file.Header.Get("Content-Type"), opened, input)
	respondResult(c, gin.H{"completed": err == nil}, err)
}

func (handler *Handler) failCollectorCapture(c *gin.Context, collector designreview.MobileCollector) {
	var input struct {
		Error string `json:"error"`
	}
	if decodeStrictJSON(c, &input) != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	err := handler.service.FailCapture(c.Request.Context(), collector, c.Param("id"), input.Error)
	respondResult(c, gin.H{"failed": err == nil}, err)
}

func (handler *Handler) createCapture(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	var input struct {
		CollectorID string `json:"collector_id"`
		Platform    string `json:"platform"`
		DeviceID    string `json:"device_id"`
	}
	if decodeStrictJSON(c, &input) != nil {
		fail(c, http.StatusBadRequest, designreview.ErrInvalid)
		return
	}
	item, err := handler.service.CreateCapture(c.Request.Context(), workspaceID, userID, input.CollectorID, input.Platform, input.DeviceID)
	respondResultStatus(c, http.StatusCreated, item, err)
}

func (handler *Handler) consumeCapture(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	item, err := handler.service.ConsumeCapture(c.Request.Context(), workspaceID, userID, c.Param("id"))
	respondResult(c, item, err)
}

func (handler *Handler) downloadCaptureAsset(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	asset, reader, err := handler.service.CaptureAsset(c.Request.Context(), workspaceID, userID, c.Param("id"))
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

func decodeStrictJSON(c *gin.Context, target any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return designreview.ErrInvalid
	}
	return nil
}
