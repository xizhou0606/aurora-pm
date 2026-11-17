package projecthttp

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"strconv"
)

type AccessGuideHandler struct {
	service     project.AccessGuideService
	requests    *project.JoinService
	memberships project.GuideMembershipReader
}

func NewAccessGuideHandler(s project.AccessGuideService) *AccessGuideHandler {
	return &AccessGuideHandler{service: s}
}

func (h *AccessGuideHandler) ConfigureMemberships(reader project.GuideMembershipReader) {
	h.memberships = reader
}

// Mounted behind authentication, account admission and canonical access policy.
func (h *AccessGuideHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/project-access-guide/:kind/:id", h.get)
	h.RegisterRequestRoutes(group)
}
func (h *AccessGuideHandler) get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"code": "INVALID_ID"})
		return
	}
	result, err := h.service.Get(c.Request.Context(), c.Param("kind"), uint(id))
	if errors.Is(err, project.ErrGuideUnavailable) {
		c.JSON(404, gin.H{"code": "NOT_FOUND"})
		return
	}
	if err != nil {
		c.JSON(500, gin.H{"code": "PROJECT_ACCESS_GUIDE_FAILED"})
		return
	}
	actor, _, ok := h.requestInput(c)
	if !ok {
		return
	}
	if h.memberships == nil {
		c.JSON(503, gin.H{"code": "PROJECT_ACCESS_GUIDE_FAILED"})
		return
	}
	states, err := h.memberships.GuideMemberships(c.Request.Context(), actor, result)
	if err != nil {
		joinResult(c, nil, err)
		return
	}
	c.JSON(200, gin.H{"data": project.GuideDetails{AccessGuide: result, Memberships: states}})
}
