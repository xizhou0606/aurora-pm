package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	notificationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func TestWorkflowAndMemberMessagesSurviveExternalDisabled(t *testing.T) {
	app := newTestApp(t, nil)
	owner := app.createUser(t, fixtureOwner, "message-owner", "super_admin", "")
	recipient := app.createUser(t, fixtureUser2, "message-recipient", "user", "")
	workspace, proj := seedPlanUnitWorkspace(t, app, owner.ID)
	var unit projectgorm.PlanUnit
	if err := app.db.First(&unit, fixtureUnit).Error; err != nil {
		t.Fatal(err)
	}
	server := &Server{db: app.db, cfg: app.cfg}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	instance := workflowgorm.WorkflowInstance{ID: 91, WorkspaceID: workspace.ID, SubjectType: "plan_unit", SubjectID: unit.ID}
	err := app.db.Transaction(func(tx *gorm.DB) error {
		if err := server.recordWorkflowNodeAssignedNotification(tx, c, instance, "development", recipient.ID); err != nil {
			return err
		}
		if err := server.recordWorkflowPreReviewInvitedNotification(tx, c, instance, workflowNode{ID: "review", Name: "产品预审"}, []uint{recipient.ID}); err != nil {
			return err
		}
		return server.recordPlanUnitMemberNotifications(tx, proj, unit, project.MemberState{}, project.MemberState{OwnerUserID: &recipient.ID}, owner.ID)
	})
	if err != nil {
		t.Fatal(err)
	}
	var events []auditgorm.OutboxEvent
	if err := app.db.Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		var intent notification.Intent
		if json.Unmarshal([]byte(event.Payload), &intent) != nil || !intent.SkipExternal {
			t.Fatal("external channel must remain disabled")
		}
	}
	page, err := notificationgorm.NewMentionRepository(app.db).ListInbox(context.Background(), workspace.ID, recipient.ID, notification.InboxFilter{Category: "message"})
	if err != nil || len(page.Items) != 3 {
		t.Fatalf("page=%+v error=%v", page, err)
	}
	for _, item := range page.Items {
		if item.ResourceType != "plan_unit" || item.ResourceID != fmt.Sprintf("%d/%d", proj.ID, unit.ID) || item.ResourceTitle != unit.Title {
			t.Fatalf("route/title=%+v", item)
		}
	}
}
