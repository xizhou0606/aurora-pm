package documenthttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	documentgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/document/gorm"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type contextActionResolver struct{}

func (contextActionResolver) EffectiveDocumentActionCodes(context.Context, uint, uint) ([]string, error) {
	return []string{
		string(document.ActionScopeAll), string(document.ActionNodesView), string(document.ActionNodesCreate),
		string(document.ActionNodesCopy), string(document.ActionShortcutsCreate),
		string(document.ActionPreferencesUpdate), string(document.ActionOwnershipTransfer),
		string(document.ActionContentView),
	}, nil
}

type contextRequirementResolver struct{}

func (contextRequirementResolver) ResolveRequirementParticipants(context.Context, uint, uint) (document.RequirementParticipants, error) {
	return document.RequirementParticipants{}, document.ErrRequirementUnavailable
}

type contextEffectRecorder struct{}

func (contextEffectRecorder) RecordDocumentMutation(context.Context, document.MutationEffect) error {
	return nil
}

type contextWorkspaceUsers struct{}

func (contextWorkspaceUsers) ValidateActiveWorkspaceUser(context.Context, uint, uint) error {
	return nil
}

func TestContextActionRoutesBindCanonicalDTOs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(documentgorm.MigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	repository := documentgorm.NewRepository(db)
	repository.ConfigureMutationRecorderFactory(func(*gorm.DB) (document.MutationRecorder, error) {
		return contextEffectRecorder{}, nil
	})
	service, err := document.NewService(repository, contextActionResolver{}, contextRequirementResolver{}, contextEffectRecorder{})
	if err != nil {
		t.Fatal(err)
	}
	service.ConfigureWorkspaceUsers(contextWorkspaceUsers{})
	principal := document.Principal{UserID: 5, WorkspaceID: 7}
	created, err := service.CreateNode(context.Background(), principal, document.CreateNodeInput{Type: document.NodeDocument, Title: "ADR"})
	if err != nil || created.Document == nil {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	handler, err := NewHandler(service)
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	group := engine.Group("/api/v1/platform", func(c *gin.Context) {
		c.Set(pmauth.PrincipalContextKey, pmauth.Principal{UserID: 5})
		c.Set(pmauth.WorkspaceIDContextKey, uint(7))
		c.Set(pmauth.ActorFactUserIDContextKey, uint(5))
	})
	handler.RegisterRoutes(group)

	requests := []struct {
		method string
		path   string
		body   string
		status int
	}{
		{http.MethodPost, "/api/v1/platform/document-nodes/1/copies", `{}`, http.StatusCreated},
		{http.MethodPost, "/api/v1/platform/document-nodes/1/shortcuts", `{}`, http.StatusCreated},
		{http.MethodPut, "/api/v1/platform/document-nodes/1/preferences", `{"pinned":true,"favorited":true}`, http.StatusOK},
		{http.MethodPut, "/api/v1/platform/documents/1/owner", `{"owner_user_id":9}`, http.StatusOK},
	}
	for _, item := range requests {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(item.method, item.path, bytes.NewBufferString(item.body))
		request.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(recorder, request)
		if recorder.Code != item.status {
			t.Fatalf("%s %s status=%d body=%s", item.method, item.path, recorder.Code, recorder.Body.String())
		}
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil || len(envelope.Data) == 0 {
			t.Fatalf("%s %s envelope=%s err=%v", item.method, item.path, recorder.Body.String(), err)
		}
	}
}
