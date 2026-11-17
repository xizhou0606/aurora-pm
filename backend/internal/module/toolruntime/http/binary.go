package toolruntimehttp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

func (h *Handler) confirmFile(c *gin.Context, auth toolruntime.ExecutionAuthorization) {
	executor, ok := h.executor.(toolruntime.BinaryChangeSetExecutor)
	if !ok {
		toolFailure(c, toolruntime.ErrToolUnavailable)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, toolruntime.MaxToolFileBytes+(1<<20))
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		toolFailure(c, toolruntime.LookupBusinessError("TOOL_FILE_INVALID"))
		return
	}
	defer c.Request.MultipartForm.RemoveAll()
	files := c.Request.MultipartForm.File["file"]
	if len(files) != 1 || len(c.Request.MultipartForm.File) != 1 || len(c.Request.MultipartForm.Value) != 0 {
		toolFailure(c, toolruntime.LookupBusinessError("TOOL_FILE_INVALID"))
		return
	}
	source, err := files[0].Open()
	if err != nil {
		toolFailure(c, toolruntime.LookupBusinessError("TOOL_FILE_INVALID"))
		return
	}
	defer source.Close()
	result, err := executor.ConfirmChangeSetWithFile(c.Request.Context(), c.Param("id"), auth, toolruntime.BinaryFile{Reader: source, Size: files[0].Size})
	if err != nil {
		toolFailure(c, err)
		return
	}
	toolOK(c, result)
}
