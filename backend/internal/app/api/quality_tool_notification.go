package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"gorm.io/gorm"
)

type qualityToolMutationEffects struct {
	tx     *gorm.DB
	config config.Notification
}

func (writer qualityToolMutationEffects) RecordBugMutation(
	ctx context.Context,
	effect quality.BugMutationEffect,
) error {
	if writer.tx == nil || effect.WorkspaceID == 0 || effect.PrincipalID == 0 ||
		effect.After.ID == 0 || effect.After.WorkspaceID != effect.WorkspaceID ||
		effect.After.TeamID == 0 || effect.After.PlanUnitID == 0 {
		return quality.ErrBugMutationConflict
	}

	scene, recipients, includeActor := qualityToolNotificationRoute(effect)
	if scene == "" {
		return nil
	}
	recipients = toolNotificationRecipients(effect.PrincipalID, includeActor, recipients)
	if len(recipients) == 0 {
		return nil
	}
	users, err := toolNotificationUsers(ctx, writer.tx, effect.WorkspaceID, effect.PrincipalID)
	if err != nil {
		return err
	}
	bug := effect.After
	assigneeID, reviewerID := uint(0), uint(0)
	if bug.AssigneeUserID != nil {
		assigneeID = *bug.AssigneeUserID
	}
	if bug.ReviewerUserID != nil {
		reviewerID = *bug.ReviewerUserID
	}
	vars := map[string]string{
		"operator":         toolNotificationUserLabel(users, effect.PrincipalID),
		"title":            strings.TrimSpace(bug.Title),
		"bugKey":           strings.TrimSpace(bug.BugKey),
		"severity":         quality.SeverityLabel(bug.SeverityKey),
		"status":           quality.StatusLabel(bug.Status),
		"statusKey":        bug.Status,
		"resolutionReason": quality.ResolutionReasonLabel(bug.ResolutionReason),
		"assignee":         toolNotificationUserLabel(users, assigneeID),
		"assigneeUserId":   fmt.Sprint(assigneeID),
		"reviewer":         toolNotificationUserLabel(users, reviewerID),
		"ccUsers":          "-",
		"note":             "-",
		"url":              notification.QualityBugURL(writer.config.PublicBaseURL, bug.ID),
	}
	if vars["title"] == "" {
		vars["title"] = "未命名缺陷"
	}
	if vars["bugKey"] == "" {
		vars["bugKey"] = fmt.Sprintf("BUG-%d", bug.ID)
	}
	for _, key := range []string{"severity", "status", "resolutionReason", "assignee", "reviewer"} {
		if strings.TrimSpace(vars[key]) == "" {
			vars[key] = "-"
		}
	}
	if effect.Before != nil {
		vars["fromStatusKey"] = effect.Before.Status
		vars["toStatusKey"] = bug.Status
	}
	return notification.Enqueue(writer.tx.WithContext(ctx), "quality_bug_notification", bug.ID, notification.Intent{
		SkipExternal: !writer.config.Enabled, ResourceType: "quality_bug", ResourceID: fmt.Sprint(bug.ID),
		WorkspaceID: bug.WorkspaceID, Scene: scene, TeamID: bug.TeamID,
		ProjectID: bug.PlanUnitID, ActorUserID: effect.PrincipalID,
		RecipientUserIDs: recipients, Vars: vars, FallbackPlainText: notification.PlainText(scene, vars),
	})
}

func qualityToolNotificationRoute(effect quality.BugMutationEffect) (string, []uint, bool) {
	after := effect.After
	if effect.Operation == quality.BugCreate {
		recipients := make([]uint, 0, 2)
		if after.ReporterUserID != nil {
			recipients = append(recipients, *after.ReporterUserID)
		}
		if after.AssigneeUserID != nil {
			recipients = append(recipients, *after.AssigneeUserID)
		}
		return notification.SceneQualityBugCreated, recipients, true
	}
	if effect.Operation != quality.BugUpdate || effect.Before == nil {
		return "", nil, false
	}
	before := *effect.Before
	if after.Status == "reopened" && before.Status != "reopened" && after.AssigneeUserID != nil {
		return notification.SceneQualityBugReturned, []uint{*after.AssigneeUserID}, false
	}
	movedToReview := after.Status == "review" && before.Status != "review"
	backfilledResolution := after.Status == "review" && before.Status == "review" &&
		strings.TrimSpace(before.ResolutionReason) == "" && strings.TrimSpace(after.ResolutionReason) != ""
	if movedToReview || backfilledResolution {
		if after.ReviewerUserID != nil {
			return notification.SceneQualityBugReviewRequested, []uint{*after.ReviewerUserID}, true
		}
		if after.ReporterUserID != nil {
			return notification.SceneQualityBugReviewRequested, []uint{*after.ReporterUserID}, true
		}
	}
	if !sameToolUserID(before.AssigneeUserID, after.AssigneeUserID) && after.AssigneeUserID != nil {
		return notification.SceneQualityBugAssigned, []uint{*after.AssigneeUserID}, false
	}
	return "", nil, false
}

func sameToolUserID(left, right *uint) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

var _ quality.BugMutationEffectSink = qualityToolMutationEffects{}
