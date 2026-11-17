package api

import (
	"fmt"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	notificationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"gorm.io/gorm"
)

func membershipRequestNotifier(cfg config.Notification) func(*gorm.DB, project.JoinScope, project.JoinMutation, project.JoinRequest, uint) error {
	return func(tx *gorm.DB, s project.JoinScope, m project.JoinMutation, r project.JoinRequest, recipient uint) error {
		scene, text := notification.SceneProjectAccessRequested, "收到项目/版本加入申请"
		if m.Decision != "" {
			if err := notificationgorm.NewMentionRepository(tx).MarkRequestReviewed(tx.Statement.Context, s.WorkspaceID, m.Actor.FactID, r.ID, "membership_access_request"); err != nil {
				return err
			}
			scene, text = notification.SceneProjectAccessReviewed, "项目/版本加入申请已有审批结果"
		}
		resourceType, resourceID := "plan_unit", fmt.Sprintf("%d/%d", s.ProjectSetID, s.PlanUnitID)
		url := notification.PublicURL(cfg.PublicBaseURL, "workspace", "inbox") + "#project-requests-" + fmt.Sprint(s.PlanUnitID)
		name := s.ProjectName

		if s.TargetKind == "workspace" {
			scene, text = notification.SceneWorkspaceAccessRequested, "收到工作区加入申请"
			if m.Decision != "" {
				scene, text = notification.SceneWorkspaceAccessReviewed, "工作区加入申请已有审批结果"
			}
			resourceType, resourceID, name = "workspace", fmt.Sprint(s.WorkspaceID), s.WorkspaceName
			url = notification.PublicURL(cfg.PublicBaseURL, "workspace", "inbox") + "#workspace-requests-" + resourceID
		}
		recipients := []uint{recipient}
		if m.Decision == "" {
			recipients = s.ReviewerIDs
		}
		return notification.Enqueue(tx, "membership_access_request", r.ID, notification.Intent{WorkspaceID: s.WorkspaceID, TeamID: s.TeamID, ProjectID: s.PlanUnitID, Scene: scene, ActorUserID: m.Actor.FactID, RecipientUserIDs: recipients, ResourceType: resourceType, ResourceID: resourceID, RequestID: r.ID, SkipExternal: !cfg.Enabled, Vars: map[string]string{"url": url, "notification_event_key": fmt.Sprintf("request:%d:%s", r.ID, m.Decision)}, FallbackPlainText: text + "\n" + name + "\n" + url})
	}
}
