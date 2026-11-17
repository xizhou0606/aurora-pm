package notification

import (
	"fmt"
	"gorm.io/gorm"
	"strconv"
	"strings"
)

// EnqueueCommentMention gives every comment/recipient one notification across channels.
// Producers resolve and validate mentioned users inside the authenticated workspace.
func EnqueueCommentMention(tx *gorm.DB, workspaceID, actorID, teamID, projectID, resourceID, commentID uint, resourceType string, recipients []uint, publicBase string, externalEnabled bool, actorName, resourceTitle string) error {
	scene, url, label := SceneWorkTaskCommentMentioned, WorkItemURL(publicBase, resourceID), "任务"
	if resourceType == "quality_bug" {
		scene, url, label = SceneQualityCommentMentioned, QualityBugURL(publicBase, resourceID), "质量工单"
	}
	filtered := make([]uint, 0, len(recipients))
	for _, id := range dedupeUserIDs(recipients) {
		if id != actorID {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	if commentID == 0 {
		return fmt.Errorf("mention comment identity is required")
	}
	if url != "" {
		url += fmt.Sprintf("#comment-%d", commentID)
	}
	actorName = strings.TrimSpace(actorName)
	if actorName == "" {
		actorName = fmt.Sprintf("用户 #%d", actorID)
	}
	area := label
	if title := strings.TrimSpace(resourceTitle); title != "" {
		area += "「" + title + "」"
	}
	return Enqueue(tx, resourceType+"_comment", commentID, Intent{WorkspaceID: workspaceID, ActorUserID: actorID, TeamID: teamID, ProjectID: projectID,
		Scene: scene, ResourceType: resourceType, ResourceID: strconv.FormatUint(uint64(resourceID), 10), CommentID: commentID,
		RecipientUserIDs: filtered, SkipExternal: !externalEnabled, Vars: map[string]string{"url": url, "notification_event_key": fmt.Sprintf("comment:%d", commentID), "operator": actorName, "title": resourceTitle},
		FallbackPlainText: fmt.Sprintf("%s在%s评论中提到了你\n%s", actorName, area, url)})
}
