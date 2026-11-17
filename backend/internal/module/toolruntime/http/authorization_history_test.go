package toolruntimehttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type historyHTTPAccess struct{ revoke bool }

func (historyHTTPAccess) CanManage(context.Context, toolruntime.ManagementPrincipal) (bool, error) {
	return false, nil
}
func (a historyHTTPAccess) CanRevoke(context.Context, toolruntime.ManagementPrincipal) (bool, error) {
	return a.revoke, nil
}
func (historyHTTPAccess) Names(context.Context, []uint) (map[uint]toolruntime.AuthorizationUser, error) {
	return nil, nil
}

type historyHTTPStore struct{}

func (historyHTTPStore) ListHistory(_ context.Context, s toolruntime.AuthorizationHistoryScope, p, n int, _ time.Time) (toolruntime.AuthorizationHistoryPage, error) {
	return toolruntime.AuthorizationHistoryPage{Items: []toolruntime.AuthorizationHistoryRecord{}, Page: p, PageSize: n}, nil
}
func (historyHTTPStore) RevokeHistory(_ context.Context, _ toolruntime.AuthorizationHistoryScope, id string, _ time.Time) error {
	if id != "r_own" {
		return toolruntime.ErrAuthorizationRecordNotFound
	}
	return nil
}
func TestAuthorizationHistoryHTTPContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		method, path  string
		login, revoke bool
		status        int
	}{
		{"GET", "/external-tooling/authorization-records", false, false, 401},
		{"GET", "/external-tooling/authorization-records?page=1&page_size=20", true, false, 200},
		{"GET", "/external-tooling/authorization-records?page_size=101", true, false, 400},
		{"GET", "/external-tooling/authorization-records?page=abc", true, false, 400},
		{"POST", "/external-tooling/authorization-records/r_own/revoke", true, false, 403},
		{"POST", "/external-tooling/authorization-records/r_other/revoke", true, true, 404},
		{"POST", "/external-tooling/authorization-records/r_own/revoke", true, true, 204},
	} {
		t.Run(test.path+string(rune(test.status)), func(t *testing.T) {
			handler := NewCredentialHandler(&toolruntime.CredentialService{}, ManagementPrincipalResolverFunc(func(*gin.Context) (toolruntime.ManagementPrincipal, error) {
				if !test.login {
					return toolruntime.ManagementPrincipal{}, nil
				}
				return toolruntime.ManagementPrincipal{PrincipalID: 7, WorkspaceID: 2, WorkspaceName: "test"}, nil
			})).WithAuthorizationHistory(toolruntime.NewAuthorizationHistoryService(historyHTTPStore{}, historyHTTPAccess{revoke: test.revoke}))
			engine := gin.New()
			handler.RegisterManagementRoutes(engine.Group(""))
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
			if response.Code != test.status {
				t.Fatalf("%d %s", response.Code, response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("history response cached")
			}
			if test.status == http.StatusOK && !strings.Contains(response.Body.String(), `"page_size":20`) {
				t.Fatal(response.Body.String())
			}
		})
	}
}
