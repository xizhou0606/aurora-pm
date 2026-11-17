package resourceaccessgorm

import (
	"encoding/json"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"gorm.io/gorm"
)

func appendAuditAndOutbox(
	tx *gorm.DB,
	principal resourceaccess.Principal,
	resourceID documentlibrary.ResourceID,
	eventType string,
	before, after any,
	now time.Time,
) error {
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return err
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	audit := AuditRecord{
		WorkspaceID: principal.WorkspaceID, ResourceID: string(resourceID), EventType: eventType,
		ActorUserID: principal.UserID, BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON), CreatedAt: now,
	}
	if err := tx.Create(&audit).Error; err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"workspace_id": principal.WorkspaceID, "resource_id": resourceID,
		"event_type": eventType, "actor_user_id": principal.UserID,
	})
	if err != nil {
		return err
	}
	return tx.Create(&OutboxRecord{
		WorkspaceID: principal.WorkspaceID, ResourceID: string(resourceID),
		EventType: eventType, PayloadJSON: string(payload), CreatedAt: now,
	}).Error
}
