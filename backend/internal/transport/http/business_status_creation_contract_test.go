package httpapi

import (
	"encoding/json"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"net/http"
	"testing"
)

func TestCreatedBusinessSubjectSnapshotsUseWorkflowSynchronizedStatus(t *testing.T) {
	app, token, workspace, project, existingUnit := newAIWorkflowFixture(t)
	planTemplateID := createPublishedSubjectStatusWorkflow(
		t, app, token, workspace.ID, "plan_unit", "acceptance",
	)
	requirementTemplateID := createPublishedSubjectStatusWorkflow(
		t, app, token, workspace.ID, "requirement", "grooming",
	)

	t.Run("plan unit", func(t *testing.T) {
		response := app.request(t, http.MethodPost, "/api/v1/platform/project-new",
			token, workspace.ID, map[string]any{
				"project_new_id": project.ID, "title": "工作流状态版本", "type": "version",
				"workflow_template_id": planTemplateID,
			})
		requireStatus(t, response, http.StatusCreated)
		data := decodeData(t, response).(map[string]any)
		unitID := uint(data["id"].(float64))
		if data["status"] != "acceptance" {
			t.Fatalf("creation response status=%v, want acceptance", data["status"])
		}
		var stored projectgorm.PlanUnit
		if err := app.db.First(&stored, unitID).Error; err != nil || stored.Status != data["status"] {
			t.Fatalf("response diverged from stored plan status: response=%v stored=%q err=%v", data["status"], stored.Status, err)
		}
		requireCreatedOutboxSnapshot(t, app, workspace.ID, "plan_unit", unitID, "acceptance", nil)
	})

	tests := []struct {
		name string
		path string
		body map[string]any
	}{
		{
			name: "legacy requirement",
			path: "/api/v1/platform/product-requirements-new",
			body: map[string]any{
				"version_task_new_id": existingUnit.ID, "title": "工作流状态需求",
				"workflow_template_id": requirementTemplateID, "priority": "p1",
			},
		},
		{
			name: "requirement pool",
			path: "/api/v1/platform/requirements",
			body: map[string]any{
				"project_new_id": project.ID, "version_task_new_id": existingUnit.ID, "title": "工作流状态需求池",
				"workflow_template_id": requirementTemplateID, "priority": "p1",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := app.request(t, http.MethodPost, test.path, token, workspace.ID, test.body)
			requireStatus(t, response, http.StatusCreated)
			data := decodeData(t, response).(map[string]any)
			requirementID := uint(data["id"].(float64))
			if data["status"] != "grooming" || data["version"] != float64(2) {
				t.Fatalf("creation response omitted workflow snapshot: %v", data)
			}
			var stored requirementgorm.Requirement
			if err := app.db.First(&stored, requirementID).Error; err != nil ||
				stored.Status != data["status"] || float64(stored.Version) != data["version"] {
				t.Fatalf("response diverged from stored requirement: response=%v stored=%+v err=%v", data, stored, err)
			}
			version := uint(2)
			requireCreatedOutboxSnapshot(t, app, workspace.ID, "requirement", requirementID, "grooming", &version)
		})
	}
}

func createPublishedSubjectStatusWorkflow(
	t *testing.T,
	app *testApp,
	token string,
	workspaceID uint,
	subjectType string,
	status string,
) uint {
	t.Helper()
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{
				"id": "waiting", "type": "human_task", "name": "等待处理",
				"config": map[string]any{"subject_status": status},
			},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "waiting"},
			{"id": "e2", "source": "waiting", "target": "end"},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates",
		token, workspaceID, map[string]any{
			"name": "snapshot-" + subjectType, "subject_type": subjectType, "definition": definition,
		})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost,
		"/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish",
		token, workspaceID, nil), http.StatusOK)
	return templateID
}

func requireCreatedOutboxSnapshot(
	t *testing.T,
	app *testApp,
	workspaceID uint,
	aggregateType string,
	aggregateID uint,
	wantStatus string,
	wantVersion *uint,
) {
	t.Helper()
	var event auditgorm.OutboxEvent
	if err := app.db.Where(
		"workspace_id = ? AND aggregate_type = ? AND aggregate_id = ? AND event_type = ?",
		workspaceID, aggregateType, aggregateID, "pm."+aggregateType+".created",
	).First(&event).Error; err != nil {
		t.Fatalf("created outbox event missing: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
		t.Fatalf("decode created outbox payload: %v", err)
	}
	if payload["status"] != wantStatus {
		t.Fatalf("created outbox status=%v, want %q payload=%s", payload["status"], wantStatus, event.Payload)
	}
	if wantVersion != nil && payload["version"] != float64(*wantVersion) {
		t.Fatalf("created outbox version=%v, want %d payload=%s", payload["version"], *wantVersion, event.Payload)
	}
}
