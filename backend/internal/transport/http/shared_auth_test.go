package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestSharedIdentityMapsExactBindingToPMPrincipal(t *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		if req.Header.Get("Authorization") != "Bearer shared-token" {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		if got := req.Header.Get("X-Workspace-ID"); got != "" {
			t.Fatalf("introspection received workspace header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"id":77,"issuer":"one2all","subject":"one2all|77","username":"upstream-alice"}}`)
	}))
	defer upstream.Close()

	app := newTestApp(t, func(cfg *config.Config) {
		cfg.AuthMode = "shared_jwt"
		cfg.SharedJWTIssuer = "one2all"
		cfg.SharedIntrospectionURL = upstream.URL
		cfg.SharedIntrospectionTimeout = time.Second
	})
	binding := identitygorm.User{
		ID: 701, ExternalIssuer: "one2all", ExternalSubject: "one2all|77", Username: "pm-alice", DisplayName: "PM Alice", Active: true,
	}
	if err := app.db.Create(&binding).Error; err != nil {
		t.Fatal(err)
	}

	server := &Server{cfg: app.cfg, db: app.db, tokens: app.tokens, authCache: map[string]cachedIdentity{}}
	engine := gin.New()
	engine.GET("/probe", server.authenticate, func(c *gin.Context) {
		principal, ok := currentPrincipal(c)
		if !ok {
			fail(c, http.StatusInternalServerError, "principal is missing")
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"principal_user_id": principal.UserID,
			"fact_user_id":      currentFactUserID(c),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("Authorization", "Bearer shared-token")
	req.Header.Set("X-Workspace-ID", "999")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	requireStatus(t, rec, http.StatusOK)

	var body map[string]uint
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["principal_user_id"] != binding.ID || body["fact_user_id"] != binding.ID {
		t.Fatalf("principal=%d fact=%d want principal=%d business=%d", body["principal_user_id"], body["fact_user_id"], binding.ID, binding.ID)
	}
	if calls.Load() != 1 {
		t.Fatalf("introspection calls=%d want=1", calls.Load())
	}
}

func TestSharedIdentityDerivesImportedSubjectWhenIntrospectionOmitsIt(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"id":77,"username":"upstream-alice"}}`)
	}))
	defer upstream.Close()

	app := newTestApp(t, func(cfg *config.Config) {
		cfg.AuthMode = "shared_jwt"
		cfg.SharedJWTIssuer = "one2all"
		cfg.SharedIntrospectionURL = upstream.URL
		cfg.SharedIntrospectionTimeout = time.Second
	})
	binding := identitygorm.User{
		ID: 702, ExternalIssuer: "one2all", ExternalSubject: "one2all|77", Username: "pm-alice", Active: true,
	}
	if err := app.db.Create(&binding).Error; err != nil {
		t.Fatal(err)
	}

	rec := app.request(t, http.MethodGet, "/api/v1/core/auth/me", "shared-token", 0, nil)
	requireStatus(t, rec, http.StatusOK)
	if got := uint(decodeData(t, rec).(map[string]any)["id"].(float64)); got != binding.ID {
		t.Fatalf("authenticated PM user id=%d want=%d", got, binding.ID)
	}
}

func TestSharedIdentityRequiresExistingEnabledExactBinding(t *testing.T) {
	tests := []struct {
		name  string
		seed  func(*testApp)
		count int64
	}{
		{
			name:  "missing binding",
			seed:  func(_ *testApp) {},
			count: 0,
		},
		{
			name: "disabled binding",
			seed: func(app *testApp) {
				user := identitygorm.User{
					ID: 703, ExternalIssuer: "one2all", ExternalSubject: "one2all|77", Username: "disabled-alice",
				}
				if err := app.db.Create(&user).Error; err != nil {
					t.Fatal(err)
				}
				if err := app.db.Model(&user).UpdateColumn("active", false).Error; err != nil {
					t.Fatal(err)
				}
			},
			count: 1,
		},
		{
			name: "numeric ID collision is not a binding",
			seed: func(app *testApp) {
				if err := app.db.Create(&identitygorm.User{
					ID: 77, ExternalIssuer: "local", ExternalSubject: "local:77", Username: "unrelated-local-user", Active: true,
				}).Error; err != nil {
					t.Fatal(err)
				}
			},
			count: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"data":{"id":77,"issuer":"one2all","subject":"one2all|77","username":"upstream-alice"}}`)
			}))
			defer upstream.Close()
			app := newTestApp(t, func(cfg *config.Config) {
				cfg.AuthMode = "shared_jwt"
				cfg.SharedJWTIssuer = "one2all"
				cfg.SharedIntrospectionURL = upstream.URL
				cfg.SharedIntrospectionTimeout = time.Second
			})
			test.seed(app)

			rec := app.request(t, http.MethodGet, "/api/v1/core/auth/me", "shared-token", 0, nil)
			requireStatus(t, rec, http.StatusForbidden)
			requireErrorCode(t, rec, apiErrorIdentityNotProvisioned)

			var users, members, workspaces int64
			app.db.Model(&identitygorm.User{}).Count(&users)
			app.db.Model(&identitygorm.WorkspaceMember{}).Count(&members)
			app.db.Model(&identitygorm.Workspace{}).Count(&workspaces)
			if users != test.count || members != 0 || workspaces != 0 {
				t.Fatalf("unexpected JIT writes: users=%d members=%d workspaces=%d", users, members, workspaces)
			}
		})
	}
}

func TestSharedTokenCacheAndWorkspaceContextAreIndependent(t *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		if got := req.Header.Get("X-Workspace-ID"); got != "" {
			t.Fatalf("introspection received workspace header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"id":77,"issuer":"one2all","subject":"one2all|77","username":"upstream-alice"}}`)
	}))
	defer upstream.Close()

	app := newTestApp(t, func(cfg *config.Config) {
		cfg.AuthMode = "shared_jwt"
		cfg.SharedJWTIssuer = "one2all"
		cfg.SharedIntrospectionURL = upstream.URL
		cfg.SharedIntrospectionTTL = 30 * time.Second
		cfg.SharedIntrospectionTimeout = time.Second
	})
	if err := app.db.Create(&identitygorm.User{
		ID: 704, ExternalIssuer: "one2all", ExternalSubject: "one2all|77", Username: "pm-alice", Active: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	for _, workspace := range []identitygorm.Workspace{
		{ID: 9, Name: "Workspace A", Slug: "workspace-a", Enabled: true},
		{ID: 10, Name: "Workspace B", Slug: "workspace-b", Enabled: true},
	} {
		if err := app.db.Create(&workspace).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, workspaceID := range []uint{9, 10} {
		if err := app.db.Create(&identitygorm.WorkspaceMember{
			WorkspaceID: workspaceID, UserID: 704, Role: "member", Enabled: true,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	for _, workspaceID := range []uint{9, 10} {
		rec := app.request(t, http.MethodGet, "/api/v1/platform/project-sets", "same-bearer", workspaceID, nil)
		requireStatus(t, rec, http.StatusOK)
	}
	if calls.Load() != 1 {
		t.Fatalf("introspection calls=%d want one token-cache entry", calls.Load())
	}
	var members int64
	app.db.Model(&identitygorm.WorkspaceMember{}).Count(&members)
	if members != 2 {
		t.Fatalf("workspace context changed membership rows=%d", members)
	}
}

func TestSharedAuthenticationMapsRevokedBearerToSessionExpired(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "revoked", http.StatusUnauthorized)
	}))
	defer upstream.Close()
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.AuthMode = "shared_jwt"
		cfg.SharedJWTIssuer = "one2all"
		cfg.SharedIntrospectionURL = upstream.URL
		cfg.SharedIntrospectionTimeout = time.Second
	})

	rec := app.request(t, http.MethodGet, "/api/v1/core/auth/me", "revoked", 0, nil)
	requireStatus(t, rec, http.StatusUnauthorized)
	requireErrorCode(t, rec, apiErrorSessionExpired)
}

func requireErrorCode(t *testing.T, rec *httptest.ResponseRecorder, expected string) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != expected {
		t.Fatalf("code=%v want=%s body=%s", body["code"], expected, rec.Body.String())
	}
}
