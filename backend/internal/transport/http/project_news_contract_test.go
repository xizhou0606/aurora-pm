package httpapi

import (
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"net/http"
	"testing"
	"time"
)

func TestProjectNewsSupportsBoundedLatestReadsWithoutChangingFullListCompatibility(t *testing.T) {
	app := newTestApp(t, nil)
	viewer := app.createUser(t, 1, "viewer", "user", "")
	workspace := identitygorm.Workspace{
		ID: 1, Name: "Project news", Slug: "project-news", Enabled: true,
	}
	team := identitygorm.Team{ID: 10, WorkspaceID: workspace.ID, Name: "Platform"}
	project := projectgorm.ProjectSet{
		ID: 20, WorkspaceID: workspace.ID, TeamID: team.ID, Name: "PM", Status: "active",
	}
	otherProject := projectgorm.ProjectSet{
		ID: 21, WorkspaceID: workspace.ID, TeamID: team.ID, Name: "Other", Status: "active",
	}
	for _, row := range []any{
		&workspace,
		&identitygorm.WorkspaceMember{
			WorkspaceID: workspace.ID, UserID: viewer.ID, Role: "member", Enabled: true,
		},
		&team,
		&project,
		&otherProject,
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	newestTime := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	rows := []projectgorm.ProjectNews{
		{ID: 101, WorkspaceID: workspace.ID, ProjectSetID: project.ID, UserID: viewer.ID,
			Title: "same-time lower id", CreatedAt: newestTime, UpdatedAt: newestTime},
		{ID: 102, WorkspaceID: workspace.ID, ProjectSetID: project.ID, UserID: viewer.ID,
			Title: "same-time higher id", CreatedAt: newestTime, UpdatedAt: newestTime},
		{ID: 103, WorkspaceID: workspace.ID, ProjectSetID: project.ID, UserID: viewer.ID,
			Title: "older", CreatedAt: newestTime.Add(-time.Hour), UpdatedAt: newestTime.Add(-time.Hour)},
		{ID: 201, WorkspaceID: workspace.ID, ProjectSetID: otherProject.ID, UserID: viewer.ID,
			Title: "other project", CreatedAt: newestTime.Add(time.Hour), UpdatedAt: newestTime.Add(time.Hour)},
	}
	for index := range rows {
		if err := app.db.Create(&rows[index]).Error; err != nil {
			t.Fatal(err)
		}
	}

	token := app.token(t, viewer)
	basePath := "/api/v1/platform/project-sets/20/news"
	limited := app.request(t, http.MethodGet, basePath+"?limit=1", token, workspace.ID, nil)
	requireStatus(t, limited, http.StatusOK)
	assertProjectNewsIDs(t, decodeData(t, limited), []uint{102})

	full := app.request(t, http.MethodGet, basePath, token, workspace.ID, nil)
	requireStatus(t, full, http.StatusOK)
	assertProjectNewsIDs(t, decodeData(t, full), []uint{102, 101, 103})
}

func assertProjectNewsIDs(t *testing.T, raw any, want []uint) {
	t.Helper()
	rows, ok := raw.([]any)
	if !ok || len(rows) != len(want) {
		t.Fatalf("project news=%v want ids=%v", raw, want)
	}
	for index, id := range want {
		row, ok := rows[index].(map[string]any)
		if !ok || row["id"] != float64(id) {
			t.Fatalf("project news[%d]=%v want id=%d", index, rows[index], id)
		}
	}
}
