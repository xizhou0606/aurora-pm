package qualityhttp

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

const (
	ErrorSceneOptionKindInvalid  = "QUALITY_SCENE_OPTION_KIND_INVALID"
	ErrorSceneOptionNameInvalid  = "QUALITY_SCENE_OPTION_NAME_INVALID"
	ErrorSceneOptionNameConflict = "QUALITY_SCENE_OPTION_NAME_CONFLICT"
	ErrorSceneOptionInUse        = "QUALITY_SCENE_OPTION_IN_USE"
	ErrorSceneOptionNotFound     = "QUALITY_SCENE_OPTION_NOT_FOUND"
)

type sceneOptionBody struct {
	Name string `json:"name"`
}

// SceneOptionHandler is the quality HTTP adapter. The service is assembled by
// the application HTTP composition root and contains the domain use case.
type SceneOptionHandler struct {
	service *quality.SceneOptionService
}

func NewSceneOptionHandler(service *quality.SceneOptionService) *SceneOptionHandler {
	return &SceneOptionHandler{service: service}
}

func (handler *SceneOptionHandler) List(c *gin.Context) {
	kind, valid := parseSceneOptionKind(c)
	if !valid {
		return
	}
	rows, err := handler.service.List(workspaceID(c), kind)
	if err != nil {
		databaseFailure(c)
		return
	}
	ok(c, gin.H{"items": rows})
}

func (handler *SceneOptionHandler) Create(c *gin.Context) {
	kind, valid := parseSceneOptionKind(c)
	if !valid {
		return
	}
	var body sceneOptionBody
	if c.ShouldBindJSON(&body) != nil {
		failCode(c, http.StatusBadRequest, ErrorSceneOptionNameInvalid, "scene option name is invalid")
		return
	}
	row, err := handler.service.Create(workspaceID(c), actorFactUserID(c), kind, body.Name)
	if errors.Is(err, quality.ErrSceneOptionNameInvalid) {
		failCode(c, http.StatusUnprocessableEntity, ErrorSceneOptionNameInvalid, "scene option name is invalid")
		return
	}
	if errors.Is(err, quality.ErrSceneOptionNameDuplicate) {
		failCode(c, http.StatusConflict, ErrorSceneOptionNameConflict, "scene option name already exists")
		return
	}
	if err != nil {
		databaseFailure(c)
		return
	}
	created(c, row)
}

func (handler *SceneOptionHandler) Delete(c *gin.Context) {
	kind, valid := parseSceneOptionKind(c)
	if !valid {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		failCode(c, http.StatusBadRequest, ErrorSceneOptionNotFound, "scene option not found")
		return
	}
	err = handler.service.Delete(workspaceID(c), uint(id), kind)
	var inUse *quality.SceneOptionInUseError
	if errors.As(err, &inUse) {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"code":        ErrorSceneOptionInUse,
			"error":       "scene option is still in use",
			"usage_count": inUse.UsageCount,
		})
		return
	}
	if errors.Is(err, quality.ErrSceneOptionNotFound) {
		failCode(c, http.StatusNotFound, ErrorSceneOptionNotFound, "scene option not found")
		return
	}
	if err != nil {
		databaseFailure(c)
		return
	}
	ok(c, gin.H{"deleted": true, "id": uint(id)})
}

func parseSceneOptionKind(c *gin.Context) (quality.SceneOptionKind, bool) {
	kind, err := quality.ParseSceneOptionKind(c.Param("kind"))
	if err != nil {
		failCode(c, http.StatusBadRequest, ErrorSceneOptionKindInvalid, "scene option kind is invalid")
		return "", false
	}
	return kind, true
}

func workspaceID(c *gin.Context) uint {
	value, _ := c.Get(pmauth.WorkspaceIDContextKey)
	workspaceID, _ := value.(uint)
	return workspaceID
}

func actorFactUserID(c *gin.Context) uint {
	value, _ := c.Get(pmauth.ActorFactUserIDContextKey)
	userID, _ := value.(uint)
	return userID
}

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, gin.H{"data": data})
}

func failCode(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code, "error": message})
}

func databaseFailure(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "database operation failed"})
}
