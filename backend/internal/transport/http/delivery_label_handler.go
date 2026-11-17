package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"gorm.io/gorm"
)

type deliveryLabelInput struct {
	ClientKey string `json:"client_key"`
	Name      string `json:"name"`
	SystemKey string `json:"system_key"`
	SortOrder int    `json:"sort_order"`
	Deleted   bool   `json:"deleted"`
}

func (s *Server) labelPlan(c *gin.Context) (projectgorm.PlanUnit, projectgorm.ProjectSet, bool) {
	id, okID := paramID(c, "id")
	if !okID {
		return projectgorm.PlanUnit{}, projectgorm.ProjectSet{}, false
	}
	workspace, _ := currentWorkspace(c)
	var unit projectgorm.PlanUnit
	if s.db.Where("id = ? AND workspace_id = ?", id, workspace.ID).First(&unit).Error != nil {
		fail(c, http.StatusNotFound, "plan unit not found")
		return projectgorm.PlanUnit{}, projectgorm.ProjectSet{}, false
	}
	var project projectgorm.ProjectSet
	if s.db.Where("id = ? AND workspace_id = ?", unit.ProjectSetID, workspace.ID).First(&project).Error != nil {
		fail(c, http.StatusNotFound, "project set not found")
		return projectgorm.PlanUnit{}, projectgorm.ProjectSet{}, false
	}
	if !s.requireProjectAccess(c, workspace.ID, project.ID) {
		return projectgorm.PlanUnit{}, projectgorm.ProjectSet{}, false
	}
	return unit, project, true
}

func (s *Server) listDeliveryLabels(c *gin.Context) {
	unit, project, allowed := s.labelPlan(c)
	if !allowed {
		return
	}
	var labels []projectgorm.DeliveryNodeLabel
	if s.db.Where("workspace_id = ? AND project_set_id = ? AND plan_unit_id = ? AND deleted = ?",
		project.WorkspaceID, project.ID, unit.ID, false).
		Order("sort_order ASC, id ASC").Find(&labels).Error != nil {
		dbFail(c)
		return
	}
	ok(c, labels)
}

func (s *Server) createDeliveryLabel(c *gin.Context) {
	unit, project, allowed := s.labelPlan(c)
	if !allowed {
		return
	}
	var body deliveryLabelInput
	if c.ShouldBindJSON(&body) != nil || strings.TrimSpace(body.Name) == "" {
		bad(c, "name required")
		return
	}
	label := projectgorm.DeliveryNodeLabel{
		WorkspaceID: project.WorkspaceID, ProjectSetID: project.ID, PlanUnitID: unit.ID,
		ClientKey: strings.TrimSpace(body.ClientKey), Name: strings.TrimSpace(body.Name),
		SystemKey: strings.TrimSpace(body.SystemKey), SortOrder: body.SortOrder,
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		lockedProject, err := lockProjectSetTx(tx, project.WorkspaceID, project.ID)
		if err != nil {
			return err
		}
		project = lockedProject
		if err := tx.Create(&label).Error; err != nil {
			return err
		}
		return s.recordMutation(tx, c, project.ID, "delivery_label", label.ID, "created", label.Name, label)
	})
	if err != nil {
		dbFail(c)
		return
	}
	created(c, label)
}

func (s *Server) patchDeliveryLabel(c *gin.Context) {
	label, project, allowed := s.deliveryLabelByID(c)
	if !allowed {
		return
	}
	var body deliveryLabelInput
	if c.ShouldBindJSON(&body) != nil || strings.TrimSpace(body.Name) == "" {
		bad(c, "name required")
		return
	}
	label.Name = strings.TrimSpace(body.Name)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&label).Update("name", label.Name).Error; err != nil {
			return err
		}
		return s.recordMutation(tx, c, project.ID, "delivery_label", label.ID, "updated", label.Name, label)
	})
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, label)
}

func (s *Server) deleteDeliveryLabel(c *gin.Context) {
	label, project, allowed := s.deliveryLabelByID(c)
	if !allowed {
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&label).Update("deleted", true).Error; err != nil {
			return err
		}
		return s.recordMutation(tx, c, project.ID, "delivery_label", label.ID, "deleted", label.Name, label)
	})
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{"deleted": true})
}

func (s *Server) deliveryLabelByID(c *gin.Context) (projectgorm.DeliveryNodeLabel, projectgorm.ProjectSet, bool) {
	id, okID := paramID(c, "id")
	if !okID {
		return projectgorm.DeliveryNodeLabel{}, projectgorm.ProjectSet{}, false
	}
	workspace, _ := currentWorkspace(c)
	var label projectgorm.DeliveryNodeLabel
	if s.db.Where("id = ? AND workspace_id = ?", id, workspace.ID).First(&label).Error != nil {
		fail(c, http.StatusNotFound, "delivery label not found")
		return projectgorm.DeliveryNodeLabel{}, projectgorm.ProjectSet{}, false
	}
	var project projectgorm.ProjectSet
	if s.db.Where("id = ? AND workspace_id = ?", label.ProjectSetID, workspace.ID).First(&project).Error != nil {
		fail(c, http.StatusNotFound, "project set not found")
		return projectgorm.DeliveryNodeLabel{}, projectgorm.ProjectSet{}, false
	}
	if !s.requireProjectAccess(c, workspace.ID, project.ID) {
		return projectgorm.DeliveryNodeLabel{}, projectgorm.ProjectSet{}, false
	}
	var unitCount int64
	s.db.Model(&projectgorm.PlanUnit{}).
		Where("id = ? AND workspace_id = ? AND project_new_id = ?",
			label.PlanUnitID, workspace.ID, project.ID).
		Count(&unitCount)
	if unitCount != 1 {
		fail(c, http.StatusNotFound, "plan unit not found")
		return projectgorm.DeliveryNodeLabel{}, projectgorm.ProjectSet{}, false
	}
	var unit projectgorm.PlanUnit
	if s.db.Where("id = ? AND workspace_id = ?", label.PlanUnitID, workspace.ID).First(&unit).Error != nil {
		fail(c, http.StatusNotFound, "plan unit not found")
		return projectgorm.DeliveryNodeLabel{}, projectgorm.ProjectSet{}, false
	}
	return label, project, true
}
