package api

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	projecthttp "github.com/xizhou0606/aurora-pm/backend/internal/module/project/http"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
	"gorm.io/gorm"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectAccessGuideSharesOnlyApplicationMetadata(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&identitygorm.User{}, &identitygorm.Workspace{}, &projectgorm.ProjectSet{}, &projectgorm.PlanUnit{}, &taskgorm.WorkTask{}, &qualitygorm.QualityBug{}); err != nil {
		t.Fatal(err)
	}
	owner := uint(47)
	for _, row := range []any{
		&identitygorm.User{ID: 47, Username: "owner", DisplayName: "Project Owner", Active: true},
		&identitygorm.Workspace{ID: 2, Name: "Target Workspace", Slug: "target", Enabled: true},
		&projectgorm.ProjectSet{ID: 11, WorkspaceID: 2, Name: "Target Project", OwnerUserID: &owner},
		&projectgorm.PlanUnit{ID: 21, WorkspaceID: 2, ProjectSetID: 11, Title: "Version", OwnerUserID: &owner},
		&taskgorm.WorkTask{ID: 31, WorkspaceID: 2, ProjectSetID: 11, ProjectID: 21, Title: "SECRET_TASK", Description: "SECRET_BODY"},
		&qualitygorm.QualityBug{ID: 32, WorkspaceID: 2, ProjectSetID: 11, ProjectID: 21, Title: "SECRET_BUG"},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(pmauth.PrincipalContextKey, pmauth.Principal{UserID: 47})
		c.Set(pmauth.ActorFactUserIDContextKey, uint(47))
		c.Next()
	})
	handler := projecthttp.NewAccessGuideHandler(projectGuideService(db))
	handler.ConfigureRequests(&project.JoinService{})
	handler.ConfigureMemberships(guideTestMemberships{})
	handler.RegisterRoutes(router.Group("/api/v1/platform"))
	for _, path := range []string{"task/31", "quality_bug/32"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/platform/project-access-guide/"+path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		var envelope struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if len(envelope.Data) != 8 || envelope.Data["workspace_name"] != "Target Workspace" || envelope.Data["owner_name"] != "Project Owner" || strings.Contains(w.Body.String(), "SECRET") {
			t.Fatalf("unexpected disclosure: %s", w.Body.String())
		}
	}
	for _, path := range []string{"task/999", "quality_bug/999", "unknown/31"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/platform/project-access-guide/"+path, nil))
		if w.Code != 404 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	if err := db.Model(&identitygorm.Workspace{}).Where("id = ?", 2).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/platform/project-access-guide/task/31", nil))
	if w.Code != 404 {
		t.Fatalf("disabled workspace: %d", w.Code)
	}
}

type guideTestMemberships struct{}

func (guideTestMemberships) GuideMemberships(context.Context, project.JoinActor, project.AccessGuide) ([]project.GuideMembership, error) {
	return []project.GuideMembership{}, nil
}
