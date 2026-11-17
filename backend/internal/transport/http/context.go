package httpapi

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

const (
	actorKey     = pmauth.ActorContextKey
	workspaceKey = pmauth.WorkspaceContextKey
)

type actor struct {
	User   identitygorm.User
	Source string
}

func currentActor(c *gin.Context) (actor, bool) {
	value, ok := c.Get(actorKey)
	if !ok {
		return actor{}, false
	}
	result, ok := value.(actor)
	return result, ok && result.User.ID > 0
}

// currentPrincipal is the canonical PM identity available after authenticate.
// Unlike actor, it intentionally has no legacy role or workspace facts.
func currentPrincipal(c *gin.Context) (pmauth.Principal, bool) {
	value, ok := c.Get(pmauth.PrincipalContextKey)
	if !ok {
		return pmauth.Principal{}, false
	}
	result, ok := value.(pmauth.Principal)
	return result, ok && result.UserID > 0
}

// factUserID returns the platform user id stored by PM business records. The
// local-user fallback remains only for the staged rollback path.
func factUserID(user identitygorm.User) uint {
	return user.ID
}

func currentFactUserID(c *gin.Context) uint {
	a, _ := currentActor(c)
	return factUserID(a.User)
}

// actorFactUserID is the same fact-user resolution under the transport naming
// used by member mutation handlers.
func actorFactUserID(c *gin.Context) uint {
	return currentFactUserID(c)
}

func currentWorkspace(c *gin.Context) (identitygorm.Workspace, bool) {
	value, ok := c.Get(workspaceKey)
	if !ok {
		return identitygorm.Workspace{}, false
	}
	result, ok := value.(identitygorm.Workspace)
	return result, ok && result.ID > 0
}

func paramID(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(strings.TrimSpace(c.Param(name)), 10, 64)
	if err != nil || value == 0 {
		bad(c, name+" is invalid")
		return 0, false
	}
	return uint(value), true
}

func queryUint(c *gin.Context, names ...string) uint {
	for _, name := range names {
		value, _ := strconv.ParseUint(strings.TrimSpace(c.Query(name)), 10, 64)
		if value > 0 {
			return uint(value)
		}
	}
	return 0
}

func pagination(c *gin.Context, defaultLimit int) (int, int) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
