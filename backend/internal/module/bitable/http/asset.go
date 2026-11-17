package bitablehttp

import (
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
)

func (h *Handler) upload(c *gin.Context) {
	p, ok := principal(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, bitable.MaxAttachmentBytes+(1<<20))
	if e := c.Request.ParseMultipartForm(1 << 20); e != nil {
		failure(c, bitable.ErrInvalidInput)
		return
	}
	defer c.Request.MultipartForm.RemoveAll()
	if len(c.Request.MultipartForm.File) != 1 || len(c.Request.MultipartForm.File["file"]) != 1 {
		failure(c, bitable.ErrInvalidInput)
		return
	}
	file, header, e := c.Request.FormFile("file")
	if e != nil {
		failure(c, bitable.ErrInvalidInput)
		return
	}
	defer file.Close()
	baseRevision, e := strconv.ParseUint(c.PostForm("baseRevision"), 10, 64)
	if e != nil {
		failure(c, bitable.ErrInvalidInput)
		return
	}
	schema, e := strconv.ParseUint(c.PostForm("expectedSchemaRevision"), 10, 64)
	if e != nil {
		failure(c, bitable.ErrInvalidInput)
		return
	}
	fence, e := strconv.ParseUint(c.PostForm("fencingToken"), 10, 64)
	if e != nil {
		failure(c, bitable.ErrInvalidInput)
		return
	}
	contract := bitable.CellContract{
		RecordID: c.Param("recordId"), FieldID: c.Param("fieldId"), BaseRevision: baseRevision,
		LeaseID: c.PostForm("leaseId"), FencingToken: fence, ConnectionID: c.PostForm("connectionId"),
		EditorInstanceID: c.PostForm("editorInstanceId"),
	}
	v, e := h.service.Upload(c.Request.Context(), p, c.Param("resourceRef"), c.Param("tableId"), bitable.Upload{RecordID: c.Param("recordId"), FieldID: c.Param("fieldId"), OperationID: c.PostForm("operationId"), Name: header.Filename, ExpectedSchemaRevision: schema, CellContract: contract, Size: header.Size, Body: file})
	respond(c, v, e)
}
func (h *Handler) preview(c *gin.Context)  { h.open(c, false) }
func (h *Handler) download(c *gin.Context) { h.open(c, true) }
func (h *Handler) open(c *gin.Context, download bool) {
	p, ok := principal(c)
	if !ok {
		return
	}
	a, body, e := h.service.OpenAttachment(c.Request.Context(), p, c.Param("resourceRef"), c.Param("tableId"), c.Param("recordId"), c.Param("fieldId"), c.Param("assetId"))
	if e != nil {
		failure(c, e)
		return
	}
	defer body.Close()
	safe := a.MimeType == "image/png" || a.MimeType == "image/jpeg" || a.MimeType == "image/gif" || a.MimeType == "image/webp"
	disposition, contentType := "attachment", "application/octet-stream"
	if !download && safe {
		disposition, contentType = "inline", a.MimeType
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox; default-src 'none'")
	c.Header("Cross-Origin-Resource-Policy", "same-origin")
	c.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": a.Name}))
	c.Header("Content-Type", contentType)
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, io.LimitReader(body, bitable.MaxAttachmentBytes+1))
}
