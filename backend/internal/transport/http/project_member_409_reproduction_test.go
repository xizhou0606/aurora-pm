package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

// Reproduce the reported roster replacement using synthetic local records.
// User 41 still owns open tasks; adding other members must not remove them.
func TestPlanUnitMembers409ReportedRoster(t *testing.T) {
	t.Parallel()
	app := newTestAppWithProjectAccess(t, nil)
	owner := app.createUser(t, 45, "roster-owner", "super_admin", "")
	for _, id := range []uint{2, 40, 44, 30, 42, 41, 90, 32, 24, 53} {
		app.createUser(t, id, fmt.Sprintf("roster-user-%d", id), "user", "")
	}
	for _, row := range []any{
		&identitygorm.Workspace{ID: 2, Name: "Roster reproduction", Slug: "roster-repro", Enabled: true},
		&identitygorm.Team{ID: 18, WorkspaceID: 2, Name: "Roster team"},
		&projectgorm.ProjectSet{ID: 18, WorkspaceID: 2, TeamID: 18, Name: "Roster set", CreatedByUserID: &owner.ID, Status: "active"},
		&projectgorm.PlanUnit{ID: 59, WorkspaceID: 2, TeamID: 18, ProjectSetID: 18, Title: "Roster unit", OwnerUserID: &owner.ID, Status: "planning"},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []uint{45, 2, 40, 44, 30, 42, 41, 90, 32, 24, 53} {
		if err := app.db.Create(&identitygorm.WorkspaceMember{WorkspaceID: 2, UserID: id, Role: "member", Enabled: true}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []uint{2, 40, 44, 30, 42, 41, 90} {
		if err := app.db.Create(&projectgorm.PlanUnitParticipant{WorkspaceID: 2, PlanUnitID: 59, UserID: id, CreatedByUserID: 45}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 45; i++ {
		if err := app.db.Create(&taskgorm.WorkTask{WorkspaceID: 2, TeamID: 18, ProjectSetID: 18, ProjectID: 59,
			Kind: "task", TaskKey: fmt.Sprintf("REPRO-%d", i+1), Title: "Synthetic responsibility", Status: "in_dev", AssigneeID: ptrUint(41)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := app.token(t, owner)
	put := func(ids []uint) int {
		t.Helper()
		response := app.request(t, http.MethodPut, "/api/v1/platform/project-new/59/members", token, 2,
			map[string]any{"owner_user_id": 45, "user_ids": ids, "reason": "via project member editor"})
		if response.Code == http.StatusConflict && !strings.Contains(response.Body.String(), "participant 41 has outstanding responsibilities: 10 item(s): 45 open tasks") {
			t.Fatalf("unexpected conflict: %s", response.Body.String())
		}
		return response.Code
	}
	ids := []uint{2, 44, 32, 30, 24, 42, 90, 53}
	if status := put(ids); status != http.StatusConflict {
		t.Fatalf("reported roster status=%d, want 409", status)
	}
	var count int64
	if err := app.db.Model(&projectgorm.PlanUnitParticipant{}).Where("workspace_id = ? AND plan_unit_id = ?", 2, 59).Count(&count).Error; err != nil || count != 7 {
		t.Fatalf("rejected write changed roster: count=%d err=%v", count, err)
	}
	if status := put(append(ids, 41)); status != http.StatusOK {
		t.Fatalf("retaining responsible member status=%d, want 200", status)
	}
}
