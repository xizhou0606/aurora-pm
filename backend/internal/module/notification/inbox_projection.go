package notification

import (
	"fmt"
	"time"
)

// InboxProjection exposes a closed set of display fields, never arbitrary
// template variables, comment bodies, external URLs or authentication secrets.
func InboxProjection(intent Intent, id, recipientID uint, occurredAt time.Time, readAt *time.Time) Mention {
	item := Mention{ID: id, Scene: intent.Scene, ResourceType: intent.ResourceType, ResourceID: intent.ResourceID, DocumentID: intent.DocumentID, ThreadID: intent.ThreadID, CommentID: intent.CommentID, RequestID: intent.RequestID, ActorUserID: intent.ActorUserID, OccurredAt: occurredAt, ReadAt: readAt}
	if InboxCategory(intent.Scene) != "message" {
		return item
	}
	item.ActorName = intent.Vars["operator"]
	item.ResourceTitle = intent.Vars["title"]
	item.ProjectID = intent.ProjectID
	switch intent.Scene {
	case SceneWorkflowNodeAssigned, SceneWorkflowPreReviewInvited:
		item.NodeID, item.NodeName = intent.Vars["nodeId"], intent.Vars["node"]
	case ScenePlanUnitMemberAssigned:
		item.ResourceTitle, item.MemberRole = intent.Vars["unit"], intent.Vars["roleKey"]
	case SceneWorkTaskStatusChanged, SceneQualityBugReviewRequested, SceneQualityBugReturned:
		item.PreviousStatus, item.Status = intent.Vars["fromStatusKey"], intent.Vars["toStatusKey"]
	case SceneWorkTaskAssigned, SceneQualityBugCreated, SceneQualityBugAssigned:
		item.Status = intent.Vars["statusKey"]
	}
	if intent.Scene == SceneQualityBugAssigned && intent.Vars["assigneeUserId"] != "" {
		item.RecipientRole = "cc"
		if intent.Vars["assigneeUserId"] == fmt.Sprint(recipientID) {
			item.RecipientRole = "assignee"
		}
	}
	return item
}

// WorkflowResourceID preserves the project/version route tuple.
func WorkflowResourceID(subjectType string, projectID, subjectID uint) string {
	if subjectType == "plan_unit" {
		return fmt.Sprintf("%d/%d", projectID, subjectID)
	}
	return fmt.Sprint(subjectID)
}
