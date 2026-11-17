package httpapi

import (
	"github.com/gin-gonic/gin"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"testing"
)

func TestRemovedWorkspaceMemberKeepsReferencedNames(t *testing.T) {
	app := newTestApp(t, nil)
	former := app.createUser(t, 110, "former-member", "user", "")
	other := app.createUser(t, 111, "unreferenced", "user", "")
	if err := app.db.Create(&identitygorm.Workspace{ID: 7, Name: "test", Slug: "test", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	member := identitygorm.WorkspaceMember{WorkspaceID: 7, UserID: former.ID, Enabled: true, Role: "member"}
	if err := app.db.Create(&member).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&former).Update("external_user_id", nil).Error; err != nil {
		t.Fatal(err)
	}
	id := factUserID(former)
	task := taskgorm.WorkTask{ID: 10, WorkspaceID: 7, AssigneeID: &id, ReporterUserID: &id}
	if err := app.db.Where("workspace_id = ? AND user_id = ?", 7, former.ID).Delete(&identitygorm.WorkspaceMember{}).Error; err != nil {
		t.Fatal(err)
	}
	server := &Server{db: app.db}
	people := server.loadWorkspaceContext(7, workspaceContextReferences{}.withTask(task))
	row := people.enrichTask(gin.H{}, task)
	if row["assignee_name"] != former.DisplayName || row["reporter_name"] != former.DisplayName {
		t.Fatalf("lost history: %v", row)
	}
	if _, ok := people.Users[factUserID(other)]; ok {
		t.Fatal("loaded unrelated user")
	}
	rows := server.projectDTORows(&gin.Context{}, []projectgorm.ProjectSet{{ID: 20, WorkspaceID: 7, OwnerUserID: &id, CreatedByUserID: &id}})
	if rows[0]["owner_name"] != former.DisplayName || rows[0]["created_by_name"] != former.DisplayName {
		t.Fatalf("project lost history: %v", rows)
	}
	var count int64
	app.db.Model(&identitygorm.WorkspaceMember{}).Where("workspace_id = ? AND user_id = ?", 7, former.ID).Count(&count)
	if count != 0 {
		t.Fatal("display restored membership")
	}
}
