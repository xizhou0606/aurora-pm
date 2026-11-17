package documenthttp

import (
	"bufio"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

func (handler *Handler) listAssets(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, idOK := resourceID(c, "id")
	if !ok || !idOK {
		return
	}
	assets, err := handler.service.ListAssets(c.Request.Context(), principal, documentID)
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"items": assets})
}

func inlineAssetContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	switch strings.ToLower(mediaType) {
	case "image/png", "image/jpeg", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func setAssetContentHeaders(c *gin.Context, asset document.Asset) {
	c.Header("Content-Type", asset.ContentType)
	c.Header("Content-Length", strconv.FormatInt(asset.SizeBytes, 10))
	c.Header("Cache-Control", "private, no-store")
	disposition := "attachment"
	if inlineAssetContentType(asset.ContentType) {
		disposition = "inline"
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": asset.FileName}))
}

func (handler *Handler) uploadAsset(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, idOK := resourceID(c, "id")
	if !ok || !idOK {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, document.DocumentAssetMaxBytes+1024*1024)
	if err := c.Request.ParseMultipartForm(4 * 1024 * 1024); err != nil {
		writeError(c, document.ErrInvalidInput)
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil || fileHeader.Size <= 0 || fileHeader.Size > document.DocumentAssetMaxBytes {
		writeError(c, document.ErrInvalidInput)
		return
	}
	source, err := fileHeader.Open()
	if err != nil {
		writeError(c, document.ErrInvalidInput)
		return
	}
	defer source.Close()
	buffered := bufio.NewReader(source)
	header, peekErr := buffered.Peek(512)
	if peekErr != nil && len(header) == 0 {
		writeError(c, document.ErrInvalidInput)
		return
	}
	asset, err := handler.service.UploadAsset(c.Request.Context(), principal, documentID, document.AssetUpload{
		FileName: fileHeader.Filename, ContentType: strings.ToLower(http.DetectContentType(header)),
		SizeBytes: fileHeader.Size, Body: buffered,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusCreated, asset)
}

func (handler *Handler) downloadAsset(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, documentOK := resourceID(c, "id")
	assetID, assetOK := resourceID(c, "assetId")
	if !ok || !documentOK || !assetOK {
		return
	}
	asset, body, err := handler.service.OpenAsset(c.Request.Context(), principal, documentID, assetID)
	if err != nil {
		writeError(c, err)
		return
	}
	defer body.Close()
	setAssetContentHeaders(c, asset)
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, body)
}

func (handler *Handler) deleteAsset(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, documentOK := resourceID(c, "id")
	assetID, assetOK := resourceID(c, "assetId")
	if !ok || !documentOK || !assetOK {
		return
	}
	if err := handler.service.DeleteAsset(c.Request.Context(), principal, documentID, assetID); err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"deleted_id": assetID})
}
