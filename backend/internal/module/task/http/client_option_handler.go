package taskhttp

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type ClientOptionHandler struct{ service *task.ClientOptionService }

func NewClientOptionHandler(service *task.ClientOptionService) *ClientOptionHandler {
	return &ClientOptionHandler{service: service}
}

func (handler *ClientOptionHandler) List(c *gin.Context) {
	rows, err := handler.service.List(c.Request.Context(), workspaceID(c), actorUserID(c))
	if err != nil {
		fail(c, http.StatusInternalServerError, "TASK_CLIENT_OPTION_LIST_FAILED", "client options could not be loaded")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"items": rows}})
}

func (handler *ClientOptionHandler) Create(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
	}
	if c.ShouldBindJSON(&body) != nil {
		fail(c, http.StatusBadRequest, "TASK_CLIENT_OPTION_INVALID", "client option is invalid")
		return
	}
	row, err := handler.service.Create(c.Request.Context(), workspaceID(c), actorUserID(c), body.Name)
	if handleError(c, err) {
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": row})
}

func (handler *ClientOptionHandler) Update(c *gin.Context) {
	id, valid := optionID(c)
	if !valid {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if c.ShouldBindJSON(&body) != nil {
		fail(c, http.StatusBadRequest, "TASK_CLIENT_OPTION_INVALID", "client option is invalid")
		return
	}
	row, err := handler.service.Update(c.Request.Context(), workspaceID(c), id, body.Name)
	if handleError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (handler *ClientOptionHandler) Delete(c *gin.Context) {
	id, valid := optionID(c)
	if !valid {
		return
	}
	if handleError(c, handler.service.Delete(c.Request.Context(), workspaceID(c), id)) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": true, "id": id}})
}

func optionID(c *gin.Context) (uint, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || value == 0 {
		fail(c, http.StatusBadRequest, "TASK_CLIENT_OPTION_NOT_FOUND", "client option not found")
		return 0, false
	}
	return uint(value), true
}

func handleError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	var inUse *task.ClientOptionInUseError
	switch {
	case errors.Is(err, task.ErrClientOptionInvalid):
		fail(c, http.StatusUnprocessableEntity, "TASK_CLIENT_OPTION_INVALID", "client option is invalid")
	case errors.Is(err, task.ErrClientOptionDuplicate):
		fail(c, http.StatusConflict, "TASK_CLIENT_OPTION_CONFLICT", "client option name already exists")
	case errors.Is(err, task.ErrClientOptionNotFound):
		fail(c, http.StatusNotFound, "TASK_CLIENT_OPTION_NOT_FOUND", "client option not found")
	case errors.As(err, &inUse):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": "TASK_CLIENT_OPTION_IN_USE", "error": "client option is still in use", "usage_count": inUse.UsageCount})
	default:
		fail(c, http.StatusInternalServerError, "TASK_CLIENT_OPTION_MUTATION_FAILED", "client option mutation failed")
	}
	return true
}

func workspaceID(c *gin.Context) uint {
	value, _ := c.Get(pmauth.WorkspaceIDContextKey)
	result, _ := value.(uint)
	return result
}
func actorUserID(c *gin.Context) uint {
	value, _ := c.Get(pmauth.ActorFactUserIDContextKey)
	result, _ := value.(uint)
	return result
}
func fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code, "error": message})
}
