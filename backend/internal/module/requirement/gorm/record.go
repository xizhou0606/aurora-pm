package requirementgorm

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type UintList []uint

func (v UintList) Value() (driver.Value, error) {
	if v == nil {
		return "[]", nil
	}
	return json.Marshal(v)
}

func (v *UintList) Scan(value any) error {
	if value == nil {
		*v = UintList{}
		return nil
	}
	var raw []byte
	switch typed := value.(type) {
	case []byte:
		raw = typed
	case string:
		raw = []byte(typed)
	default:
		return fmt.Errorf("unsupported UintList value %T", value)
	}
	if len(raw) == 0 {
		*v = UintList{}
		return nil
	}
	return json.Unmarshal(raw, v)
}

type Requirement struct {
	ID           uint `gorm:"primaryKey" json:"id"`
	WorkspaceID  uint `gorm:"not null;index" json:"workspace_id"`
	TeamID       uint `gorm:"not null;index" json:"team_id"`
	ProjectSetID uint `gorm:"column:project_new_id;not null;index" json:"project_new_id"`
	// PlanUnitID remains the in-process compatibility field used throughout PM.
	// ScheduledPlanUnitID owns the nullable database column so pool requirements
	// can exist before they are committed to a version.
	PlanUnitID           uint      `gorm:"-" json:"-"`
	ScheduledPlanUnitID  *uint     `gorm:"column:version_task_new_id;index" json:"version_task_new_id,omitempty"`
	Version              uint      `gorm:"not null;default:1" json:"version"`
	WorkflowInstanceID   *uint     `gorm:"index" json:"workflow_instance_id,omitempty"`
	Title                string    `gorm:"size:220;not null" json:"title"`
	Description          string    `gorm:"type:text" json:"description,omitempty"`
	ProductDocURL        string    `gorm:"size:1000" json:"product_doc_url,omitempty"`
	ProductOwnerUserID   *uint     `gorm:"index" json:"product_owner_user_id,omitempty"`
	ProductOwnerUserIDs  UintList  `gorm:"type:text" json:"product_owner_user_ids"`
	DesignURL            string    `gorm:"size:1000" json:"design_url,omitempty"`
	DesignOwnerUserID    *uint     `gorm:"index" json:"design_owner_user_id,omitempty"`
	DesignOwnerUserIDs   UintList  `gorm:"type:text" json:"design_owner_user_ids"`
	FrontendOwnerUserID  *uint     `gorm:"index" json:"frontend_owner_user_id,omitempty"`
	FrontendOwnerUserIDs UintList  `gorm:"type:text" json:"frontend_owner_user_ids"`
	BackendOwnerUserID   *uint     `gorm:"index" json:"backend_owner_user_id,omitempty"`
	BackendOwnerUserIDs  UintList  `gorm:"type:text" json:"backend_owner_user_ids"`
	TesterUserIDs        UintList  `gorm:"type:text" json:"tester_user_ids"`
	DeveloperUserIDs     UintList  `gorm:"type:text" json:"developer_user_ids"`
	AcceptanceCriteria   string    `gorm:"type:text" json:"acceptance_criteria,omitempty"`
	Priority             string    `gorm:"size:30;not null;default:normal" json:"priority"`
	Status               string    `gorm:"size:30;not null;default:backlog" json:"status"`
	MutationState        string    `gorm:"size:24;not null;default:'';index" json:"-"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func UniqueOwnerIDs(ids UintList) UintList {
	seen := make(map[uint]struct{}, len(ids))
	out := make(UintList, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func FirstOwnerID(ids UintList, fallback *uint) *uint {
	for _, id := range UniqueOwnerIDs(ids) {
		value := id
		return &value
	}
	if fallback != nil && *fallback > 0 {
		value := *fallback
		return &value
	}
	return nil
}

func CoalesceOwnerIDs(ids UintList, fallback *uint) UintList {
	out := UniqueOwnerIDs(ids)
	if len(out) > 0 {
		return out
	}
	if fallback != nil && *fallback > 0 {
		return UintList{*fallback}
	}
	return UintList{}
}

func (r *Requirement) SyncRoleOwners() {
	r.ProductOwnerUserIDs = CoalesceOwnerIDs(r.ProductOwnerUserIDs, r.ProductOwnerUserID)
	r.ProductOwnerUserID = FirstOwnerID(r.ProductOwnerUserIDs, nil)
	r.DesignOwnerUserIDs = CoalesceOwnerIDs(r.DesignOwnerUserIDs, r.DesignOwnerUserID)
	r.DesignOwnerUserID = FirstOwnerID(r.DesignOwnerUserIDs, nil)
	r.FrontendOwnerUserIDs = CoalesceOwnerIDs(r.FrontendOwnerUserIDs, r.FrontendOwnerUserID)
	r.FrontendOwnerUserID = FirstOwnerID(r.FrontendOwnerUserIDs, nil)
	r.BackendOwnerUserIDs = CoalesceOwnerIDs(r.BackendOwnerUserIDs, r.BackendOwnerUserID)
	r.BackendOwnerUserID = FirstOwnerID(r.BackendOwnerUserIDs, nil)
	r.TesterUserIDs = UniqueOwnerIDs(r.TesterUserIDs)
	r.DeveloperUserIDs = UniqueOwnerIDs(r.DeveloperUserIDs)
}

func (r *Requirement) BeforeSave(_ *gorm.DB) error {
	if r.PlanUnitID > 0 {
		value := r.PlanUnitID
		r.ScheduledPlanUnitID = &value
	}
	r.SyncRoleOwners()
	return nil
}

func (r *Requirement) AfterFind(_ *gorm.DB) error {
	if r.ScheduledPlanUnitID == nil {
		r.PlanUnitID = 0
		return nil
	}
	r.PlanUnitID = *r.ScheduledPlanUnitID
	return nil
}
