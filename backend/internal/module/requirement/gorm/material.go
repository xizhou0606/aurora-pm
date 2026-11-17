package requirementgorm

import "time"

// RequirementMaterial is the canonical store for design drafts and test-case
// files that belong to a requirement, independent of workflow node state.
type RequirementMaterial struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID    uint      `gorm:"not null;index:idx_requirement_material_scope,priority:1;index" json:"workspace_id"`
	RequirementID  uint      `gorm:"not null;index:idx_requirement_material_scope,priority:2;index" json:"requirement_id"`
	Kind           string    `gorm:"size:32;not null;index:idx_requirement_material_scope,priority:3;index" json:"kind"`
	Source         string    `gorm:"size:32;not null;index" json:"source"`
	Name           string    `gorm:"size:220;not null" json:"name"`
	ContentType    string    `gorm:"size:160;not null" json:"content_type"`
	Size           int64     `gorm:"not null" json:"size"`
	ObjectKey      string    `gorm:"size:1000;not null;uniqueIndex" json:"object_key"`
	URL            string    `gorm:"size:2000;not null" json:"url"`
	AIRunID        *uint     `gorm:"index" json:"ai_run_id,omitempty"`
	UploaderUserID uint      `gorm:"not null;index" json:"uploader_user_id"`
	CreatedAt      time.Time `json:"created_at"`
}

func (RequirementMaterial) TableName() string { return "requirement_materials" }
