package projecthttp

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"

	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
	"strconv"
)

func (h *AccessGuideHandler) ConfigureRequests(s *project.JoinService) { h.requests = s }
func (h *AccessGuideHandler) RegisterRequestRoutes(group *gin.RouterGroup) {
	group.GET("/workspace-access-requests/:id", h.joinRequestContext)
	group.POST("/workspace-access-requests/:id", h.createJoinRequest)
	group.PATCH("/workspace-access-requests/:id/:requestId", h.reviewJoinRequest)
	group.GET("/project-access-guide/:kind/:id/requests", h.joinRequestContext)
	group.POST("/project-access-guide/:kind/:id/requests", h.createJoinRequest)
	group.PATCH("/project-access-guide/:kind/:id/requests/:requestId", h.reviewJoinRequest)
}
func (h *AccessGuideHandler) requestInput(c *gin.Context) (project.JoinActor, project.JoinTarget, bool) {
	raw, _ := c.Get(pmauth.PrincipalContextKey)
	p, ok := raw.(pmauth.Principal)
	fact, _ := c.Get(pmauth.ActorFactUserIDContextKey)
	f, _ := fact.(uint)
	if !ok || p.UserID == 0 || f == 0 {
		c.JSON(401, gin.H{"code": "UNAUTHORIZED"})
		return project.JoinActor{}, project.JoinTarget{}, false
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 || h.requests == nil {
		c.JSON(400, gin.H{"code": "INVALID_REQUEST"})
		return project.JoinActor{}, project.JoinTarget{}, false
	}
	kind := c.Param("kind")
	if kind == "" {
		kind = "workspace"
	} else if kind != "task" && kind != "quality_bug" && kind != "plan_unit" {
		joinResult(c, nil, project.ErrJoinInvalid)
		return project.JoinActor{}, project.JoinTarget{}, false
	}
	return project.JoinActor{UserID: p.UserID, FactID: f}, project.JoinTarget{Kind: kind, ID: uint(id)}, true
}
func joinResult(c *gin.Context, data any, err error) {
	if err == nil {
		c.JSON(200, gin.H{"data": data})
		return
	}
	status, code := 500, "PROJECT_ACCESS_REQUEST_FAILED"
	switch {
	case errors.Is(err, project.ErrJoinWorkspaceRequired):
		status, code = 403, "TARGET_WORKSPACE_MEMBERSHIP_REQUIRED"
	case errors.Is(err, project.ErrJoinForbidden):
		status, code = 403, "PROJECT_ACCESS_REQUEST_FORBIDDEN"
	case errors.Is(err, project.ErrJoinConflict):
		status, code = 409, "PROJECT_ACCESS_REQUEST_CONFLICT"
	case errors.Is(err, project.ErrJoinInvalid):
		status, code = 400, "PROJECT_ACCESS_REQUEST_INVALID"
	case errors.Is(err, project.ErrGuideUnavailable):
		status, code = 404, "NOT_FOUND"
	}
	c.JSON(status, gin.H{"code": code})
}
func (h *AccessGuideHandler) joinRequestContext(c *gin.Context) {
	a, t, ok := h.requestInput(c)
	if !ok {
		return
	}
	v, e := h.requests.Context(c.Request.Context(), a, t)
	joinResult(c, v, e)
}
func (h *AccessGuideHandler) createJoinRequest(c *gin.Context) {
	a, t, ok := h.requestInput(c)
	if !ok {
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if c.ShouldBindJSON(&b) != nil {
		joinResult(c, nil, project.ErrJoinInvalid)
		return
	}
	v, e := h.requests.Create(c.Request.Context(), a, t, b.Reason)
	joinResult(c, v, e)
}
func (h *AccessGuideHandler) reviewJoinRequest(c *gin.Context) {
	a, t, ok := h.requestInput(c)
	if !ok {
		return
	}
	id, err := strconv.ParseUint(c.Param("requestId"), 10, 64)
	if err != nil {
		joinResult(c, nil, project.ErrJoinInvalid)
		return
	}
	var b struct {
		Decision string `json:"decision"`
	}
	if c.ShouldBindJSON(&b) != nil {
		joinResult(c, nil, project.ErrJoinInvalid)
		return
	}
	v, e := h.requests.Review(c.Request.Context(), a, t, uint(id), b.Decision)
	joinResult(c, v, e)
}
