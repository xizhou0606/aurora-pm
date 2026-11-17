package notification

import "strings"

const EventTypePrefix = "pm.notification."

const (
	SceneWorkspaceAccessRequested       = "workspace.access_requested"
	SceneWorkspaceAccessReviewed        = "workspace.access_reviewed"
	SceneProjectAccessRequested         = "project.access_requested"
	SceneProjectAccessReviewed          = "project.access_reviewed"
	SceneWorkTaskCommentMentioned       = "work_task.comment_mentioned"
	SceneQualityCommentMentioned        = "quality_bug.comment_mentioned"
	SceneDocumentAccessRequested        = "document.access_requested"
	SceneDocumentAccessReviewed         = "document.access_reviewed"
	SceneDocumentCommentMentioned       = "document.comment_mentioned"
	SceneQualityBugCreated              = "quality_bug.created"
	SceneQualityBugAssigned             = "quality_bug.assigned"
	SceneQualityBugReviewRequested      = "quality_bug.review_requested"
	SceneQualityBugReturned             = "quality_bug.returned"
	SceneQualityBugCommented            = "quality_bug.commented"
	SceneWorkTaskStatusChanged          = "work_task.status_changed"
	SceneWorkTaskAssigned               = "work_task.assigned"
	SceneWorkTaskCommented              = "work_task.commented"
	SceneWorkflowNodeAssigned           = "workflow.node_assigned"
	SceneWorkflowPreReviewInvited       = "workflow.pre_review_invited"
	ScenePlanUnitMemberAssigned         = "plan_unit.member_assigned"
	SceneIdentityPasswordResetRequested = "identity.password_reset_requested"
)

func AllowsGlobalScope(scene string) bool {
	return strings.TrimSpace(scene) == SceneIdentityPasswordResetRequested
}

type Intent struct {
	CommentID         uint              `json:"comment_id,omitempty"`
	SkipExternal      bool              `json:"skip_external,omitempty"`
	ResourceType      string            `json:"resource_type,omitempty"`
	ResourceID        string            `json:"resource_id,omitempty"`
	RequestID         uint              `json:"request_id,omitempty"`
	DocumentID        uint              `json:"document_id,omitempty"`
	ThreadID          uint              `json:"thread_id,omitempty"`
	WorkspaceID       uint              `json:"workspace_id"`
	Scene             string            `json:"scene"`
	TeamID            uint              `json:"team_id"`
	ProjectID         uint              `json:"project_id"`
	ActorUserID       uint              `json:"actor_user_id"`
	RecipientUserIDs  []uint            `json:"recipient_user_ids"`
	Vars              map[string]string `json:"vars"`
	FallbackPlainText string            `json:"fallback_plain_text"`
}

// ValidScope keeps workspace-only document scenes distinct from project scenes.
func ValidScope(intent Intent) bool {
	if intent.WorkspaceID == 0 {
		return false
	}
	if (intent.Scene == SceneProjectAccessRequested || intent.Scene == SceneProjectAccessReviewed) && intent.ResourceType == "project_set" {
		return intent.TeamID > 0 && intent.ProjectID == 0 && intent.RequestID > 0
	}
	if intent.Scene == SceneWorkspaceAccessRequested || intent.Scene == SceneWorkspaceAccessReviewed {
		return intent.TeamID == 0 && intent.ProjectID == 0 && intent.ResourceType == "workspace" && intent.RequestID > 0
	}
	if AllowsGlobalScope(intent.Scene) {
		return intent.TeamID == 0 && intent.ProjectID == 0 && intent.DocumentID == 0 && intent.ThreadID == 0
	}
	if intent.Scene == SceneDocumentAccessRequested || intent.Scene == SceneDocumentAccessReviewed {
		return intent.TeamID == 0 && intent.ProjectID == 0 && strings.TrimSpace(intent.Vars["resourceId"]) != ""
	}
	if intent.Scene == SceneDocumentCommentMentioned {
		return intent.DocumentID > 0 && intent.TeamID == 0 && intent.ProjectID == 0
	}
	return intent.TeamID > 0 && intent.ProjectID > 0 && intent.DocumentID == 0
}

func EventType(scene string) string {
	return EventTypePrefix + strings.TrimSpace(scene)
}

func IsNotificationEvent(eventType string) bool {
	return strings.HasPrefix(strings.TrimSpace(eventType), EventTypePrefix)
}

func dedupeUserIDs(values []uint) []uint {
	seen := map[uint]struct{}{}
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
