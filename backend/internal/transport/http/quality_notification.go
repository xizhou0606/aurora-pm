package httpapi

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
)

func (s *Server) recordQualityBugNotification(
	tx *gorm.DB,
	c *gin.Context,
	bug qualitygorm.QualityBug,
	ctx workspaceContext,
	scene string,
	recipientIDs []uint,
	ccUserIDs []uint,
	note string,
	includeActor bool,
	previousStatus ...string,
) error {
	actorID := currentFactUserID(c)
	recipients := qualityNotificationRecipients(actorID, recipientIDs, includeActor)
	if len(recipients) == 0 {
		return nil
	}
	vars := s.qualityNotificationVars(c, bug, ctx, ccUserIDs, note)
	vars["operator"] = notificationPeopleLabel(tx, ctx, actorID)
	if len(previousStatus) > 0 {
		vars["fromStatusKey"] = previousStatus[0]
		vars["toStatusKey"] = bug.Status
	}
	fallback := qualityNotificationPlainText(scene, vars)
	return notification.Enqueue(tx, "quality_bug_notification", bug.ID, notification.Intent{
		SkipExternal: !s.cfg.Notification.Enabled, ResourceType: "quality_bug", ResourceID: fmt.Sprint(bug.ID),
		WorkspaceID: bug.WorkspaceID, Scene: scene, TeamID: bug.TeamID,
		ProjectID: bug.ProjectID, ActorUserID: actorID,
		RecipientUserIDs: recipients, Vars: vars, FallbackPlainText: fallback,
	})
}

func (s *Server) recordQualityPatchNotification(
	tx *gorm.DB,
	c *gin.Context,
	before, after qualitygorm.QualityBug,
	payload qualityWorkflowPayload,
	ctx workspaceContext,
) error {
	oldStatus := normalizeQualityBugStatus(before.Status)
	newStatus := normalizeQualityBugStatus(after.Status)
	movedToReview := newStatus == "review" && oldStatus != "review"
	backfilledResolution := newStatus == "review" && oldStatus == "review" &&
		strings.TrimSpace(before.ResolutionReason) == "" && strings.TrimSpace(after.ResolutionReason) != ""

	reopened := newStatus == "reopened" && oldStatus != "reopened"
	legacyReturnToProgress := payload.IsRegressionReturn && newStatus == "in_progress"
	if (reopened || legacyReturnToProgress) && after.AssigneeUserID != nil {
		return s.recordQualityBugNotification(
			tx, c, after, ctx, notification.SceneQualityBugReturned,
			[]uint{*after.AssigneeUserID}, nil, payload.RegressionNote, false, oldStatus,
		)
	}
	if movedToReview || backfilledResolution {
		reviewerID := uint(0)
		if after.ReviewerUserID != nil {
			reviewerID = *after.ReviewerUserID
		} else if after.ReporterUserID != nil {
			reviewerID = *after.ReporterUserID
		}
		return s.recordQualityBugNotification(
			tx, c, after, ctx, notification.SceneQualityBugReviewRequested,
			[]uint{reviewerID}, nil, "", true, oldStatus,
		)
	}
	if (payload.AssignmentTargetChange && after.AssigneeUserID != nil) || len(payload.CCUserIDs) > 0 {
		recipients := append([]uint(nil), payload.CCUserIDs...)
		if payload.AssignmentTargetChange && after.AssigneeUserID != nil {
			recipients = append([]uint{*after.AssigneeUserID}, recipients...)
		}
		return s.recordQualityBugNotification(
			tx, c, after, ctx, notification.SceneQualityBugAssigned,
			recipients, payload.CCUserIDs, payload.AssignmentNote, false,
		)
	}
	return nil
}

func (s *Server) recordQualityCommentNotification(
	tx *gorm.DB,
	c *gin.Context,
	bug qualitygorm.QualityBug,
	comment qualitygorm.QualityBugComment,
	ctx workspaceContext,
) error {
	validated, err := commentMentionWorkspaceRecipients(tx, bug.WorkspaceID, comment.Body)
	if err != nil {
		return err
	}
	if err := notification.EnqueueCommentMention(tx, bug.WorkspaceID, comment.UserID, bug.TeamID, bug.ProjectID, bug.ID, comment.ID, "quality_bug", validated, s.cfg.Notification.PublicBaseURL, s.cfg.Notification.Enabled, notificationPeopleLabel(tx, ctx, comment.UserID), bug.Title); err != nil {
		return err
	}
	var participantIDs []uint
	if err := tx.Model(&qualitygorm.QualityBugComment{}).
		Where("workspace_id = ? AND quality_bug_id = ?", bug.WorkspaceID, bug.ID).
		Distinct("user_id").Pluck("user_id", &participantIDs).Error; err != nil {
		return err
	}
	direct := make([]uint, 0, 3)
	for _, userID := range []*uint{bug.AssigneeUserID, bug.ReporterUserID, bug.ReviewerUserID} {
		if userID != nil {
			direct = append(direct, *userID)
		}
	}
	recipients := commentNotificationRecipients(
		comment.UserID, direct, participantIDs, nil,
	)
	mentionSet := map[uint]bool{}
	for _, id := range validated {
		mentionSet[id] = true
	}
	remaining := recipients[:0]
	for _, id := range recipients {
		if !mentionSet[id] {
			remaining = append(remaining, id)
		}
	}
	recipients = remaining
	if len(recipients) == 0 {
		return nil
	}
	vars := s.qualityNotificationVars(c, bug, ctx, nil, "")
	vars["url"] += fmt.Sprintf("#comment-%d", comment.ID)
	vars["comment"] = commentNotificationExcerpt(comment.Body)
	vars["operator"] = notificationPeopleLabel(tx, ctx, comment.UserID)
	return notification.Enqueue(tx, "quality_bug_notification", bug.ID, notification.Intent{
		SkipExternal: !s.cfg.Notification.Enabled, ResourceType: "quality_bug", ResourceID: fmt.Sprint(bug.ID),
		CommentID: comment.ID, WorkspaceID: bug.WorkspaceID, Scene: notification.SceneQualityBugCommented,
		TeamID: bug.TeamID, ProjectID: bug.ProjectID, ActorUserID: comment.UserID,
		RecipientUserIDs: recipients, Vars: vars,
		FallbackPlainText: qualityNotificationPlainText(notification.SceneQualityBugCommented, vars),
	})
}

func (s *Server) qualityNotificationVars(
	c *gin.Context,
	bug qualitygorm.QualityBug,
	ctx workspaceContext,
	ccUserIDs []uint,
	note string,
) map[string]string {
	operatorID := currentFactUserID(c)
	assigneeID := uint(0)
	if bug.AssigneeUserID != nil {
		assigneeID = *bug.AssigneeUserID
	}
	vars := map[string]string{
		"operator": qualityNotificationUserLabel(ctx, &operatorID),
		"title":    strings.TrimSpace(bug.Title),
		"bugKey":   strings.TrimSpace(bug.BugKey), "severity": qualitySeverityLabel(bug.SeverityKey),
		"status":           qualityStatusLabel(bug.Status),
		"statusKey":        bug.Status,
		"resolutionReason": qualityResolutionReasonLabel(bug.ResolutionReason),
		"assignee":         qualityNotificationUserLabel(ctx, bug.AssigneeUserID),
		"assigneeUserId":   fmt.Sprint(assigneeID),
		"reviewer":         qualityNotificationUserLabel(ctx, bug.ReviewerUserID),
		"ccUsers":          qualityNotificationUserListLabel(ctx, ccUserIDs),
		"note":             quality.RichTextPlainText(note), "url": s.qualityNotificationURL(bug.ID),
	}
	if vars["title"] == "" {
		vars["title"] = "未命名缺陷"
	}
	if vars["bugKey"] == "" {
		vars["bugKey"] = fmt.Sprintf("BUG-%d", bug.ID)
	}
	for _, key := range []string{"severity", "status", "resolutionReason", "assignee", "reviewer", "note"} {
		if strings.TrimSpace(vars[key]) == "" {
			vars[key] = "-"
		}
	}
	return vars
}

func qualityNotificationRecipients(actorID uint, ids []uint, includeActor bool) []uint {
	seen := map[uint]struct{}{}
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 || (!includeActor && actorID > 0 && id == actorID) {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func qualityNotificationUserLabel(ctx workspaceContext, id *uint) string {
	if id == nil || *id == 0 {
		return "-"
	}
	if label := strings.TrimSpace(userDisplayName(ctx, id)); label != "" {
		return label
	}
	return retiredUserDisplayName(*id)
}

func qualityNotificationUserListLabel(ctx workspaceContext, ids []uint) string {
	parts := make([]string, 0, len(ids))
	for _, id := range qualityNotificationRecipients(0, ids, true) {
		parts = append(parts, qualityNotificationUserLabel(ctx, &id))
	}
	return strings.Join(parts, "、")
}

func (s *Server) qualityNotificationURL(bugID uint) string {
	return notification.QualityBugURL(s.cfg.Notification.PublicBaseURL, bugID)
}

func qualityNotificationPlainText(scene string, vars map[string]string) string {
	return notification.PlainText(scene, vars)
}
