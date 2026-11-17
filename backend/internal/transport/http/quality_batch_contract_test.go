package httpapi

import (
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm/records"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	"net/http"
	"testing"
)

func TestQualityBugBatchCreateIsAtomicAndPreservesClientMapping(t *testing.T) {
	fix := setupTeamUserAssignmentFixture(t)
	response := fix.app.request(t, http.MethodPost,
		"/api/v1/platform/quality/bugs/batch-create", fix.token, fix.workspaceID,
		map[string]any{"bugs": []map[string]any{
			{"client_id": "bug-a", "requirement_id": fix.requirement, "title": "Bug A"},
			{"client_id": "bug-b", "requirement_id": fix.requirement, "title": "Bug B", "priority": "high"},
		}},
	)
	requireStatus(t, response, http.StatusCreated)
	data := decodeData(t, response).(map[string]any)
	if data["created_count"] != float64(2) || data["atomic"] != true {
		t.Fatalf("batch create=%s", response.Body.String())
	}
	created := data["created"].([]any)
	if created[0].(map[string]any)["client_id"] != "bug-a" ||
		created[1].(map[string]any)["client_id"] != "bug-b" ||
		created[0].(map[string]any)["bug_key"] == "" {
		t.Fatalf("client mapping=%s", response.Body.String())
	}
	var bugs, activities, audits, outbox int64
	fix.app.db.Model(&qualitygorm.QualityBug{}).Count(&bugs)
	fix.app.db.Model(&qualitygorm.QualityBugActivity{}).Count(&activities)
	fix.app.db.Model(&auditgorm.AuditLog{}).Count(&audits)
	fix.app.db.Model(&auditgorm.OutboxEvent{}).Count(&outbox)
	if bugs != 2 || activities != 2 || audits != 2 || outbox != 4 {
		t.Fatalf("bugs=%d activities=%d audits=%d outbox=%d", bugs, activities, audits, outbox)
	}
	var notificationCount, receiptCount int64
	fix.app.db.Model(&auditgorm.OutboxEvent{}).Where("event_type = ?", notification.EventType(notification.SceneQualityBugCreated)).Count(&notificationCount)
	fix.app.db.Model(&records.InboxReceipt{}).Where("category = ?", "message").Count(&receiptCount)
	// Both bugs are reported by the current actor; retain external events but
	// do not put the actor's own creations in their inbox.
	if notificationCount != 2 || receiptCount != 0 {
		t.Fatalf("notifications=%d receipts=%d", notificationCount, receiptCount)
	}
}

func TestQualityBugBatchCreateRejectsInvalidItemWithoutWrites(t *testing.T) {
	fix := setupTeamUserAssignmentFixture(t)
	response := fix.app.request(t, http.MethodPost,
		"/api/v1/platform/quality/bugs/batch-create", fix.token, fix.workspaceID,
		map[string]any{"bugs": []map[string]any{
			{"client_id": "valid", "requirement_id": fix.requirement, "title": "Valid"},
			{"client_id": "invalid", "requirement_id": fix.requirement, "title": ""},
		}},
	)
	requireStatus(t, response, http.StatusUnprocessableEntity)
	var bugs, activities, audits, outbox int64
	fix.app.db.Model(&qualitygorm.QualityBug{}).Count(&bugs)
	fix.app.db.Model(&qualitygorm.QualityBugActivity{}).Count(&activities)
	fix.app.db.Model(&auditgorm.AuditLog{}).Count(&audits)
	fix.app.db.Model(&auditgorm.OutboxEvent{}).Count(&outbox)
	if bugs != 0 || activities != 0 || audits != 0 || outbox != 0 {
		t.Fatalf("invalid batch wrote bugs=%d activities=%d audits=%d outbox=%d", bugs, activities, audits, outbox)
	}
}

func TestQualityBugBatchDeleteRequiresCompleteScopeAndDeletesAtomically(t *testing.T) {
	fix := setupTeamUserAssignmentFixture(t)
	bugs := []qualitygorm.QualityBug{
		{ID: 801, WorkspaceID: fix.workspaceID, TeamID: fix.teamID, ProjectSetID: fix.projectID,
			ProjectID: fix.planID, RequirementID: fix.requirement, BugKey: "BUG-801",
			Title: "Bug 801", Status: "triage", Priority: "medium", SeverityKey: "p2"},
		{ID: 802, WorkspaceID: fix.workspaceID, TeamID: fix.teamID, ProjectSetID: fix.projectID,
			ProjectID: fix.planID, RequirementID: fix.requirement, BugKey: "BUG-802",
			Title: "Bug 802", Status: "triage", Priority: "medium", SeverityKey: "p2"},
	}
	for index := range bugs {
		if err := fix.app.db.Create(&bugs[index]).Error; err != nil {
			t.Fatal(err)
		}
		if err := fix.app.db.Create(&qualitygorm.QualityBugActivity{
			WorkspaceID: fix.workspaceID, QualityBugID: bugs[index].ID,
			UserID: fix.users["actor"], Action: "created",
		}).Error; err != nil {
			t.Fatal(err)
		}
		if err := fix.app.db.Create(&qualitygorm.QualityBugComment{
			WorkspaceID: fix.workspaceID, QualityBugID: bugs[index].ID,
			UserID: fix.users["actor"], Body: "batch cleanup",
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	blocked := fix.app.request(t, http.MethodPost,
		"/api/v1/platform/quality/bugs/batch-delete", fix.token, fix.workspaceID,
		map[string]any{"ids": []uint{801, 999}},
	)
	requireStatus(t, blocked, http.StatusNotFound)
	var retained int64
	fix.app.db.Model(&qualitygorm.QualityBug{}).Where("id IN ?", []uint{801, 802}).Count(&retained)
	if retained != 2 {
		t.Fatalf("failed atomic delete retained=%d", retained)
	}
	deleted := fix.app.request(t, http.MethodPost,
		"/api/v1/platform/quality/bugs/batch-delete", fix.token, fix.workspaceID,
		map[string]any{"ids": []uint{802, 801, 801}},
	)
	requireStatus(t, deleted, http.StatusOK)
	data := decodeData(t, deleted).(map[string]any)
	if data["deleted_count"] != float64(2) || data["affected_count"] != float64(2) ||
		data["atomic"] != true || len(data["deleted_ids"].([]any)) != 2 {
		t.Fatalf("batch delete=%s", deleted.Body.String())
	}
	var remaining, activities, comments, audits, outbox int64
	fix.app.db.Model(&qualitygorm.QualityBug{}).Where("id IN ?", []uint{801, 802}).Count(&remaining)
	fix.app.db.Model(&qualitygorm.QualityBugActivity{}).Where("quality_bug_id IN ?", []uint{801, 802}).Count(&activities)
	fix.app.db.Model(&qualitygorm.QualityBugComment{}).Where("quality_bug_id IN ?", []uint{801, 802}).Count(&comments)
	fix.app.db.Model(&auditgorm.AuditLog{}).Where("resource_id IN ?", []uint{801, 802}).Count(&audits)
	fix.app.db.Model(&auditgorm.OutboxEvent{}).Where("aggregate_id IN ?", []uint{801, 802}).Count(&outbox)
	if remaining != 0 || activities != 0 || comments != 0 || audits != 2 || outbox != 2 {
		t.Fatalf("remaining=%d activities=%d comments=%d audits=%d outbox=%d", remaining, activities, comments, audits, outbox)
	}
}

func TestQualityBugBatchAssignmentRequiresCompleteScopeAndUpdatesAtomically(t *testing.T) {
	fix := setupTeamUserAssignmentFixture(t)
	assigneeID := fix.users["member"]
	bugs := []qualitygorm.QualityBug{
		{ID: 811, WorkspaceID: fix.workspaceID, TeamID: fix.teamID, ProjectSetID: fix.projectID,
			ProjectID: fix.planID, RequirementID: fix.requirement, BugKey: "BUG-811",
			Title: "Bug 811", Status: "triage", Priority: "medium", SeverityKey: "p2"},
		{ID: 812, WorkspaceID: fix.workspaceID, TeamID: fix.teamID, ProjectSetID: fix.projectID,
			ProjectID: fix.planID, RequirementID: fix.requirement, BugKey: "BUG-812",
			Title: "Bug 812", Status: "in_progress", Priority: "medium", SeverityKey: "p2"},
	}
	if err := fix.app.db.Create(&bugs).Error; err != nil {
		t.Fatal(err)
	}

	blocked := fix.app.request(t, http.MethodPost,
		"/api/v1/platform/quality/bugs/batch-assignment", fix.token, fix.workspaceID,
		map[string]any{"ids": []uint{811, 999}, "assignee_user_id": assigneeID},
	)
	requireStatus(t, blocked, http.StatusNotFound)
	var assignedBefore int64
	fix.app.db.Model(&qualitygorm.QualityBug{}).
		Where("id IN ? AND assignee_user_id IS NOT NULL", []uint{811, 812}).Count(&assignedBefore)
	if assignedBefore != 0 {
		t.Fatalf("failed atomic assignment updated %d bugs", assignedBefore)
	}

	assigned := fix.app.request(t, http.MethodPost,
		"/api/v1/platform/quality/bugs/batch-assignment", fix.token, fix.workspaceID,
		map[string]any{
			"ids": []uint{812, 811, 811}, "assignee_user_id": assigneeID,
			"assignment_note": "统一指派",
		},
	)
	requireStatus(t, assigned, http.StatusOK)
	data := decodeData(t, assigned).(map[string]any)
	if data["affected_count"] != float64(2) || data["atomic"] != true ||
		len(data["affected_ids"].([]any)) != 2 {
		t.Fatalf("batch assignment=%s", assigned.Body.String())
	}
	var assignedCount, activityCount int64
	fix.app.db.Model(&qualitygorm.QualityBug{}).
		Where("id IN ? AND assignee_user_id = ?", []uint{811, 812}, assigneeID).Count(&assignedCount)
	fix.app.db.Model(&qualitygorm.QualityBugActivity{}).
		Where("quality_bug_id IN ? AND action = ?", []uint{811, 812}, "assigned").Count(&activityCount)
	if assignedCount != 2 || activityCount != 2 {
		t.Fatalf("assigned=%d activities=%d", assignedCount, activityCount)
	}
}
