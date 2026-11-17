package httpapi

import (
	"fmt"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
)

// recordPlanUnitMemberNotifications enqueues one IM notification per newly
// assigned owner / added participant of a plan unit. Assignments are real
// hand-offs, so a self-assignment still notifies. Called after a successful
// member replacement; every event joins its own transaction via Enqueue.
func (s *Server) recordPlanUnitMemberNotifications(
	tx *gorm.DB,
	proj projectgorm.ProjectSet,
	unit projectgorm.PlanUnit,
	before, after project.MemberState,
	actorID uint,
) error {
	// New owner (the transfer target, not the previous owner) and newly added
	// participants are enqueued separately so each message carries the
	// recipient's own role.
	var newOwner uint
	if after.OwnerUserID != nil && *after.OwnerUserID > 0 &&
		(before.OwnerUserID == nil || *before.OwnerUserID != *after.OwnerUserID) {
		newOwner = *after.OwnerUserID
	}
	previous := make(map[uint]struct{}, len(before.ParticipantUserIDs))
	for _, id := range before.ParticipantUserIDs {
		previous[id] = struct{}{}
	}
	added := make([]uint, 0, len(after.ParticipantUserIDs))
	for _, id := range after.ParticipantUserIDs {
		if _, existed := previous[id]; !existed {
			added = append(added, id)
		}
	}
	if newOwner == 0 && len(added) == 0 {
		return nil
	}

	ids := append([]uint{actorID}, added...)
	if newOwner > 0 {
		ids = append(ids, newOwner)
	}
	ctx := notificationUsersTx(tx, ids...)
	baseVars := map[string]string{
		"projectSet": proj.Name,
		"unit":       unit.Title,
		"operator":   notificationPeopleLabel(tx, ctx, actorID),
		"url":        notification.PlanUnitURL(s.cfg.Notification.PublicBaseURL, proj.ID, unit.ID),
	}

	enqueue := func(recipient uint, role string) error {
		vars := map[string]string{
			"projectSet": baseVars["projectSet"], "unit": baseVars["unit"],
			"operator": baseVars["operator"], "url": baseVars["url"],
			"role": role,
		}
		vars["roleKey"] = "participant"
		if role == "负责人" {
			vars["roleKey"] = "owner"
		}
		return notification.Enqueue(tx, "plan_unit_notification", unit.ID, notification.Intent{
			SkipExternal: !s.cfg.Notification.Enabled, ResourceType: "plan_unit", ResourceID: fmt.Sprintf("%d/%d", proj.ID, unit.ID),
			WorkspaceID: unit.WorkspaceID, Scene: notification.ScenePlanUnitMemberAssigned,
			TeamID: unit.TeamID, ProjectID: proj.ID, ActorUserID: actorID,
			RecipientUserIDs: []uint{recipient}, Vars: vars,
			FallbackPlainText: notification.PlainText(notification.ScenePlanUnitMemberAssigned, vars),
		})
	}
	if newOwner > 0 {
		if err := enqueue(newOwner, "负责人"); err != nil {
			return err
		}
	}
	for _, id := range added {
		if err := enqueue(id, "参与人"); err != nil {
			return err
		}
	}
	return nil
}
