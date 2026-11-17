package requirementhttp

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type linkHandlerStore struct {
	command requirement.ResourceLinkCommand
}

func (s *linkHandlerStore) ReadResourceLinks(context.Context, uint, uint, uint, bool) (requirement.ResourceLinkPage, error) {
	return requirement.ResourceLinkPage{}, nil
}
func (s *linkHandlerStore) ApplyResourceLink(_ context.Context, c requirement.ResourceLinkCommand) (requirement.ResourceLinkPage, error) {
	s.command = c
	return requirement.ResourceLinkPage{}, nil
}
func (s *linkHandlerStore) ResourceLinkProjectSets(context.Context, uint, []string) (map[string][]uint, error) {
	return nil, nil
}
func TestResourceLinkHTTPUsesAuthenticatedScopeAndPathOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, authenticated := range []bool{false, true} {
		store := &linkHandlerStore{}
		router := gin.New()
		group := router.Group("/api/v1/platform")
		if authenticated {
			group.Use(func(c *gin.Context) {
				c.Set(pmauth.PrincipalContextKey, pmauth.Principal{UserID: 3})
				c.Set(pmauth.WorkspaceIDContextKey, uint(7))
			})
		}
		NewResourceLinksHandler(requirement.NewResourceLinkService(store)).RegisterRoutes(group)
		request := httptest.NewRequest(http.MethodDelete, "/api/v1/platform/requirements/11/document-resources/doc", strings.NewReader(`{"resource_id":"forged","WorkspaceID":999,"UserID":999,"RequirementID":999,"Remove":false,"expected_version":1,"expected_policy_revision":1,"operation_id":"test"}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if !authenticated {
			if response.Code != 401 || store.command.WorkspaceID != 0 {
				t.Fatal("unauthenticated mutation")
			}
			continue
		}
		if response.Code != 200 {
			t.Fatalf("status %d: %s", response.Code, response.Body.String())
		}
		c := store.command
		if c.WorkspaceID != 7 || c.UserID != 3 || c.RequirementID != 11 || c.ResourceID != "doc" || !c.Remove {
			t.Fatalf("forged scope accepted: %+v", c)
		}
	}
}
