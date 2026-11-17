package httpapi

import (
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"net/http"
	"testing"
)

func TestQualityBugRequirementMoveUpdatesHierarchyForWorkspaceMember(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	targetProject := projectgorm.ProjectSet{
		ID: 31, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
		Name: "Target project", Code: "TARGET",
	}
	targetPlan := projectgorm.PlanUnit{
		ID: 32, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
		ProjectSetID: targetProject.ID, Title: "Target plan",
	}
	targetRequirement := requirementgorm.Requirement{
		ID: 33, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
		ProjectSetID: targetProject.ID, PlanUnitID: targetPlan.ID, Title: "Target requirement",
	}
	for _, row := range []any{&targetProject, &targetPlan, &targetRequirement} {
		if err := fix.app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	bug := fix.createBug(t, 600, "triage", false)
	moved := fix.request(t, "owner", http.MethodPatch, bug.ID, map[string]any{
		"requirement_id": targetRequirement.ID,
	})
	requireStatus(t, moved, http.StatusOK)
	data := decodeData(t, moved).(map[string]any)
	if data["team_id"] != float64(fix.project.TeamID) ||
		data["project_set_id"] != float64(targetProject.ID) ||
		data["project_id"] != float64(targetPlan.ID) ||
		data["requirement_id"] != float64(targetRequirement.ID) {
		t.Fatalf("moved hierarchy response=%v", data)
	}
	var stored qualitygorm.QualityBug
	if err := fix.app.db.First(&stored, bug.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ProjectSetID != targetProject.ID || stored.ProjectID != targetPlan.ID ||
		stored.RequirementID != targetRequirement.ID || stored.TeamID != fix.project.TeamID {
		t.Fatalf("stored hierarchy=%+v", stored)
	}
}

func TestQualityBugReporterMayMoveWithinAuthorizedProjectButNotAcrossWorkspace(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	sameProjectRequirement := requirementgorm.Requirement{
		ID: 14, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
		ProjectSetID: fix.project.ID, PlanUnitID: fix.plan.ID, Title: "Other requirement",
	}
	if err := fix.app.db.Create(&sameProjectRequirement).Error; err != nil {
		t.Fatal(err)
	}
	reporterBug := fix.createBug(t, 601, "triage", false)
	moved := fix.request(t, "reporter", http.MethodPatch, reporterBug.ID, map[string]any{
		"requirement_id": sameProjectRequirement.ID,
	})
	requireStatus(t, moved, http.StatusOK)
	if got := decodeData(t, moved).(map[string]any)["requirement_id"]; got != float64(14) {
		t.Fatalf("requirement_id=%v body=%s", got, moved.Body.String())
	}

	foreignRequirement := seedForeignQualityUploadScope(t, fix)
	foreignBug := fix.createBug(t, 602, "triage", false)
	foreign := fix.request(t, "manager", http.MethodPatch, foreignBug.ID, map[string]any{
		"requirement_id": foreignRequirement.ID,
	})
	requireStatus(t, foreign, http.StatusNotFound)
	var unchanged qualitygorm.QualityBug
	if err := fix.app.db.First(&unchanged, foreignBug.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.RequirementID != fix.requirement.ID || unchanged.ProjectSetID != fix.project.ID {
		t.Fatalf("cross-workspace move changed bug=%+v", unchanged)
	}
}
