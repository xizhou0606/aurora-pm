package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

func TestWorkflowPreReviewOpinionsContract(t *testing.T) {
	app, ownerToken, workspace, _, unit := newAIWorkflowFixture(t)
	owner := identitygorm.User{}
	if err := app.db.Where("username = ?", "owner").First(&owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&owner).Updates(map[string]any{
		"display_name": "产品负责人", "avatar_url": "https://cdn.example.test/owner.png",
	}).Error; err != nil {
		t.Fatal(err)
	}

	reviewer := app.createUser(t, 2, "reviewer", "user", "")
	reviewer.DisplayName = "测试同学"
	reviewer.AvatarURL = "https://cdn.example.test/reviewer.png"
	if err := app.db.Model(&reviewer).Updates(map[string]any{
		"display_name": reviewer.DisplayName, "avatar_url": reviewer.AvatarURL,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&identitygorm.WorkspaceMember{
		WorkspaceID: workspace.ID, UserID: reviewer.ID, Role: "member", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	reviewerToken := app.token(t, reviewer)

	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "pre-review", "type": "approval", "name": "需求预审", "config": map[string]any{"critical_gate": true}},
			{"id": "review", "type": "approval", "name": "产品评审"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "pre-review"},
			{"id": "e2", "source": "pre-review", "target": "review", "kind": "approve"},
			{"id": "e3", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", ownerToken, workspace.ID, map[string]any{
		"name": "预审意见流", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", ownerToken, workspace.ID, nil), http.StatusOK)

	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "预审意见需求"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))

	foreign := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/nodes/review/opinions", ownerToken, workspace.ID, map[string]any{
		"body": "非预审节点不应接收意见",
	})
	requireStatus(t, foreign, http.StatusNotFound)

	first := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/nodes/pre-review/opinions", ownerToken, workspace.ID, map[string]any{
		"body": "边界条件需要补充",
	})
	requireStatus(t, first, http.StatusCreated)
	firstData := decodeData(t, first).(map[string]any)
	if firstData["body"] != "边界条件需要补充" || firstData["author_display_name"] != "产品负责人" || firstData["author_avatar_url"] != "https://cdn.example.test/owner.png" {
		t.Fatalf("created opinion contract=%v", firstData)
	}
	opinionID := uint(firstData["id"].(float64))

	second := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/nodes/pre-review/opinions", reviewerToken, workspace.ID, map[string]any{
		"body": "建议补齐验收标准",
	})
	requireStatus(t, second, http.StatusCreated)

	listed := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/nodes/pre-review/opinions", reviewerToken, workspace.ID, nil)
	requireStatus(t, listed, http.StatusOK)
	rows := decodeData(t, listed).([]any)
	if len(rows) != 2 {
		t.Fatalf("opinion list length=%d want 2", len(rows))
	}
	newest := rows[0].(map[string]any)
	if newest["body"] != "建议补齐验收标准" || newest["author_display_name"] != "测试同学" {
		t.Fatalf("newest opinion contract=%v", newest)
	}

	updatedByOther := app.request(t, http.MethodPatch, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/nodes/pre-review/opinions/"+uintString(opinionID), reviewerToken, workspace.ID, map[string]any{
		"body": "尝试改别人的意见",
	})
	requireStatus(t, updatedByOther, http.StatusOK)

	past := time.Now().UTC().Add(-2 * time.Second)
	if err := app.db.Model(&workflowgorm.WorkflowNodeOpinion{}).Where("id = ?", opinionID).Update("created_at", past).Error; err != nil {
		t.Fatal(err)
	}
	updated := app.request(t, http.MethodPatch, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/nodes/pre-review/opinions/"+uintString(opinionID), ownerToken, workspace.ID, map[string]any{
		"body": "边界条件与异常路径都要补充",
	})
	requireStatus(t, updated, http.StatusOK)
	updatedData := decodeData(t, updated).(map[string]any)
	if updatedData["body"] != "边界条件与异常路径都要补充" || updatedData["edited"] != true {
		t.Fatalf("updated opinion contract=%v", updatedData)
	}
}

func TestWorkflowPreReviewInviteEnqueuesFeishuNotification(t *testing.T) {
	app, ownerToken, workspace, _, unit := newNotifyingAIWorkflowFixture(t)
	reviewer := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "reviewer")
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "pre-review", "type": "approval", "name": "需求预审", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "pre-review"},
			{"id": "e2", "source": "pre-review", "target": "end", "kind": "approve"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "预审邀请流程", definition)
	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "邀请预审需求"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))

	invited := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/nodes/pre-review/opinions", ownerToken, workspace.ID, map[string]any{
		"invite_user_ids": []uint{reviewer.ID},
	})
	requireStatus(t, invited, http.StatusOK)
	payload := decodeData(t, invited).(map[string]any)
	ids, _ := payload["invited_user_ids"].([]any)
	if len(ids) != 1 || uint(ids[0].(float64)) != reviewer.ID {
		t.Fatalf("invited payload=%v", payload)
	}

	var events []auditgorm.OutboxEvent
	if err := app.db.Where(
		"aggregate_type = ? AND aggregate_id = ? AND event_type = ?",
		"workflow_node_notification", instanceID, notification.EventType(notification.SceneWorkflowPreReviewInvited),
	).Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("pre-review invite notifications=%d want 1", len(events))
	}
	if !strings.Contains(events[0].Payload, `"scene":"workflow.pre_review_invited"`) {
		t.Fatalf("invite payload=%s", events[0].Payload)
	}
}
