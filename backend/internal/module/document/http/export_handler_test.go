package documenthttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type exportStore struct {
	document.Store
	row document.Document
}

func (store exportStore) GetDocument(_ context.Context, workspaceID, id uint) (document.Document, error) {
	if store.row.WorkspaceID != workspaceID || store.row.ID != id || store.row.ArchivedAt != nil {
		return document.Document{}, document.ErrDocumentNotFound
	}
	return store.row, nil
}

type exportActions []string

func (actions exportActions) EffectiveDocumentActionCodes(context.Context, uint, uint) ([]string, error) {
	return actions, nil
}

func TestExportRouteSavedSnapshotAndFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	all := exportActions{string(document.ActionScopeAll), string(document.ActionContentView), string(document.ActionContentExport)}
	now := time.Now()
	for _, test := range []struct {
		name      string
		path      string
		workspace uint
		identity  bool
		actions   exportActions
		archived  *time.Time
		status    int
	}{
		{"snapshot", "/11/export", 7, true, all, nil, 200},
		{"missing export", "/11/export", 7, true, all[:2], nil, 403},
		{"missing view", "/11/export", 7, true, exportActions{all[0], all[2]}, nil, 403},
		{"cross tenant", "/11/export", 8, true, all, nil, 404},
		{"missing document", "/99/export", 7, true, all, nil, 404},
		{"archived", "/11/export", 7, true, all, &now, 404},
		{"invalid id", "/bad/export", 7, true, all, nil, 400},
		{"no identity", "/11/export", 7, false, all, nil, 401},
		{"no workspace", "/11/export", 0, true, all, nil, 401},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := exportStore{row: document.Document{ID: 11, WorkspaceID: 7, NodeID: 2, Content: document.DefaultContent(), SchemaVersion: 2, Revision: 13, ArchivedAt: test.archived}}
			service, err := document.NewService(store, test.actions, contextRequirementResolver{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			handler, err := NewHandler(service)
			if err != nil {
				t.Fatal(err)
			}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				if test.identity {
					c.Set(pmauth.PrincipalContextKey, pmauth.Principal{UserID: 5})
				}
				c.Set(pmauth.WorkspaceIDContextKey, test.workspace)
			})
			handler.RegisterRoutes(router.Group("/api/v1/platform"))
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/platform/documents"+test.path, nil)
			// Untrusted headers cannot override middleware's authenticated workspace.
			request.Header.Set("X-Workspace-ID", "7")
			router.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if test.status == 200 {
				var payload struct {
					Data document.Document `json:"data"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
					t.Fatal(err)
				}
				if payload.Data.ID != 11 || payload.Data.Revision != 13 || payload.Data.SchemaVersion != 2 || string(payload.Data.Content) != string(store.row.Content) {
					t.Fatalf("snapshot=%+v", payload.Data)
				}
				if response.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("export must not be cached")
				}
			}
		})
	}
}
