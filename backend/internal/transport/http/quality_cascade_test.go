package httpapi

import (
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	"net/http"
	"testing"
)

func TestDeleteRequirementCascadesQualityBugsAndActivities(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	bug := fix.createBug(t, 800, "triage", false)
	createQualityActivity(t, fix, bug.ID)
	response := fix.app.request(
		t, http.MethodDelete,
		"/api/v1/platform/product-requirements-new/"+uintString(fix.requirement.ID),
		fix.app.token(t, fix.users["owner"]), fix.workspaceID, nil,
	)
	requireStatus(t, response, http.StatusOK)
	assertQualityRowsRemoved(t, fix, bug.ID)
}

func TestDeleteProjectSetCascadesQualityBugsAndActivities(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	bug := fix.createBug(t, 801, "triage", false)
	createQualityActivity(t, fix, bug.ID)
	response := fix.app.request(
		t, http.MethodDelete,
		"/api/v1/platform/project-sets/"+uintString(fix.project.ID),
		fix.app.token(t, fix.users["owner"]), fix.workspaceID, nil,
	)
	requireStatus(t, response, http.StatusOK)
	assertQualityRowsRemoved(t, fix, bug.ID)
}

func createQualityActivity(
	t *testing.T,
	fix qualityPermissionFixture,
	bugID uint,
) {
	t.Helper()
	if err := fix.app.db.Create(&qualitygorm.QualityBugActivity{
		WorkspaceID: fix.workspaceID, QualityBugID: bugID,
		UserID: fix.users["reporter"].ID, Action: "created", Summary: "提交缺陷",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := fix.app.db.Create(&qualitygorm.QualityBugComment{
		WorkspaceID: fix.workspaceID, QualityBugID: bugID,
		UserID: fix.users["reporter"].ID, Body: "cascade cleanup",
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func assertQualityRowsRemoved(
	t *testing.T,
	fix qualityPermissionFixture,
	bugID uint,
) {
	t.Helper()
	var bugs, activities, comments int64
	if err := fix.app.db.Unscoped().Model(&qualitygorm.QualityBug{}).
		Where("id = ?", bugID).Count(&bugs).Error; err != nil {
		t.Fatal(err)
	}
	if err := fix.app.db.Model(&qualitygorm.QualityBugActivity{}).
		Where("quality_bug_id = ?", bugID).Count(&activities).Error; err != nil {
		t.Fatal(err)
	}
	if err := fix.app.db.Model(&qualitygorm.QualityBugComment{}).
		Where("quality_bug_id = ?", bugID).Count(&comments).Error; err != nil {
		t.Fatal(err)
	}
	if bugs != 0 || activities != 0 || comments != 0 {
		t.Fatalf("quality cascade left bugs=%d activities=%d comments=%d", bugs, activities, comments)
	}
}
