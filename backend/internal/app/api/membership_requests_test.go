package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	notificationrecords "github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm/records"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	resourcegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
	"gorm.io/gorm"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMembershipRequestsWorkspaceThenOnlyOwningVersion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	records := []any{&identitygorm.User{}, &identitygorm.Workspace{}, &identitygorm.WorkspaceMember{}, &identitygorm.AccessRole{}, &identitygorm.AccessUserRole{}, &identitygorm.AccessServiceInterface{}, &identitygorm.AccessRoleServiceInterface{}, &identitygorm.AccessUserGroup{}, &identitygorm.AccessUserGroupBinding{}, &identitygorm.AccessGroupServiceInterface{}, &projectgorm.ProjectSet{}, &projectgorm.PlanUnit{}, &projectgorm.ProjectSetMembership{}, &projectgorm.PlanUnitParticipant{}, &projectgorm.PlanUnitMemberMutationAudit{}, &resourcegorm.AccessRequestRecord{}, &auditgorm.OutboxEvent{}, &notificationrecords.InboxReceipt{}}
	if err := db.AutoMigrate(records...); err != nil {
		t.Fatal(err)
	}
	owner := uint(9)
	seed := []any{
		&identitygorm.User{ID: 8, ExternalSubject: "applicant", Username: "applicant", Active: true, Status: "active"},
		&identitygorm.User{ID: 9, ExternalSubject: "owner", Username: "owner", Active: true, Status: "active"},
		&identitygorm.User{ID: 10, ExternalSubject: "other", Username: "other", Active: true, Status: "active"},
		&identitygorm.Workspace{ID: 2, Name: "Target", Slug: "target", Enabled: true},
		&identitygorm.Workspace{ID: 3, Name: "Other", Slug: "other", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 2, UserID: 9, Role: "owner", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 3, UserID: 10, Role: "owner", Enabled: true},
		&identitygorm.AccessRole{ID: 1, Code: "member-manager", Name: "Manager", Enabled: true},
		&identitygorm.AccessUserRole{UserID: 9, RoleID: 1}, &identitygorm.AccessUserRole{UserID: 10, RoleID: 1},
		&identitygorm.AccessServiceInterface{ID: 1, Code: "test.workspace.review", HTTPMethod: "PATCH", RouteTemplate: "/api/v1/platform/workspace-access-requests/:id/:requestId", ActionCode: string(accesscatalog.ActionAccessManagementWorkspacesMembersCreate), Name: "workspace", Enabled: true},
		&identitygorm.AccessServiceInterface{ID: 2, Code: "test.project.review", HTTPMethod: "PATCH", RouteTemplate: "/api/v1/platform/project-access-guide/:kind/:id/requests/:requestId", ActionCode: string(accesscatalog.ActionProjectsParticipantsManage), Name: "project", Enabled: true},
		&identitygorm.AccessRoleServiceInterface{RoleID: 1, ServiceInterfaceID: 1}, &identitygorm.AccessRoleServiceInterface{RoleID: 1, ServiceInterfaceID: 2},
		&projectgorm.ProjectSet{ID: 11, WorkspaceID: 2, TeamID: 1, Name: "Set", CreatedByUserID: &owner},
		&projectgorm.PlanUnit{ID: 21, WorkspaceID: 2, TeamID: 1, ProjectSetID: 11, Title: "v1", OwnerUserID: &owner},
		&projectgorm.PlanUnit{ID: 22, WorkspaceID: 2, TeamID: 1, ProjectSetID: 11, Title: "v2", OwnerUserID: &owner},
	}
	for _, row := range seed {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	call := func(user uint, method, path, body string, want int) map[string]any {
		t.Helper()
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(pmauth.PrincipalContextKey, pmauth.Principal{UserID: user})
			c.Set(pmauth.ActorFactUserIDContextKey, user)
			c.Next()
		})
		buildProjectAccessGuideRoutes(db)(router.Group("/api/v1/platform"))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/v1/platform"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s as %d: %d %s", method, path, user, w.Code, w.Body.String())
		}
		var response map[string]any
		json.Unmarshal(w.Body.Bytes(), &response)
		data, _ := response["data"].(map[string]any)
		return data
	}
	root := "/project-access-guide/plan_unit/21/requests"
	call(8, "POST", root, `{"reason":"join"}`, 403)
	requestContext := call(8, "GET", "/workspace-access-requests/2", "", 200)
	if requestContext["can_request"] != true || requestContext["can_review"] != false {
		t.Fatalf("scope %+v", requestContext)
	}
	request := call(8, "POST", "/workspace-access-requests/2", `{"reason":"join workspace"}`, 200)
	id := uint(request["id"].(float64))
	path := fmt.Sprintf("/workspace-access-requests/2/%d", id)
	call(8, "PATCH", path, `{"decision":"approved"}`, 403)
	call(10, "PATCH", path, `{"decision":"approved"}`, 403)
	call(9, "PATCH", path, `{"decision":"approved"}`, 200)
	call(9, "PATCH", path, `{"decision":"approved"}`, 200)
	var member identitygorm.WorkspaceMember
	if err := db.Where("workspace_id = 2 AND user_id = 8").First(&member).Error; err != nil || member.Role != "member" || !member.Enabled {
		t.Fatalf("membership %+v %v", member, err)
	}
	var count int64
	db.Model(&projectgorm.PlanUnitParticipant{}).Count(&count)
	if count != 0 {
		t.Fatal("workspace approval added project")
	}
	request = call(8, "POST", root, `{"reason":"join v1"}`, 200)
	id = uint(request["id"].(float64))
	call(10, "PATCH", fmt.Sprintf("%s/%d", root, id), `{"decision":"approved"}`, 403)
	call(9, "PATCH", fmt.Sprintf("/project-access-guide/plan_unit/22/requests/%d", id), `{"decision":"approved"}`, 404)
	call(9, "PATCH", fmt.Sprintf("%s/%d", root, id), `{"decision":"approved"}`, 200)
	var participant projectgorm.PlanUnitParticipant
	if err := db.First(&participant).Error; err != nil || participant.UserID != 8 || participant.PlanUnitID != 21 || participant.WorkspaceID != 2 {
		t.Fatalf("participant %+v %v", participant, err)
	}
	db.Model(&projectgorm.PlanUnitParticipant{}).Count(&count)
	if count != 1 {
		t.Fatal("granted other version")
	}
	db.Model(&identitygorm.AccessUserRole{}).Where("user_id = 8").Count(&count)
	if count != 0 {
		t.Fatal("granted business role")
	}
	db.Model(&auditgorm.OutboxEvent{}).Count(&count)
	if count != 4 {
		t.Fatalf("outbox retries duplicated: %d", count)
	}
	db.Model(&notificationrecords.InboxReceipt{}).Where("category = ?", "access").Count(&count)
	if count != 4 {
		t.Fatalf("inbox missing %d", count)
	}
	requestContext = call(8, "GET", root, "", 200)
	if requestContext["has_access"] != true || requestContext["can_request"] != false {
		t.Fatalf("approved state %+v", requestContext)
	}
	details := call(8, "GET", "/project-access-guide/plan_unit/21", "", 200)
	states := details["memberships"].([]any)
	if len(states) != 2 || states[0].(map[string]any)["status"] != "joined" || states[1].(map[string]any)["status"] != "joined" {
		t.Fatalf("aggregate states %+v", states)
	}
	setRoot := "/project-access-guide/project_set/11/requests"
	call(8, "GET", setRoot, "", 400)
	call(8, "POST", setRoot, `{"reason":"all versions"}`, 400)
	call(9, "PATCH", setRoot+"/1", `{"decision":"approved"}`, 400)
	// Even legacy rows must not grant access after project-set membership retirement.
	if err := db.Create(&projectgorm.ProjectSetMembership{WorkspaceID: 2, ProjectSetID: 11, UserID: 8, CreatedByUserID: 9}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("workspace_id = ? AND plan_unit_id = ? AND user_id = ?", 2, 21, 8).Delete(&projectgorm.PlanUnitParticipant{}).Error; err != nil {
		t.Fatal(err)
	}
	access, _ := newProjectAccessService(db)
	for _, unit := range []uint{21, 22} {
		allowed, err := access.CanAccessPlanUnit(context.Background(), 2, unit, 8)
		if err != nil || allowed {
			t.Fatalf("removed member retains access %d: %v %v", unit, allowed, err)
		}
	}
	filter, err := access.BuildMemberFilter(context.Background(), "plan_units", "id", 2, 8)
	if err != nil {
		t.Fatal(err)
	}
	db.Model(&projectgorm.PlanUnit{}).Where("workspace_id = ?", 2).Where(filter.SQL, filter.Args...).Count(&count)
	if count != 0 {
		t.Fatalf("removed member visible in list: %d", count)
	}
	details = call(8, "GET", "/project-access-guide/plan_unit/21", "", 200)
	if details["memberships"].([]any)[1].(map[string]any)["status"] != "requestable" {
		t.Fatalf("removed member cannot reapply: %+v", details)
	}
}
