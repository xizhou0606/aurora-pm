package records

import "time"

// InboxReceipt owns recipient state only; message facts remain in outbox_events.
type InboxReceipt struct {
	ID              uint   `gorm:"primaryKey"`
	WorkspaceID     uint   `gorm:"not null;uniqueIndex:ux_notification_receipt_event,priority:1;uniqueIndex:ux_notification_receipt_key,priority:1;index:ix_notification_receipt_category,priority:1"`
	RecipientUserID uint   `gorm:"not null;uniqueIndex:ux_notification_receipt_event,priority:2;uniqueIndex:ux_notification_receipt_key,priority:2;index:ix_notification_receipt_category,priority:2"`
	EventID         uint   `gorm:"not null;uniqueIndex:ux_notification_receipt_event,priority:3"`
	EventKey        string `gorm:"not null;uniqueIndex:ux_notification_receipt_key,priority:3"`
	Category        string `gorm:"not null;index:ix_notification_receipt_category,priority:3"`
	ReadAt          *time.Time
}

func (InboxReceipt) TableName() string { return "notification_inbox_receipts" }
