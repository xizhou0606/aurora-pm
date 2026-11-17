package requirementhttp

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

// MaterialHandler exposes requirement-owned design and test-case files.
type MaterialHandler struct {
	service *requirement.MaterialService
}

func NewMaterialHandler(service *requirement.MaterialService) *MaterialHandler {
	return &MaterialHandler{service: service}
}

func (handler *MaterialHandler) List(c *gin.Context) {
	requirementID, ok := routeID(c, "id")
	if !ok {
		return
	}
	kind := strings.TrimSpace(c.Query("kind"))
	rows, err := handler.service.List(workspaceID(c), requirementID, kind)
	if errors.Is(err, requirement.ErrMaterialKindInvalid) {
		failCode(c, http.StatusBadRequest, "REQUIREMENT_MATERIAL_KIND_INVALID", "kind must be design or test_case")
		return
	}
	if err != nil {
		databaseFailure(c)
		return
	}
	data := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		data = append(data, MaterialDTO(row))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}

func (handler *MaterialHandler) Delete(c *gin.Context) {
	requirementID, ok := routeID(c, "id")
	if !ok {
		return
	}
	materialID, ok := routeID(c, "materialId")
	if !ok {
		return
	}
	err := handler.service.Delete(workspaceID(c), requirementID, materialID)
	if errors.Is(err, requirement.ErrMaterialNotFound) {
		failCode(c, http.StatusNotFound, "REQUIREMENT_MATERIAL_NOT_FOUND", "requirement material not found")
		return
	}
	if err != nil {
		databaseFailure(c)
		return
	}
	okJSON(c, gin.H{"deleted": true, "id": materialID})
}

func MaterialDTO(row requirement.Material) gin.H {
	kind := "uploaded"
	if row.Source == requirement.MaterialSourceAIGenerated {
		kind = "ai_generated"
	}
	return gin.H{
		"id": row.ID, "requirement_id": row.RequirementID, "kind": row.Kind, "source": row.Source,
		"name": row.Name, "content_type": row.ContentType, "size": row.Size,
		"url": MaterialOpenURL(row), "origin_url": MaterialOriginURL(row), "artifact_kind": kind,
		"ai_run_id": row.AIRunID, "uploader_user_id": row.UploaderUserID, "created_at": row.CreatedAt,
	}
}

func materialDownloadPath(row requirement.Material) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(row.URL)), "http") && strings.TrimSpace(row.ObjectKey) == "" {
		return row.URL
	}
	return fmt.Sprintf("/api/v1/platform/requirements/%d/materials/%d/download", row.RequirementID, row.ID)
}

func workspaceID(c *gin.Context) uint {
	value, _ := c.Get(pmauth.WorkspaceIDContextKey)
	id, _ := value.(uint)
	return id
}

func actorFactUserID(c *gin.Context) uint {
	value, _ := c.Get(pmauth.ActorFactUserIDContextKey)
	id, _ := value.(uint)
	return id
}

func routeID(c *gin.Context, name string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		failCode(c, http.StatusNotFound, "REQUIREMENT_MATERIAL_NOT_FOUND", "requirement material not found")
		return 0, false
	}
	return uint(id), true
}

func okJSON(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func createdJSON(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, gin.H{"data": data})
}

func failCode(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code, "error": message})
}

func databaseFailure(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "database operation failed"})
}
