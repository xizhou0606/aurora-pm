package task

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"strconv"
)

// StatusNotification is built from persisted before/after facts inside the caller's
// transaction. GUI and Tool adapters share recipient and no-op semantics here.
func StatusNotification(row Snapshot, previous string, actorID uint, operator, baseURL string, externalEnabled bool) (notification.Intent, bool) {
	if previous == "" || previous == row.Status {
		return notification.Intent{}, false
	}
	recipients := []uint{}
	seen := map[uint]bool{0: true, actorID: true}
	for _, id := range []*uint{row.AssigneeID, row.ReporterUserID} {
		if id != nil && !seen[*id] {
			recipients = append(recipients, *id)
			seen[*id] = true
		}
	}
	if len(recipients) == 0 {
		return notification.Intent{}, false
	}
	vars := map[string]string{"operator": operator, "title": row.Title, "taskKey": row.TaskKey,
		"fromStatus": WorkTaskStatusLabel(previous), "toStatus": WorkTaskStatusLabel(row.Status),
		"fromStatusKey": previous, "toStatusKey": row.Status, "url": notification.WorkItemURL(baseURL, row.ID)}
	return notification.Intent{WorkspaceID: row.WorkspaceID, TeamID: row.TeamID, ProjectID: row.PlanUnitID,
		Scene: notification.SceneWorkTaskStatusChanged, ActorUserID: actorID, RecipientUserIDs: recipients,
		ResourceType: "work_task", ResourceID: strconv.FormatUint(uint64(row.ID), 10), SkipExternal: !externalEnabled,
		Vars: vars, FallbackPlainText: notification.PlainText(notification.SceneWorkTaskStatusChanged, vars)}, true
}
