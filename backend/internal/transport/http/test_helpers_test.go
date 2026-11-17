package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
)

var testDBCounter atomic.Uint64

type testApp struct {
	engine        http.Handler
	db            *gorm.DB
	cfg           config.Config
	tokens        *pmauth.Tokens
	projectAccess project.AccessChecker
}

func newTestApp(t *testing.T, mutate func(*config.Config)) *testApp {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		Environment: "test", HTTPAddr: ":0", DatabaseDriver: "sqlite",
		DatabaseDSN: fmt.Sprintf("file:pm_test_%d?mode=memory&cache=shared", testDBCounter.Add(1)),
		AuthMode:    "local", LocalJWTSecret: "pm-test-local-secret-that-is-at-least-thirty-two-characters",
		TokenTTL: 3600 * 1e9,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	// Test-only retired column: legacy collision fixtures prove online code ignores it.
	if !db.Migrator().HasColumn("users", "external_user_id") {
		if err := db.Exec("ALTER TABLE users ADD COLUMN external_user_id bigint").Error; err != nil {
			t.Fatal(err)
		}
	}
	app := &testApp{projectAccess: fakeAccessAllower{}}
	app.engine = NewWithDependencies(cfg, db, Dependencies{
		ProjectAccess: app.projectAccess,
	})
	app.db = db
	app.cfg = cfg
	app.tokens = pmauth.NewTokens(cfg)
	return app
}

// rebuildEngine replaces the Gin engine with a new one that preserves the
// same project-access dependency as the original app.
func (a *testApp) rebuildEngine(t *testing.T, store materialObjectStore, storeErr error) {
	t.Helper()
	a.engine = newEngineWithDependencies(a.cfg, a.db, store, storeErr, Dependencies{
		ProjectAccess: a.projectAccess,
	})
}

func (a *testApp) createUser(t *testing.T, id uint, username, role, password string) identitygorm.User {
	t.Helper()
	hash := ""
	if password != "" {
		raw, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
		if err != nil {
			t.Fatal(err)
		}
		hash = string(raw)
	}
	user := identitygorm.User{
		ID: id, ExternalSubject: fmt.Sprintf("local:%d", id),
		Username: username, DisplayName: username, Role: role,
		PasswordHash: hash, Active: true,
	}
	if err := a.db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if role == identity.LegacySuperAdminRole {
		administratorRole := identitygorm.AccessRole{
			Code: identity.AccessAdministratorRoleCode, Name: "Access administrator",
			Enabled: true, SystemManaged: true,
		}
		if err := a.db.Where("code = ?", administratorRole.Code).FirstOrCreate(&administratorRole).Error; err != nil {
			t.Fatal(err)
		}
		if err := a.db.FirstOrCreate(&identitygorm.AccessUserRole{UserID: user.ID, RoleID: administratorRole.ID}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return user
}

func (a *testApp) token(t *testing.T, user identitygorm.User) string {
	t.Helper()
	token, err := a.tokens.SignLocal(user.ID, user.Username, user.Role)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func (a *testApp) request(
	t *testing.T,
	method, path, token string,
	workspaceID uint,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if workspaceID > 0 {
		req.Header.Set("X-Workspace-ID", fmt.Sprint(workspaceID))
	}
	rec := httptest.NewRecorder()
	a.engine.ServeHTTP(rec, req)
	return rec
}

func decodeData(t *testing.T, rec *httptest.ResponseRecorder) any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response %d %q: %v", rec.Code, rec.Body.String(), err)
	}
	return envelope["data"]
}

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, status, rec.Body.String())
	}
}

// fakeAccessAllower is a test stub that grants project access to every user.
// It also implements AccessFilterBuilder with an empty filter (admin fast
// path), so every project is visible.
type fakeAccessAllower struct{}

func (fakeAccessAllower) CanAccessProject(_ context.Context, _, _, _ uint) (bool, error) {
	return true, nil
}
func (fakeAccessAllower) RequireProjectAccess(_ context.Context, _, _, _ uint) error {
	return nil
}
func (fakeAccessAllower) IsAccessAdministrator(_ context.Context, _, _ uint) (bool, error) {
	return true, nil
}
func (fakeAccessAllower) BuildMemberFilter(_ context.Context, _, _ string, _, _ uint) (project.AccessFilter, error) {
	return project.AccessFilter{}, nil
}
func (fakeAccessAllower) CanAccessPlanUnit(_ context.Context, _, _, _ uint) (bool, error) {
	return true, nil
}
func (fakeAccessAllower) CanManagePlanUnitMembers(_ context.Context, _, _, _ uint) (bool, error) {
	return true, nil
}

// fakeAccessByRole grants project access when the userID is in `allowed`, and
// administrator capability only when the userID is also in `admins`. This
// mirrors the production AccessService split: access vs. admin-role.
// projectAccessByUser optionally scopes each user's SQL filter to that user's
// accessible project IDs; a user without an entry gets a deny-all filter
// (matching the production owner/participant EXISTS filter, which matches
// nothing for a non-member).
type fakeAccessByRole struct {
	allowed             map[uint]struct{}
	admins              map[uint]struct{}
	projectAccessByUser map[uint][]uint
}

func newFakeAccessByRole(admins []uint, allowed []uint) *fakeAccessByRole {
	f := &fakeAccessByRole{
		allowed: make(map[uint]struct{}, len(allowed)),
		admins:  make(map[uint]struct{}, len(admins)),
	}
	for _, id := range admins {
		f.admins[id] = struct{}{}
	}
	for _, id := range allowed {
		f.allowed[id] = struct{}{}
	}
	return f
}

func (f *fakeAccessByRole) CanAccessProject(_ context.Context, _, projectID, userID uint) (bool, error) {
	if _, isAdmin := f.admins[userID]; isAdmin {
		return true, nil
	}
	if _, ok := f.allowed[userID]; !ok {
		return false, nil
	}
	for _, id := range f.projectAccessByUser[userID] {
		if id == projectID {
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeAccessByRole) RequireProjectAccess(_ context.Context, _, projectID, userID uint) error {
	allowed, err := f.CanAccessProject(nil, 0, projectID, userID)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("denied")
	}
	return nil
}
func (f *fakeAccessByRole) IsAccessAdministrator(_ context.Context, _, userID uint) (bool, error) {
	_, ok := f.admins[userID]
	return ok, nil
}
func (f *fakeAccessByRole) CanAccessPlanUnit(_ context.Context, _, planUnitID, userID uint) (bool, error) {
	if _, isAdmin := f.admins[userID]; isAdmin {
		return true, nil
	}
	if _, ok := f.allowed[userID]; !ok {
		return false, nil
	}
	for _, id := range f.projectAccessByUser[userID] {
		if id == planUnitID {
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeAccessByRole) CanManagePlanUnitMembers(_ context.Context, _, _, userID uint) (bool, error) {
	_, ok := f.admins[userID]
	return ok, nil
}
func (f *fakeAccessByRole) BuildMemberFilter(_ context.Context, table, projectColumn string, _, userID uint) (project.AccessFilter, error) {
	if _, isAdmin := f.admins[userID]; isAdmin {
		return project.AccessFilter{}, nil
	}
	column := projectColumn
	if table != "" && !strings.Contains(column, ".") {
		column = table + "." + column
	}
	// The fake maps memberships by project-set ID; for plan units that means
	// the owning set, not the unit's own id.
	if column == "plan_units.id" {
		column = "plan_units.project_new_id"
	}
	ids := f.projectAccessByUser[userID]
	if len(ids) == 0 {
		// Non-members match no project, mirroring the production EXISTS filter.
		return project.AccessFilter{SQL: "1 = 0"}, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	return project.AccessFilter{SQL: column + " IN (" + strings.Join(placeholders, ", ") + ")", Args: args}, nil
}

// newTestAppWithProjectAccess creates a test app with a fake project access
// checker (allows everything), real MemberFacts, and real
// ResponsibilityInspector.
func newTestAppWithProjectAccess(t *testing.T, mutate func(*config.Config)) *testApp {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		Environment: "test", HTTPAddr: ":0", DatabaseDriver: "sqlite",
		DatabaseDSN:    fmt.Sprintf("file:pm_test_%d?mode=memory&cache=shared", testDBCounter.Add(1)),
		AuthMode:       "local",
		LocalJWTSecret: "pm-test-local-secret-that-is-at-least-thirty-two-characters",
		TokenTTL:       3600 * 1e9,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	// Test-only retired column: legacy collision fixtures prove online code ignores it.
	if !db.Migrator().HasColumn("users", "external_user_id") {
		if err := db.Exec("ALTER TABLE users ADD COLUMN external_user_id bigint").Error; err != nil {
			t.Fatal(err)
		}
	}

	memberRepo := projectgorm.NewPlanUnitMemberRepository(db)
	impactRepo := projectgorm.NewRemovalImpactRepository(db)
	engine := NewWithDependencies(cfg, db, Dependencies{
		ProjectAccess:           fakeAccessAllower{},
		MemberFacts:             memberRepo,
		ResponsibilityInspector: impactRepo,
	})
	return &testApp{engine: engine, db: db, cfg: cfg, tokens: pmauth.NewTokens(cfg), projectAccess: fakeAccessAllower{}}
}

// newTestAppWithRoleAccess creates a test app where `admins` get the
// access-administrator capability and `allowed` users can access the project.
// Use admins=[] for non-admin negative tests. projectAccessByUser scopes each
// user's SQL filter to their accessible project IDs.
func newTestAppWithRoleAccess(t *testing.T, admins []uint, allowed []uint, mutate func(*config.Config)) *testApp {
	return newTestAppWithRoleAccessAndProjects(t, admins, allowed, nil, mutate)
}

// newTestAppWithRoleAccessAndProjects is newTestAppWithRoleAccess with a
// per-user accessible-project mapping for the SQL-level filter.
func newTestAppWithRoleAccessAndProjects(
	t *testing.T, admins, allowed []uint, projectAccessByUser map[uint][]uint, mutate func(*config.Config),
) *testApp {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		Environment: "test", HTTPAddr: ":0", DatabaseDriver: "sqlite",
		DatabaseDSN:    fmt.Sprintf("file:pm_test_%d?mode=memory&cache=shared", testDBCounter.Add(1)),
		AuthMode:       "local",
		LocalJWTSecret: "pm-test-local-secret-that-is-at-least-thirty-two-characters",
		TokenTTL:       3600 * 1e9,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	// Test-only retired column: legacy collision fixtures prove online code ignores it.
	if !db.Migrator().HasColumn("users", "external_user_id") {
		if err := db.Exec("ALTER TABLE users ADD COLUMN external_user_id bigint").Error; err != nil {
			t.Fatal(err)
		}
	}

	access := newFakeAccessByRole(admins, allowed)
	access.projectAccessByUser = projectAccessByUser
	memberRepo := projectgorm.NewPlanUnitMemberRepository(db)
	impactRepo := projectgorm.NewRemovalImpactRepository(db)
	engine := NewWithDependencies(cfg, db, Dependencies{
		ProjectAccess:           access,
		MemberFacts:             memberRepo,
		ResponsibilityInspector: impactRepo,
	})
	return &testApp{engine: engine, db: db, cfg: cfg, tokens: pmauth.NewTokens(cfg), projectAccess: access}
}

// newTestAppDenyAll creates a test app where project access ALWAYS fails.
// Use this for negative tests proving non-members get 404.
func newTestAppDenyAll(t *testing.T, mutate func(*config.Config)) *testApp {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		Environment: "test", HTTPAddr: ":0", DatabaseDriver: "sqlite",
		DatabaseDSN:    fmt.Sprintf("file:pm_test_%d?mode=memory&cache=shared", testDBCounter.Add(1)),
		AuthMode:       "local",
		LocalJWTSecret: "pm-test-local-secret-that-is-at-least-thirty-two-characters",
		TokenTTL:       3600 * 1e9,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	// Test-only retired column: legacy collision fixtures prove online code ignores it.
	if !db.Migrator().HasColumn("users", "external_user_id") {
		if err := db.Exec("ALTER TABLE users ADD COLUMN external_user_id bigint").Error; err != nil {
			t.Fatal(err)
		}
	}
	engine := NewWithDependencies(cfg, db, Dependencies{
		ProjectAccess: fakeDenyAll{},
	})
	return &testApp{engine: engine, db: db, cfg: cfg, tokens: pmauth.NewTokens(cfg), projectAccess: fakeDenyAll{}}
}

// fakeDenyAll always refuses project access.
type fakeDenyAll struct{}

func (fakeDenyAll) CanAccessProject(_ context.Context, _, _, _ uint) (bool, error) {
	return false, nil
}
func (fakeDenyAll) RequireProjectAccess(_ context.Context, _, _, _ uint) error {
	return project.ErrProjectAccessNotFound
}
func (fakeDenyAll) IsAccessAdministrator(_ context.Context, _, _ uint) (bool, error) {
	return false, nil
}
