package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	workbenchgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

var errBITeamMetricSettingsVersionConflict = errors.New("BI team metric settings version conflict")

type biTeamMetricSettingsRequest struct {
	ExpectedVersion               *int   `json:"expected_version"`
	RequirementWorkflowTemplateID uint   `json:"requirement_workflow_template_id"`
	ApprovalNodeID                string `json:"approval_node_id"`
}

type biTeamMetricSettingsDTO struct {
	Version                       int                          `json:"version"`
	Configured                    bool                         `json:"configured"`
	Valid                         bool                         `json:"valid"`
	RequirementWorkflowTemplateID uint                         `json:"requirement_workflow_template_id,omitempty"`
	RequirementWorkflowTemplate   string                       `json:"requirement_workflow_template_name,omitempty"`
	RequirementWorkflowVersion    uint                         `json:"requirement_workflow_published_version,omitempty"`
	ApprovalNodeID                string                       `json:"approval_node_id,omitempty"`
	ApprovalNodeName              string                       `json:"approval_node_name,omitempty"`
	UpdatedByUserID               uint                         `json:"updated_by_user_id,omitempty"`
	Options                       []biTeamMetricWorkflowOption `json:"options"`
}

type biTeamMetricWorkflowOption struct {
	TemplateID       uint                             `json:"template_id"`
	TemplateName     string                           `json:"template_name"`
	PublishedVersion uint                             `json:"published_version"`
	ApprovalNodes    []biTeamMetricApprovalNodeOption `json:"approval_nodes"`
}

type biTeamMetricApprovalNodeOption struct {
	NodeID   string `json:"node_id"`
	NodeName string `json:"node_name"`
}

func (s *Server) getBITeamMetricSettings(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	settings, err := s.loadBITeamMetricSettings(workspace.ID)
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, settings)
}

func (s *Server) loadBITeamMetricSettings(workspaceID uint) (biTeamMetricSettingsDTO, error) {
	options, err := s.loadBITeamMetricWorkflowOptions(workspaceID)
	if err != nil {
		return biTeamMetricSettingsDTO{}, err
	}
	dto, err := s.loadBITeamMetricSettingsState(workspaceID)
	dto.Options = options
	return dto, err
}

func (s *Server) loadBITeamMetricSettingsState(workspaceID uint) (biTeamMetricSettingsDTO, error) {
	var row workbenchgorm.BITeamMetricSettings
	err := s.db.Where("workspace_id = ?", workspaceID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return biTeamMetricSettingsDTO{}, nil
	}
	if err != nil {
		return biTeamMetricSettingsDTO{}, err
	}
	dto := biTeamMetricSettingsDTO{
		Version: row.Version, Configured: true,
		RequirementWorkflowTemplateID: row.RequirementWorkflowTemplateID,
		ApprovalNodeID:                row.ApprovalNodeID, UpdatedByUserID: row.UpdatedByUserID,
	}
	template, version, definition, loadErr := s.loadPublishedWorkflow(
		workspaceID, row.RequirementWorkflowTemplateID, "requirement",
	)
	if loadErr != nil {
		if errors.Is(loadErr, errPublishedWorkflowNotFound) || errors.Is(loadErr, errPublishedWorkflowInvalid) {
			return dto, nil
		}
		return dto, loadErr
	}
	dto.RequirementWorkflowTemplate = template.Name
	dto.RequirementWorkflowVersion = version.Version
	if node, found := workflowNodeByID(definition, row.ApprovalNodeID); found && node.Type == "approval" {
		dto.Valid = true
		dto.ApprovalNodeName = node.Name
	}
	return dto, nil
}

func (s *Server) loadBITeamMetricWorkflowOptions(workspaceID uint) ([]biTeamMetricWorkflowOption, error) {
	var templates []workflowgorm.WorkflowTemplate
	if err := s.db.Where(
		"workspace_id = ? AND subject_type = ? AND published_version > ?", workspaceID, "requirement", 0,
	).Order("name ASC, id ASC").Find(&templates).Error; err != nil {
		return nil, err
	}
	options := make([]biTeamMetricWorkflowOption, 0, len(templates))
	for _, template := range templates {
		var version workflowgorm.WorkflowTemplateVersion
		if err := s.db.Where(
			"workspace_id = ? AND template_id = ? AND version = ?",
			workspaceID, template.ID, template.PublishedVersion,
		).First(&version).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		definition, issues := compileWorkflowDefinition(version.Definition)
		if len(issues) > 0 {
			continue
		}
		nodes := make([]biTeamMetricApprovalNodeOption, 0)
		for _, node := range definition.Nodes {
			if node.Type == "approval" {
				nodes = append(nodes, biTeamMetricApprovalNodeOption{NodeID: node.ID, NodeName: node.Name})
			}
		}
		if len(nodes) == 0 {
			continue
		}
		options = append(options, biTeamMetricWorkflowOption{
			TemplateID: template.ID, TemplateName: template.Name,
			PublishedVersion: version.Version, ApprovalNodes: nodes,
		})
	}
	return options, nil
}

func (s *Server) putBITeamMetricSettings(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	var body biTeamMetricSettingsRequest
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	if body.ExpectedVersion == nil || *body.ExpectedVersion < 0 {
		bad(c, "expected_version required")
		return
	}
	approvalNodeID := strings.TrimSpace(body.ApprovalNodeID)
	if body.RequirementWorkflowTemplateID == 0 || approvalNodeID == "" {
		bad(c, "requirement_workflow_template_id and approval_node_id required")
		return
	}
	template, version, definition, err := s.loadPublishedWorkflow(
		workspace.ID, body.RequirementWorkflowTemplateID, "requirement",
	)
	if err != nil {
		if !errors.Is(err, errPublishedWorkflowNotFound) && !errors.Is(err, errPublishedWorkflowInvalid) {
			dbFail(c)
			return
		}
		failCode(c, http.StatusUnprocessableEntity, apiErrorBITeamMetricWorkflowInvalid,
			"published requirement workflow template not found")
		return
	}
	node, found := workflowNodeByID(definition, approvalNodeID)
	if !found || node.Type != "approval" {
		failCode(c, http.StatusUnprocessableEntity, apiErrorBITeamMetricApprovalNodeInvalid,
			"approval_node_id must identify an approval node in the published requirement workflow")
		return
	}
	actorID := currentFactUserID(c)
	var saved workbenchgorm.BITeamMetricSettings
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if *body.ExpectedVersion == 0 {
			saved = workbenchgorm.BITeamMetricSettings{
				WorkspaceID: workspace.ID, Version: 1,
				RequirementWorkflowTemplateID: template.ID, ApprovalNodeID: node.ID,
				UpdatedByUserID: actorID,
			}
			created := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "workspace_id"}},
				DoNothing: true,
			}).Create(&saved)
			if created.Error != nil {
				return created.Error
			}
			if created.RowsAffected != 1 {
				return errBITeamMetricSettingsVersionConflict
			}
			return s.recordMutation(tx, c, 0, "bi_team_metric_settings", saved.ID,
				"updated", "团队指标统计口径", gin.H{
					"version": saved.Version, "requirement_workflow_template_id": template.ID,
					"requirement_workflow_published_version": version.Version,
					"approval_node_id":                       node.ID,
				})
		}
		var current workbenchgorm.BITeamMetricSettings
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("workspace_id = ?", workspace.ID).First(&current).Error
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		currentVersion := 0
		if findErr == nil {
			currentVersion = current.Version
		}
		if *body.ExpectedVersion != currentVersion {
			return errBITeamMetricSettingsVersionConflict
		}
		if current.ID != 0 {
			updates := map[string]any{
				"version":                          current.Version + 1,
				"requirement_workflow_template_id": template.ID,
				"approval_node_id":                 node.ID,
				"updated_by_user_id":               actorID,
			}
			result := tx.Model(&workbenchgorm.BITeamMetricSettings{}).
				Where("id = ? AND workspace_id = ? AND version = ?", current.ID, workspace.ID, current.Version).
				Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errBITeamMetricSettingsVersionConflict
			}
			saved = current
			saved.Version++
			saved.RequirementWorkflowTemplateID = template.ID
			saved.ApprovalNodeID = node.ID
			saved.UpdatedByUserID = actorID
		}
		return s.recordMutation(tx, c, 0, "bi_team_metric_settings", saved.ID,
			"updated", "团队指标统计口径", gin.H{
				"version": saved.Version, "requirement_workflow_template_id": template.ID,
				"requirement_workflow_published_version": version.Version,
				"approval_node_id":                       node.ID,
			})
	})
	if errors.Is(err, errBITeamMetricSettingsVersionConflict) {
		failCode(c, http.StatusConflict, apiErrorBITeamMetricSettingsConflict,
			"team metric settings have changed; reload and retry")
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	result, err := s.loadBITeamMetricSettings(workspace.ID)
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, result)
}
