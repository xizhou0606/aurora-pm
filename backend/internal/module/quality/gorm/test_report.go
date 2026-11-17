package qualitygorm

import "time"

// PlanUnitTestReport stores version-scoped QA reports. Ownership is the plan
// unit; project_set_id is a denormalized filter column only.
type PlanUnitTestReport struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID    uint      `gorm:"not null;index:idx_plan_unit_test_report_scope,priority:1;index" json:"workspace_id"`
	PlanUnitID     uint      `gorm:"not null;index:idx_plan_unit_test_report_scope,priority:2;index" json:"plan_unit_id"`
	ProjectSetID   uint      `gorm:"not null;index" json:"project_set_id"`
	Kind           string    `gorm:"size:16;not null;index" json:"kind"`
	Name           string    `gorm:"size:220;not null" json:"name"`
	ContentType    string    `gorm:"size:160;not null" json:"content_type"`
	Size           int64     `gorm:"not null" json:"size"`
	ObjectKey      string    `gorm:"size:1000;not null;uniqueIndex" json:"object_key"`
	URL            string    `gorm:"size:2000;not null" json:"url"`
	UploaderUserID uint      `gorm:"not null;index" json:"uploader_user_id"`
	CreatedAt      time.Time `json:"created_at"`
}

func (PlanUnitTestReport) TableName() string { return "plan_unit_test_reports" }
