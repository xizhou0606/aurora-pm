package httpapi

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
)

// These tests cover the shared-identity regime where users.external_user_id
// differs from users.id: membership facts (project_sets.created_by_user_id,
// plan_units.owner_user_id, plan_unit_participants.user_id) store the PM internal ID,
// and external import identifiers must not affect any access decision. The apps below use the REAL AccessService so
// the creator, unit-owner, unit-participant, admin, and outsider branches run
// against production SQL.

const (
	sharedWSID        = uint(2100)
	sharedTeamID      = uint(2210)
	projOwned         = uint(2110) // creator: ext-owner (internal 101)
	projShared        = uint(2111) // creator: stranger; unit participant: ext-participant (internal 102)
	projHidden        = uint(2112) // creator: stranger; nobody here can access it
	unitOwned         = uint(2211)
	unitShared        = uint(2212)
	unitHidden        = uint(2213)
	unitSharedSibling = uint(2214)
	reqOwned          = uint(2311)
	reqShared         = uint(2312)
	reqHidden         = uint(2313)
	ownerUserID       = uint(101)
	participantUserID = uint(102)
	outsiderUserID    = uint(103)
	adminUserID       = uint(104)
	strangerUserID    = uint(199)
)

type sharedIdentityFixture struct {
	*testApp
	ownerToken    string
	partToken     string
	outsiderToken string
	adminToken    string
}

// newSharedIdentityApp builds a test app wired with the production
// AccessService (real PlanUnitAccessRepository + AdminRoleChecker) so the
// internal-ID resolution runs through the same SQL as production.
func newSharedIdentityApp(t *testing.T) *sharedIdentityFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		Environment: "test", HTTPAddr: ":0", DatabaseDriver: "sqlite",
		DatabaseDSN:    fmt.Sprintf("file:pm_shared_id_%d?mode=memory&cache=shared", testDBCounter.Add(1)),
		AuthMode:       "local",
		LocalJWTSecret: "pm-test-local-secret-that-is-at-least-thirty-two-characters",
		TokenTTL:       3600 * 1e9,
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	memberRepo := projectgorm.NewPlanUnitMemberRepository(db)
	impactRepo := projectgorm.NewRemovalImpactRepository(db)
	adminChecker := projectgorm.NewAdminRoleChecker(db)
	unitAccess := projectgorm.NewPlanUnitAccessRepository(db)
	access, err := project.NewAccessService(
		adminChecker,
		adminChecker,
		unitAccess,
		unitAccess,
		unitAccess,
	)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewWithDependencies(cfg, db, Dependencies{
		ProjectAccess:           access,
		MemberFacts:             memberRepo,
		ResponsibilityInspector: impactRepo,
	})
	app := &testApp{engine: engine, db: db, cfg: cfg, tokens: pmauth.NewTokens(cfg), projectAccess: access}

	// Shared-identity users: internal users.id (101..104) deliberately differ
	// from their external fact IDs (501..504). PM-only users (stranger) keep
	// external_user_id = 0 so their fact ID equals users.id.
	users := []identitygorm.User{
		{ID: 101, ExternalSubject: "local:101", Username: "ext-owner", DisplayName: "Ext Owner", Role: "user", Active: true},
		{ID: 102, ExternalSubject: "local:102", Username: "ext-part", DisplayName: "Ext Participant", Role: "user", Active: true},
		{ID: 103, ExternalSubject: "local:103", Username: "outsider", DisplayName: "Outsider", Role: "user", Active: true},
		{ID: 104, ExternalSubject: "local:104", Username: "access-admin", DisplayName: "Access Admin", Role: "user", Active: true},
		{ID: 199, ExternalSubject: "local:199", Username: "stranger", DisplayName: "Stranger", Role: "user", Active: true},
		{ID: 120, ExternalSubject: "local:120", Username: "candidate", DisplayName: "Member Candidate", Role: "user", Active: true},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	// access-administrator role + binding. AccessUserRole stores users.id.
	role := identitygorm.AccessRole{ID: 30, Code: "access-administrator", Name: "Access Administrator", Enabled: true}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&identitygorm.AccessUserRole{UserID: 104, RoleID: role.ID}).Error; err != nil {
		t.Fatal(err)
	}

	for _, row := range []any{
		&identitygorm.Workspace{ID: sharedWSID, Name: "SharedWS", Slug: "sharedws", Enabled: true},
		&identitygorm.Team{ID: sharedTeamID, WorkspaceID: sharedWSID, Name: "SharedTeam"},
		&identitygorm.WorkspaceMember{WorkspaceID: sharedWSID, UserID: 101, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: sharedWSID, UserID: 102, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: sharedWSID, UserID: 103, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: sharedWSID, UserID: 104, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: sharedWSID, UserID: 120, Role: "member", Enabled: true},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	strangerOwner := strangerUserID
	ownerID := ownerUserID
	projects := []projectgorm.ProjectSet{
		{ID: projOwned, WorkspaceID: sharedWSID, TeamID: sharedTeamID, Name: "Owned", CreatedByUserID: &ownerID, OwnerUserID: &ownerID, Status: "active"},
		{ID: projShared, WorkspaceID: sharedWSID, TeamID: sharedTeamID, Name: "Participated", CreatedByUserID: &strangerOwner, OwnerUserID: &strangerOwner, Status: "active"},
		{ID: projHidden, WorkspaceID: sharedWSID, TeamID: sharedTeamID, Name: "Hidden", CreatedByUserID: &strangerOwner, OwnerUserID: &strangerOwner, Status: "active"},
	}
	for i := range projects {
		if err := db.Create(&projects[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	unitOwner := ownerUserID
	sharedUnitOwner := strangerUserID
	units := []projectgorm.PlanUnit{
		{ID: unitOwned, WorkspaceID: sharedWSID, TeamID: sharedTeamID, ProjectSetID: projOwned, Title: "Owned V1", OwnerUserID: &unitOwner, Status: "active"},
		{ID: unitShared, WorkspaceID: sharedWSID, TeamID: sharedTeamID, ProjectSetID: projShared, Title: "Shared V1", OwnerUserID: &sharedUnitOwner, Status: "active"},
		// A sibling unit in the SAME accessible set but with no role for the
		// participant: visible to the creator/admin, never to the participant.
		{ID: unitSharedSibling, WorkspaceID: sharedWSID, TeamID: sharedTeamID, ProjectSetID: projShared, Title: "Shared V2 sibling", OwnerUserID: &sharedUnitOwner, Status: "active"},
		{ID: unitHidden, WorkspaceID: sharedWSID, TeamID: sharedTeamID, ProjectSetID: projHidden, Title: "Hidden V1", OwnerUserID: &sharedUnitOwner, Status: "active"},
	}
	for i := range units {
		if err := db.Create(&units[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	requirements := []requirementgorm.Requirement{
		{ID: reqOwned, WorkspaceID: sharedWSID, TeamID: sharedTeamID, ProjectSetID: projOwned, PlanUnitID: unitOwned, Title: "Owned requirement secret", Status: "draft"},
		{ID: reqShared, WorkspaceID: sharedWSID, TeamID: sharedTeamID, ProjectSetID: projShared, PlanUnitID: unitShared, Title: "Shared requirement", Status: "draft"},
		{ID: reqHidden, WorkspaceID: sharedWSID, TeamID: sharedTeamID, ProjectSetID: projHidden, PlanUnitID: unitHidden, Title: "Hidden requirement secret", Status: "draft"},
	}
	for i := range requirements {
		if err := db.Create(&requirements[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	// The participant row stores the FACT ID (502), not users.id (102).
	if err := db.Create(&projectgorm.PlanUnitParticipant{
		WorkspaceID: sharedWSID, PlanUnitID: unitShared, UserID: participantUserID, CreatedByUserID: ownerUserID,
	}).Error; err != nil {
		t.Fatal(err)
	}

	return &sharedIdentityFixture{
		testApp:       app,
		ownerToken:    app.token(t, users[0]),
		partToken:     app.token(t, users[1]),
		outsiderToken: app.token(t, users[2]),
		adminToken:    app.token(t, users[3]),
	}
}

// TestSharedIdentityProjectListFiltering proves creator/unit-owner/unit-
// participant/admin see their projects while an unrelated workspace member
// sees none — all through PM-internal IDs, regardless of external bindings.
func TestSharedIdentityProjectListFiltering(t *testing.T) {
	fx := newSharedIdentityApp(t)

	listIDs := func(token string) []uint {
		rec := fx.request(t, http.MethodGet, "/api/v1/platform/project-sets", token, sharedWSID, nil)
		requireStatus(t, rec, http.StatusOK)
		data := decodeData(t, rec).([]any)
		ids := make([]uint, 0, len(data))
		for _, row := range data {
			ids = append(ids, uint(row.(map[string]any)["id"].(float64)))
		}
		return ids
	}

	if got := listIDs(fx.ownerToken); !equalUintSlice(got, []uint{projOwned}) {
		t.Fatalf("creator sees %v, want [%d]: external_user_id != users.id broke the creator gate", got, projOwned)
	}
	if got := listIDs(fx.partToken); !equalUintSlice(got, []uint{projShared}) {
		t.Fatalf("unit participant sees %v, want [%d]", got, projShared)
	}
	if got := listIDs(fx.outsiderToken); len(got) != 0 {
		t.Fatalf("outsider sees %v, want none", got)
	}
	if got := listIDs(fx.adminToken); !equalUintSlice(got, []uint{projHidden, projShared, projOwned}) {
		t.Fatalf("admin sees %v, want all three", got)
	}
}

// TestSharedIdentityDirectLink404 proves an inaccessible project and its
// sub-resources are indistinguishable from missing ones.
func TestSharedIdentityDirectLink404(t *testing.T) {
	fx := newSharedIdentityApp(t)

	paths := []string{
		"/api/v1/platform/project-sets/" + fmt.Sprint(projHidden),
		"/api/v1/platform/product-requirements-new/" + fmt.Sprint(reqHidden),
		"/api/v1/platform/project-new/" + fmt.Sprint(unitHidden),
		"/api/v1/platform/project-new/" + fmt.Sprint(unitHidden) + "/members",
	}
	for _, path := range paths {
		rec := fx.request(t, http.MethodGet, path, fx.partToken, sharedWSID, nil)
		requireStatus(t, rec, http.StatusNotFound)
	}

	// The creator can reach their own project's detail and requirement.
	if rec := fx.request(t, http.MethodGet, "/api/v1/platform/project-sets/"+fmt.Sprint(projOwned), fx.ownerToken, sharedWSID, nil); rec.Code != http.StatusOK {
		t.Fatalf("creator project detail status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := fx.request(t, http.MethodGet, "/api/v1/platform/product-requirements-new/"+fmt.Sprint(reqOwned), fx.ownerToken, sharedWSID, nil); rec.Code != http.StatusOK {
		t.Fatalf("creator requirement detail status=%d body=%s", rec.Code, rec.Body.String())
	}
}

// TestSharedIdentityScopeOptionsFiltering proves the workspace-wide cascader
// only offers accessible projects and children under shared identity.
func TestSharedIdentityScopeOptionsFiltering(t *testing.T) {
	fx := newSharedIdentityApp(t)

	scopeFor := func(token string) map[string]any {
		rec := fx.request(t, http.MethodGet, "/api/v1/platform/project-requirement-scope-options", token, sharedWSID, nil)
		requireStatus(t, rec, http.StatusOK)
		return decodeData(t, rec).(map[string]any)
	}

	owner := scopeFor(fx.ownerToken)
	assertJSONRowIDs(t, owner["project_sets"], []uint{projOwned})
	assertJSONRowIDs(t, owner["projects"], []uint{unitOwned})
	assertJSONRowIDs(t, owner["requirements"], []uint{reqOwned})

	part := scopeFor(fx.partToken)
	assertJSONRowIDs(t, part["project_sets"], []uint{projShared})
	assertJSONRowIDs(t, part["projects"], []uint{unitShared})
	assertJSONRowIDs(t, part["requirements"], []uint{reqShared})

	outsider := scopeFor(fx.outsiderToken)
	assertJSONRowIDs(t, outsider["project_sets"], []uint{})
	assertJSONRowIDs(t, outsider["projects"], []uint{})
	assertJSONRowIDs(t, outsider["requirements"], []uint{})

	admin := scopeFor(fx.adminToken)
	assertJSONRowIDs(t, admin["project_sets"], []uint{projHidden, projShared, projOwned})
	assertJSONRowIDs(t, admin["projects"], []uint{unitOwned, unitSharedSibling, unitShared, unitHidden})
	assertJSONRowIDs(t, admin["requirements"], []uint{reqOwned, reqShared, reqHidden})
}

// TestSharedIdentityPlanUnitListScopedToOwnUnits proves the version list
// inside an accessible project set is scoped to the caller's own units: a
// unit participant never enumerates sibling units they hold no role on.
func TestSharedIdentityPlanUnitListScopedToOwnUnits(t *testing.T) {
	fx := newSharedIdentityApp(t)

	unitsFor := func(token string, projectSetID uint) []uint {
		rec := fx.request(t, http.MethodGet,
			"/api/v1/platform/project-new?project_new_id="+fmt.Sprint(projectSetID),
			token, sharedWSID, nil)
		requireStatus(t, rec, http.StatusOK)
		rows := decodeData(t, rec).([]any)
		ids := make([]uint, 0, len(rows))
		for _, raw := range rows {
			ids = append(ids, uint(raw.(map[string]any)["id"].(float64)))
		}
		return ids
	}

	// The participant of unitShared sees only their unit, not the sibling.
	if got := unitsFor(fx.partToken, projShared); !equalUintSlice(got, []uint{unitShared}) {
		t.Fatalf("unit participant sees %v, want [%d] only", got, unitShared)
	}
	// The set creator sees every unit of their set.
	if got := unitsFor(fx.ownerToken, projOwned); !equalUintSlice(got, []uint{unitOwned}) {
		t.Fatalf("creator sees %v, want [%d]", got, unitOwned)
	}
	// The admin sees all units of the shared set.
	if got := unitsFor(fx.adminToken, projShared); !equalUintSlice(got, []uint{unitSharedSibling, unitShared}) {
		t.Fatalf("admin sees %v, want [%d %d]", got, unitSharedSibling, unitShared)
	}
}

// TestSharedIdentityRequirementKeywordSearchNoLeak proves a keyword-only
// requirement search never returns requirements from inaccessible projects.
func TestSharedIdentityRequirementKeywordSearchNoLeak(t *testing.T) {
	fx := newSharedIdentityApp(t)

	search := func(token, keyword string) []any {
		rec := fx.request(t, http.MethodGet, "/api/v1/platform/product-requirements-new?q="+keyword, token, sharedWSID, nil)
		requireStatus(t, rec, http.StatusOK)
		return decodeData(t, rec).([]any)
	}

	// The secret keyword only exists in the hidden project's requirement.
	rows := search(fx.partToken, "secret")
	if len(rows) != 0 {
		t.Fatalf("participant keyword search leaked %d hidden requirement(s)", len(rows))
	}
	rows = search(fx.outsiderToken, "requirement")
	if len(rows) != 0 {
		t.Fatalf("outsider keyword search leaked %d requirement(s)", len(rows))
	}
	rows = search(fx.ownerToken, "secret")
	if len(rows) != 1 {
		t.Fatalf("creator keyword search returned %d rows, want 1", len(rows))
	}
	rows = search(fx.adminToken, "secret")
	if len(rows) != 2 {
		t.Fatalf("admin keyword search returned %d rows, want 2", len(rows))
	}
}

// TestSharedIdentityPlanUnitManageFlag proves the plan unit member-management
// flag: set creator, unit owner, and access-administrator true; plain unit
// participant false.
func TestSharedIdentityPlanUnitManageFlag(t *testing.T) {
	fx := newSharedIdentityApp(t)

	flagFor := func(token string, unitID uint) bool {
		rec := fx.request(t, http.MethodGet, "/api/v1/platform/project-new/"+fmt.Sprint(unitID), token, sharedWSID, nil)
		requireStatus(t, rec, http.StatusOK)
		data := decodeData(t, rec).(map[string]any)
		return data["can_manage_participants"].(bool)
	}

	if !flagFor(fx.ownerToken, unitOwned) {
		t.Fatal("set creator must manage unit members")
	}
	if !flagFor(fx.adminToken, unitShared) {
		t.Fatal("access-administrator must manage unit members")
	}
	if flagFor(fx.partToken, unitShared) {
		t.Fatal("plain unit participant must NOT manage unit members")
	}
}

// TestSharedIdentityMemberOptionsSearchesNonMembers proves member-options
// enumerates workspace users who have NOT joined the unit, and that a plain
// participant is rejected.
func TestSharedIdentityMemberOptionsSearchesNonMembers(t *testing.T) {
	fx := newSharedIdentityApp(t)

	// The candidate (users.id 120, no external ID) never joined any unit.
	rec := fx.request(t, http.MethodGet,
		"/api/v1/platform/project-new/"+fmt.Sprint(unitOwned)+"/member-options?q=candidate",
		fx.ownerToken, sharedWSID, nil)
	requireStatus(t, rec, http.StatusOK)
	items := decodeData(t, rec).(map[string]any)["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("member-options returned %d items, want the un-joined candidate", len(items))
	}
	row := items[0].(map[string]any)
	// Canonical fact ID: users.id because external_user_id = 0.
	if row["id"].(float64) != float64(120) {
		t.Fatalf("candidate id=%v, want 120 (fact ID)", row["id"])
	}

	// A plain unit participant may not enumerate member options at all.
	rec = fx.request(t, http.MethodGet,
		"/api/v1/platform/project-new/"+fmt.Sprint(unitShared)+"/member-options",
		fx.partToken, sharedWSID, nil)
	requireStatus(t, rec, http.StatusForbidden)
}

// TestSharedIdentityMemberOptionsReturnsInternalIDs proves the member search
// returns canonical internal IDs — never the external
// users.id that would render as "用户 33".
func TestSharedIdentityMemberOptionsReturnsInternalIDs(t *testing.T) {
	fx := newSharedIdentityApp(t)

	rec := fx.request(t, http.MethodGet,
		"/api/v1/platform/project-new/"+fmt.Sprint(unitShared)+"/member-options?q=ext-part",
		fx.adminToken, sharedWSID, nil)
	requireStatus(t, rec, http.StatusOK)
	items := decodeData(t, rec).(map[string]any)["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("search returned %d items, want 1", len(items))
	}
	row := items[0].(map[string]any)
	if row["id"].(float64) != float64(participantUserID) {
		t.Fatalf("member id=%v, want internal ID %d, never external_user_id 502", row["id"], participantUserID)
	}
	if row["display_name"] != "Ext Participant" {
		t.Fatalf("display_name=%v", row["display_name"])
	}
}

// TestSharedIdentityPatchOwnerForbidden proves a plain PATCH cannot move
// owner_user_id; creator changes only go through the transfer route.
func TestSharedIdentityPatchOwnerForbidden(t *testing.T) {
	fx := newSharedIdentityApp(t)

	rec := fx.request(t, http.MethodPatch,
		"/api/v1/platform/project-sets/"+fmt.Sprint(projOwned),
		fx.ownerToken, sharedWSID, map[string]any{"owner_user_id": participantUserID})
	requireStatus(t, rec, http.StatusBadRequest)

	// The creator survived untouched.
	var row projectgorm.ProjectSet
	if err := fx.db.Where("id = ?", projOwned).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.CreatedByUserID == nil || *row.CreatedByUserID != ownerUserID {
		t.Fatalf("created_by_user_id changed via PATCH: %v", row.CreatedByUserID)
	}
}

// TestSharedIdentityTodoMentionsNoLeak proves workbench mentions never surface
// tasks from inaccessible projects, even when the user was @-mentioned there.
func TestSharedIdentityTodoMentionsNoLeak(t *testing.T) {
	fx := newSharedIdentityApp(t)

	// A comment in the hidden project's task mentions the participant.
	// Mentions are structured editor JSON (type=mention with an id), not raw
	// @-text, so the body must carry the fact user ID in that shape.
	now := time.Now().UTC()
	task := seedSharedTask(t, fx.db, projHidden, unitHidden, reqHidden, "hidden task title")
	mentionBody := fmt.Sprintf(
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"mention","attrs":{"id":%d}}]}]}`,
		participantUserID,
	)
	if err := fx.db.Exec(
		"INSERT INTO work_task_comments (workspace_id, work_task_id, user_id, body, created_at) VALUES (?, ?, ?, ?, ?)",
		sharedWSID, task, ownerUserID, mentionBody, now,
	).Error; err != nil {
		t.Fatalf("seed comment: %v", err)
	}

	rec := fx.request(t, http.MethodGet,
		"/api/v1/platform/workbench/todos/feed?type=mention", fx.partToken, sharedWSID, nil)
	requireStatus(t, rec, http.StatusOK)
	rows := decodeData(t, rec).([]any)
	for _, raw := range rows {
		row := raw.(map[string]any)
		if title, _ := row["title"].(string); title == "hidden task title" {
			t.Fatal("workbench mention feed leaked an inaccessible project's task")
		}
	}

	rec = fx.request(t, http.MethodGet,
		"/api/v1/platform/workbench/todos/summary?include_mentions=true", fx.partToken, sharedWSID, nil)
	requireStatus(t, rec, http.StatusOK)
	summary := decodeData(t, rec).(map[string]any)["summary"].(map[string]any)
	if mentions, _ := summary["mentions"].(float64); mentions != 0 {
		t.Fatalf("mention summary counted %v inaccessible mention(s)", mentions)
	}
}

// TestSharedIdentityWorkspaceIsolation proves membership in one workspace
// never leaks another workspace's project through the same fact ID.
func TestSharedIdentityWorkspaceIsolation(t *testing.T) {
	fx := newSharedIdentityApp(t)

	// A second workspace with the same creator fact on a project there.
	otherWS := uint(2600)
	for _, row := range []any{
		&identitygorm.Workspace{ID: otherWS, Name: "OtherWS", Slug: "otherws", Enabled: true},
		&identitygorm.Team{ID: 2610, WorkspaceID: otherWS, Name: "OtherTeam"},
		&identitygorm.WorkspaceMember{WorkspaceID: otherWS, UserID: 102, Role: "member", Enabled: true},
	} {
		if err := fx.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	otherCreator := strangerUserID
	if err := fx.db.Create(&projectgorm.ProjectSet{
		ID: 2611, WorkspaceID: otherWS, TeamID: 2610, Name: "OtherWS Project", CreatedByUserID: &otherCreator, Status: "active",
	}).Error; err != nil {
		t.Fatal(err)
	}

	// The participant asks with the OTHER workspace header: they are not a
	// member of that workspace's project, so it must not be enumerable.
	rec := fx.request(t, http.MethodGet, "/api/v1/platform/project-sets", fx.partToken, otherWS, nil)
	requireStatus(t, rec, http.StatusOK)
	rows := decodeData(t, rec).([]any)
	for _, raw := range rows {
		if uint(raw.(map[string]any)["id"].(float64)) == 2611 {
			t.Fatal("project from another workspace leaked via workspace header switch")
		}
	}
}

// TestSharedIdentityOrgProjectsFiltered proves the core /projects listing
// follows the same project data access as /project-sets.
func TestSharedIdentityOrgProjectsFiltered(t *testing.T) {
	fx := newSharedIdentityApp(t)

	rec := fx.request(t, http.MethodGet, "/api/v1/core/projects", fx.partToken, sharedWSID, nil)
	requireStatus(t, rec, http.StatusOK)
	rows := decodeData(t, rec).([]any)
	if len(rows) != 1 || uint(rows[0].(map[string]any)["id"].(float64)) != projShared {
		t.Fatalf("core /projects returned %v, want only project %d", rows, projShared)
	}
}

func seedSharedTask(t *testing.T, db *gorm.DB, projectID, unitID, requirementID uint, title string) uint {
	t.Helper()
	task := map[string]any{
		"workspace_id": sharedWSID, "team_id": sharedTeamID, "project_set_id": projectID,
		"project_id": unitID, "requirement_id": requirementID, "kind": "task",
		"task_key": "SHARED-" + fmt.Sprint(projectID), "title": title, "status": "todo",
		"priority": "medium",
	}
	columns := make([]string, 0, len(task))
	placeholders := make([]string, 0, len(task))
	args := make([]any, 0, len(task))
	for column, value := range task {
		columns = append(columns, column)
		placeholders = append(placeholders, "?")
		args = append(args, value)
	}
	if err := db.Exec(
		"INSERT INTO work_tasks ("+joinStrings(columns, ", ")+") VALUES ("+joinStrings(placeholders, ", ")+")",
		args...,
	).Error; err != nil {
		t.Fatalf("seed work task: %v", err)
	}
	var id uint
	db.Raw("SELECT id FROM work_tasks WHERE workspace_id = ? AND title = ?", sharedWSID, title).Scan(&id)
	return id
}

func joinStrings(values []string, sep string) string {
	result := ""
	for i, value := range values {
		if i > 0 {
			result += sep
		}
		result += value
	}
	return result
}

func equalUintSlice(got, want []uint) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestInternalIDMemberRowsResolveNames proves member profiles use canonical IDs.
func TestInternalIDMemberRowsResolveNames(t *testing.T) {
	fx := newSharedIdentityApp(t)
	rec := fx.request(t, http.MethodGet,
		"/api/v1/platform/project-new/"+fmt.Sprint(unitShared)+"/members",
		fx.adminToken, sharedWSID, nil)
	requireStatus(t, rec, http.StatusOK)
	items := decodeData(t, rec).(map[string]any)["items"].([]any)

	found := false
	for _, raw := range items {
		row := raw.(map[string]any)
		// The stored internal ID must surface the real display name.
		if row["display_name"] == "Ext Participant" {
			found = true
		}
		if name, _ := row["display_name"].(string); name == "" && row["username"] == "" {
			t.Fatalf("member row has no resolvable name: %v", row)
		}
	}
	if !found {
		t.Fatalf("legacy internal-ID participant row did not resolve to its user: %v", items)
	}
}

var _ = gorm.ErrRecordNotFound
