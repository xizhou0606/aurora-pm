package identityhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

type fakeOrgSyncSource struct{}

func (fakeOrgSyncSource) ListFeishuDepartments(context.Context) ([]identity.FeishuDepartment, error) {
	return []identity.FeishuDepartment{
		{OpenID: "od-a", ParentID: "0", Name: "研发组", MemberCount: 1},
	}, nil
}

func (fakeOrgSyncSource) ListFeishuDepartmentUsers(
	_ context.Context,
	departmentID string,
) ([]identity.FeishuDepartmentUser, error) {
	if departmentID == "od-a" {
		return []identity.FeishuDepartmentUser{
			{OpenID: "ou-1", Name: "张三", DepartmentID: "od-a"},
		}, nil
	}
	return nil, nil
}

type fakeOrgSyncStore struct{}

func (fakeOrgSyncStore) ReplaceDepartments(
	context.Context, []identity.FeishuDepartment,
) error {
	return nil
}

func (fakeOrgSyncStore) DepartmentSyncedAt(context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

func (fakeOrgSyncStore) ResolveUsersByOpenID(
	context.Context, []string,
) (map[string]uint, error) {
	return map[string]uint{"ou-1": 11}, nil
}

func (fakeOrgSyncStore) ResolveUniqueUsersByName(
	context.Context, []string,
) (map[string]uint, error) {
	return map[string]uint{}, nil
}

func (fakeOrgSyncStore) SetUserDepartments(
	_ context.Context, assignments []identity.FeishuUserDepartmentAssignment,
) error {
	return nil
}

func (fakeOrgSyncStore) ClearUserDepartments(context.Context, []uint) error {
	return nil
}

func (fakeOrgSyncStore) FeishuOrgStatus(context.Context) (identity.FeishuOrgSyncStatus, error) {
	return identity.FeishuOrgSyncStatus{
		HasSynced: true, SyncedAt: time.Now(), Departments: 1, MappedUsers: 1, UnmappedUsers: 0,
	}, nil
}

func newFeishuOrgTestRouter(t *testing.T, withOrg bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := NewAccessHandler(nil, nil)
	if withOrg {
		sync, err := identity.NewFeishuOrgSyncService(fakeOrgSyncSource{}, fakeOrgSyncStore{}, true)
		if err != nil {
			t.Fatal(err)
		}
		handler.SetFeishuOrg(sync)
	}
	engine := gin.New()
	handler.RegisterRoutes(engine.Group("/api/v1/core"))
	return engine
}

func TestSyncFeishuOrgFailsClosedWithoutService(t *testing.T) {
	engine := newFeishuOrgTestRouter(t, false)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/core/access/admin/groups/feishu-org/sync", nil))
	if recorder.Code != http.StatusConflict ||
		!strings.Contains(recorder.Body.String(), "FEISHU_ORG_SYNC_UNAVAILABLE") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/core/access/admin/groups/feishu-org/status", nil))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status endpoint status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSyncFeishuOrgRunsSyncAndReportsStatus(t *testing.T) {
	engine := newFeishuOrgTestRouter(t, true)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/core/access/admin/groups/feishu-org/sync", nil))
	if recorder.Code != http.StatusOK ||
		!strings.Contains(recorder.Body.String(), `"matched_users":1`) ||
		!strings.Contains(recorder.Body.String(), `"departments":1`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/core/access/admin/groups/feishu-org/status", nil))
	if recorder.Code != http.StatusOK ||
		!strings.Contains(recorder.Body.String(), `"has_synced":true`) ||
		!strings.Contains(recorder.Body.String(), `"mapped_users":1`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
