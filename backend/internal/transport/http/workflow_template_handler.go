package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

type workflowTemplateBody struct {
	Name        string             `json:"name"`
	SubjectType string             `json:"subject_type"`
	Description string             `json:"description"`
	Definition  workflowDefinition `json:"definition"`
}

func (s *Server) listWorkflowTemplates(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	query := s.db.Where("workspace_id = ?", workspace.ID)
	if subjectType := strings.TrimSpace(c.Query("subject_type")); subjectType != "" {
		query = query.Where("subject_type = ?", subjectType)
	}
	var rows []workflowgorm.WorkflowTemplate
	if query.Order("updated_at DESC").Find(&rows).Error != nil {
		dbFail(c)
		return
	}
	data := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		data = append(data, workflowTemplateDTO(row, false))
	}
	if strings.TrimSpace(c.Query("include_nodes")) == "1" {
		s.attachPublishedWorkflowNodes(workspace.ID, rows, data)
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}

func (s *Server) createWorkflowTemplate(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	var body workflowTemplateBody
	if c.ShouldBindJSON(&body) != nil || strings.TrimSpace(body.Name) == "" || !validWorkflowSubject(body.SubjectType) {
		bad(c, "name and valid subject_type required")
		return
	}
	userID := currentFactUserID(c)
	stripWorkflowTemplateAssignees(&body.Definition)
	stripRetiredWorkflowNodeFlags(&body.Definition)
	raw, _ := json.Marshal(body.Definition)
	definition, issues := compileWorkflowDefinition(string(raw))
	issues = append(issues, validateWorkflowActionEdges(definition)...)
	issues = append(issues, validateWorkflowCapabilities(definition, body.SubjectType)...)
	issues = append(issues, s.validateWorkflowRequirementsGuardBindings(workspace.ID, definition)...)
	if len(issues) > 0 {
		fail(c, http.StatusUnprocessableEntity, strings.Join(issues, "；"))
		return
	}
	row := workflowgorm.WorkflowTemplate{
		WorkspaceID: workspace.ID, Name: strings.TrimSpace(body.Name), SubjectType: body.SubjectType,
		Description: strings.TrimSpace(body.Description), DraftDefinition: string(raw),
		CreatedByUserID: userID, UpdatedByUserID: userID,
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return s.recordMutation(tx, c, 0, "workflow_template", row.ID, "created", row.Name, gin.H{"subject_type": row.SubjectType})
	}); err != nil {
		dbFail(c)
		return
	}
	created(c, workflowTemplateDTO(row, true))
}

func (s *Server) getWorkflowTemplate(c *gin.Context) {
	row, okRow := s.workflowTemplateByID(c)
	if !okRow {
		return
	}
	ok(c, workflowTemplateDTO(row, true))
}

func (s *Server) patchWorkflowTemplate(c *gin.Context) {
	row, okRow := s.workflowTemplateByID(c)
	if !okRow {
		return
	}
	var body workflowTemplateBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	updates := map[string]any{"updated_by_user_id": currentFactUserID(c)}
	if strings.TrimSpace(body.Name) != "" {
		updates["name"] = strings.TrimSpace(body.Name)
	}
	if body.Description != "" {
		updates["description"] = strings.TrimSpace(body.Description)
	}
	if len(body.Definition.Nodes) > 0 {
		stripWorkflowTemplateAssignees(&body.Definition)
		stripRetiredWorkflowNodeFlags(&body.Definition)
		raw, _ := json.Marshal(body.Definition)
		updates["draft_definition"] = string(raw)
	} else {
		var definition workflowDefinition
		if json.Unmarshal([]byte(row.DraftDefinition), &definition) == nil {
			stripWorkflowTemplateAssignees(&definition)
			stripRetiredWorkflowNodeFlags(&definition)
			raw, _ := json.Marshal(definition)
			updates["draft_definition"] = string(raw)
		}
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&row).Updates(updates).Error; err != nil {
			return err
		}
		return s.recordMutation(tx, c, 0, "workflow_template", row.ID, "updated", row.Name, gin.H{"fields": allowedFieldKeys(updates)})
	}); err != nil {
		dbFail(c)
		return
	}
	_ = s.db.First(&row, row.ID).Error
	ok(c, workflowTemplateDTO(row, true))
}

func (s *Server) validateWorkflowTemplate(c *gin.Context) {
	row, okRow := s.workflowTemplateByID(c)
	if !okRow {
		return
	}
	definition, issues := compileWorkflowDefinition(row.DraftDefinition)
	stripWorkflowTemplateAssignees(&definition)
	issues = append(issues, validateWorkflowActionEdges(definition)...)
	issues = append(issues, validateWorkflowCapabilities(definition, row.SubjectType)...)
	issues = append(issues, s.validateWorkflowRequirementsGuardBindings(row.WorkspaceID, definition)...)
	ok(c, gin.H{"valid": len(issues) == 0, "issues": issues})
}

func (s *Server) validateWorkflowDefinition(c *gin.Context) {
	row, okRow := s.workflowTemplateByID(c)
	if !okRow {
		return
	}
	var body struct {
		Definition workflowDefinition `json:"definition"`
	}
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	stripWorkflowTemplateAssignees(&body.Definition)
	stripRetiredWorkflowNodeFlags(&body.Definition)
	raw, _ := json.Marshal(body.Definition)
	definition, issues := compileWorkflowDefinition(string(raw))
	issues = append(issues, validateWorkflowActionEdges(definition)...)
	issues = append(issues, validateWorkflowCapabilities(definition, row.SubjectType)...)
	issues = append(issues, s.validateWorkflowRequirementsGuardBindings(row.WorkspaceID, definition)...)
	ok(c, gin.H{"valid": len(issues) == 0, "issues": uniqueStrings(issues)})
}

func (s *Server) publishWorkflowTemplate(c *gin.Context) {
	row, okRow := s.workflowTemplateByID(c)
	if !okRow {
		return
	}
	definition, issues := compileWorkflowDefinition(row.DraftDefinition)
	stripWorkflowTemplateAssignees(&definition)
	issues = append(issues, validateWorkflowActionEdges(definition)...)
	issues = append(issues, validateWorkflowCapabilities(definition, row.SubjectType)...)
	issues = append(issues, s.validateWorkflowRequirementsGuardBindings(row.WorkspaceID, definition)...)
	if len(issues) > 0 {
		fail(c, http.StatusUnprocessableEntity, strings.Join(issues, "；"))
		return
	}
	normalizedDefinition, _ := json.Marshal(definition)
	hash := sha256.Sum256(normalizedDefinition)
	version := row.PublishedVersion + 1
	versionRow := workflowgorm.WorkflowTemplateVersion{
		WorkspaceID: row.WorkspaceID, TemplateID: row.ID, Version: version,
		Definition: string(normalizedDefinition), DefinitionHash: hex.EncodeToString(hash[:]),
		PublishedByUser: currentFactUserID(c), PublishedAt: time.Now().UTC(),
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&versionRow).Error; err != nil {
			return err
		}
		if err := tx.Model(&row).Updates(map[string]any{"draft_definition": string(normalizedDefinition), "published_version": version}).Error; err != nil {
			return err
		}
		return s.recordMutation(tx, c, 0, "workflow_template", row.ID, "published", row.Name, gin.H{"version": version, "definition_hash": versionRow.DefinitionHash})
	}); err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{"template_id": row.ID, "version": version, "definition_hash": versionRow.DefinitionHash})
}

func (s *Server) workflowTemplateByID(c *gin.Context) (workflowgorm.WorkflowTemplate, bool) {
	id, okID := paramID(c, "id")
	if !okID {
		return workflowgorm.WorkflowTemplate{}, false
	}
	workspace, _ := currentWorkspace(c)
	var row workflowgorm.WorkflowTemplate
	if s.db.Where("id = ? AND workspace_id = ?", id, workspace.ID).First(&row).Error != nil {
		fail(c, http.StatusNotFound, "workflow template not found")
		return workflowgorm.WorkflowTemplate{}, false
	}
	return row, true
}

func workflowTemplateDTO(row workflowgorm.WorkflowTemplate, includeDefinition bool) gin.H {
	dto := gin.H{
		"id": row.ID, "workspace_id": row.WorkspaceID, "name": row.Name,
		"subject_type": row.SubjectType, "description": row.Description,
		"published_version": row.PublishedVersion, "created_by_user_id": row.CreatedByUserID,
		"updated_by_user_id": row.UpdatedByUserID, "created_at": row.CreatedAt, "updated_at": row.UpdatedAt,
	}
	if includeDefinition {
		var definition workflowDefinition
		_ = json.Unmarshal([]byte(row.DraftDefinition), &definition)
		stripWorkflowTemplateAssignees(&definition)
		dto["definition"] = definition
	}
	return dto
}

func validWorkflowSubject(value string) bool { return value == "requirement" || value == "plan_unit" }

var workflowFilterSkipNodeTypes = map[string]bool{
	"start": true, "end": true, "parallel_split": true, "parallel_join": true,
}

func workflowTemplateFilterNodes(definition workflowDefinition) []gin.H {
	nodes := make([]gin.H, 0, len(definition.Nodes))
	for _, node := range definition.Nodes {
		id := strings.TrimSpace(node.ID)
		name := strings.TrimSpace(node.Name)
		if id == "" || name == "" || workflowFilterSkipNodeTypes[node.Type] {
			continue
		}
		nodes = append(nodes, gin.H{"id": id, "type": node.Type, "name": name})
	}
	return nodes
}

func (s *Server) attachPublishedWorkflowNodes(
	workspaceID uint,
	templates []workflowgorm.WorkflowTemplate,
	data []gin.H,
) {
	ids := make([]uint, 0, len(templates))
	for _, row := range templates {
		if row.PublishedVersion > 0 {
			ids = append(ids, row.ID)
		}
	}
	versions := map[string]workflowgorm.WorkflowTemplateVersion{}
	if len(ids) > 0 {
		var rows []workflowgorm.WorkflowTemplateVersion
		if s.db.Where("workspace_id = ? AND template_id IN ?", workspaceID, ids).Find(&rows).Error == nil {
			for _, row := range rows {
				versions[strconv.FormatUint(uint64(row.TemplateID), 10)+":"+strconv.FormatUint(uint64(row.Version), 10)] = row
			}
		}
	}
	for index, row := range templates {
		data[index]["nodes"] = []gin.H{}
		if row.PublishedVersion == 0 {
			continue
		}
		raw := row.DraftDefinition
		// published_version_id is the immutable binding key used by
		// requirements_guard template-node mappings.
		if version, ok := versions[strconv.FormatUint(uint64(row.ID), 10)+":"+strconv.FormatUint(uint64(row.PublishedVersion), 10)]; ok {
			raw = version.Definition
			data[index]["published_version_id"] = version.ID
		}
		var definition workflowDefinition
		if json.Unmarshal([]byte(raw), &definition) != nil {
			continue
		}
		data[index]["nodes"] = workflowTemplateFilterNodes(definition)
	}
}
