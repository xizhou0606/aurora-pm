package httpapi

import (
	"strings"
	"time"

	"gorm.io/gorm"

	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/cover"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	requirementhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/http"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

type requirementPoolWorkflowOverview struct {
	InstanceID       uint                                  `json:"instance_id"`
	Status           string                                `json:"status"`
	CurrentNodeNames []string                              `json:"current_node_names"`
	CurrentNodes     []requirementPoolWorkflowNodeOverview `json:"current_nodes"`
	CompletedSteps   int                                   `json:"completed_steps"`
	TotalSteps       int                                   `json:"total_steps"`
}

type requirementPoolWorkflowNodeOverview struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	PlannedEndAt *time.Time `json:"planned_end_at,omitempty"`
	Overdue      bool       `json:"overdue"`
}

type requirementPoolDeliverableOverview struct {
	RequirementDocument    bool                               `json:"requirement_document"`
	RequirementDocumentURL string                             `json:"requirement_document_url"`
	Design                 bool                               `json:"design"`
	DesignURL              string                             `json:"design_url"`
	DesignURLs             []string                           `json:"design_urls,omitempty"`
	DesignArtifacts        []*requirementPoolDesignArtifact   `json:"design_artifacts,omitempty"`
	TestCases              bool                               `json:"test_cases"`
	TestCaseArtifact       *requirementPoolTestCaseArtifact   `json:"test_case_artifact"`
	TestCaseArtifacts      []*requirementPoolTestCaseArtifact `json:"test_case_artifacts,omitempty"`
	RequirementCover       *requirementPoolCoverOverview      `json:"requirement_cover,omitempty"`
}

type requirementPoolCoverOverview struct {
	Status     string    `json:"status"`
	Generation uint      `json:"generation"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type requirementPoolDesignArtifact struct {
	ID          uint      `json:"id,omitempty"`
	Name        string    `json:"name"`
	ContentType string    `json:"content_type,omitempty"`
	Size        int64     `json:"size,omitempty"`
	URL         string    `json:"url,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}

type requirementPoolTestCaseArtifact struct {
	ID            uint      `json:"id"`
	RequirementID uint      `json:"requirement_id"`
	InstanceID    uint      `json:"instance_id"`
	NodeID        string    `json:"node_id"`
	Kind          string    `json:"kind"`
	Name          string    `json:"name"`
	ContentType   string    `json:"content_type"`
	Size          int64     `json:"size"`
	URL           string    `json:"url,omitempty"`
	AIRunID       *uint     `json:"ai_run_id"`
	CreatedAt     time.Time `json:"created_at"`
}

type requirementPoolOverview struct {
	Workflow     *requirementPoolWorkflowOverview
	Deliverables requirementPoolDeliverableOverview
}

func (s *Server) loadRequirementPoolOverviews(
	workspaceID uint,
	requirements []requirementgorm.Requirement,
	businessDate time.Time,
) (map[uint]requirementPoolOverview, error) {
	result := make(map[uint]requirementPoolOverview, len(requirements))
	instanceIDs := make([]uint, 0, len(requirements))
	for _, requirement := range requirements {
		requirementDocumentURL := strings.TrimSpace(requirement.ProductDocURL)
		designURL := strings.TrimSpace(requirement.DesignURL)
		result[requirement.ID] = requirementPoolOverview{Deliverables: requirementPoolDeliverableOverview{
			RequirementDocument:    requirementDocumentURL != "",
			RequirementDocumentURL: requirementDocumentURL,
			Design:                 designURL != "",
			DesignURL:              designURL,
		}}
		if requirement.WorkflowInstanceID != nil && *requirement.WorkflowInstanceID > 0 {
			instanceIDs = append(instanceIDs, *requirement.WorkflowInstanceID)
		}
	}
	if err := s.loadRequirementPoolCovers(workspaceID, requirements, result); err != nil {
		return nil, err
	}
	if err := s.loadRequirementPoolMaterials(workspaceID, requirements, result); err != nil {
		return nil, err
	}
	if len(instanceIDs) == 0 {
		return result, nil
	}

	var instances []workflowgorm.WorkflowInstance
	if err := s.db.Where(
		"workspace_id = ? AND subject_type = ? AND id IN ?",
		workspaceID, "requirement", instanceIDs,
	).Find(&instances).Error; err != nil {
		return nil, err
	}
	versions, err := loadRequirementPoolWorkflowVersions(s.db, workspaceID, instances)
	if err != nil {
		return nil, err
	}
	runs, err := loadRequirementPoolWorkflowRuns(s.db, workspaceID, instanceIDs)
	if err != nil {
		return nil, err
	}
	artifacts, err := loadRequirementPoolWorkflowArtifacts(s.db, workspaceID, instanceIDs)
	if err != nil {
		return nil, err
	}
	scheduleScopes, err := loadRequirementPoolScheduleScopes(s.db, workspaceID, requirements)
	if err != nil {
		return nil, err
	}

	instanceByID := make(map[uint]workflowgorm.WorkflowInstance, len(instances))
	for _, instance := range instances {
		instanceByID[instance.ID] = instance
	}
	for _, requirement := range requirements {
		if requirement.WorkflowInstanceID == nil {
			continue
		}
		instance, found := instanceByID[*requirement.WorkflowInstanceID]
		if !found || instance.SubjectID != requirement.ID {
			continue
		}
		version, found := versions[instance.TemplateVersionID]
		if !found {
			continue
		}
		definition, issues := compileWorkflowDefinition(version.Definition)
		if len(issues) > 0 {
			continue
		}
		selected, conditionEdges, selectionErr := workflowSelectedTopologyForInstance(
			s.db, definition, instance, runs[instance.ID],
		)
		if selectionErr != nil {
			continue
		}
		displayNodeIDs, displayEdgeIDs := workflowDisplayTopology(
			definition, runs[instance.ID], conditionEdges, currentWorkflowNodes(instance.CurrentNodeIDs),
		)
		displayNodes := make(map[string]bool, len(displayNodeIDs))
		for _, nodeID := range displayNodeIDs {
			if selected[nodeID] {
				displayNodes[nodeID] = true
			}
		}
		schedule := unavailableWorkflowSubjectSchedule()
		plans, planErr := decodeWorkflowNodePlans(instance)
		scope, scopeFound := scheduleScopes[requirement.ID]
		if planErr == nil && scopeFound {
			schedule = workflowScheduleForParts(definition, displayNodes, displayEdgeIDs, plans, scope)
		}
		overview := result[requirement.ID]
		overview.Workflow = buildRequirementPoolWorkflowOverview(
			instance, definition, runs[instance.ID], selected, schedule, businessDate,
		)
		artifactList := selectRequirementPoolTestCaseArtifacts(
			definition, selected, artifacts[instance.ID],
		)
		if !overview.Deliverables.TestCases && len(artifactList) > 0 {
			overview.Deliverables.TestCases = true
			overview.Deliverables.TestCaseArtifact = requirementPoolTestCaseArtifactDTO(&artifactList[len(artifactList)-1])
			overview.Deliverables.TestCaseArtifacts = requirementPoolTestCaseArtifactDTOs(artifactList)
		}
		designURLs := selectRequirementPoolDesignURLs(
			definition, selected, runs[instance.ID], overview.Deliverables.DesignURL,
		)
		if merged := mergeRequirementPoolDesignURLs(overview.Deliverables.DesignURLs, designURLs); len(merged) > 0 {
			overview.Deliverables.Design = true
			overview.Deliverables.DesignURL = merged[len(merged)-1]
			overview.Deliverables.DesignURLs = merged
			overview.Deliverables.DesignArtifacts = mergeRequirementPoolDesignArtifacts(
				overview.Deliverables.DesignArtifacts, designURLs,
			)
		}
		result[requirement.ID] = overview
	}
	return result, nil
}

func (s *Server) loadRequirementPoolCovers(
	workspaceID uint,
	requirements []requirementgorm.Requirement,
	result map[uint]requirementPoolOverview,
) error {
	ids := make([]uint, 0, len(requirements))
	for _, requirement := range requirements {
		ids = append(ids, requirement.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	var covers []requirementgorm.RequirementCover
	if err := s.db.Select("workspace_id", "requirement_id", "generation", "status", "updated_at").
		Where("workspace_id = ? AND requirement_id IN ? AND status IN ?", workspaceID, ids,
			[]string{requirementcover.StatusPending, requirementcover.StatusProcessing, requirementcover.StatusReady, requirementcover.StatusFailed}).
		Find(&covers).Error; err != nil {
		return err
	}
	for _, cover := range covers {
		overview, ok := result[cover.RequirementID]
		if !ok {
			continue
		}
		status := cover.Status
		if status == requirementcover.StatusProcessing {
			status = requirementcover.StatusPending
		}
		overview.Deliverables.RequirementCover = &requirementPoolCoverOverview{
			Status: status, Generation: cover.Generation, UpdatedAt: cover.UpdatedAt,
		}
		result[cover.RequirementID] = overview
	}
	return nil
}

func (s *Server) loadRequirementPoolMaterials(
	workspaceID uint,
	requirements []requirementgorm.Requirement,
	result map[uint]requirementPoolOverview,
) error {
	ids := make([]uint, 0, len(requirements))
	for _, row := range requirements {
		ids = append(ids, row.ID)
	}
	if len(ids) == 0 || s.materials == nil {
		return nil
	}
	testCases, err := s.materials.ListForRequirements(workspaceID, ids, requirement.MaterialKindTestCase)
	if err != nil {
		return err
	}
	designs, err := s.materials.ListForRequirements(workspaceID, ids, requirement.MaterialKindDesign)
	if err != nil {
		return err
	}
	casesByRequirement := map[uint][]requirement.Material{}
	for _, row := range testCases {
		casesByRequirement[row.RequirementID] = append(casesByRequirement[row.RequirementID], row)
	}
	designsByRequirement := map[uint][]requirement.Material{}
	for _, row := range designs {
		designsByRequirement[row.RequirementID] = append(designsByRequirement[row.RequirementID], row)
	}
	for _, row := range requirements {
		overview := result[row.ID]
		if materials := casesByRequirement[row.ID]; len(materials) > 0 {
			items := make([]*requirementPoolTestCaseArtifact, 0, len(materials))
			for _, material := range materials {
				items = append(items, requirementPoolMaterialArtifactDTO(material))
			}
			overview.Deliverables.TestCases = true
			overview.Deliverables.TestCaseArtifact = items[len(items)-1]
			overview.Deliverables.TestCaseArtifacts = items
		}
		if materials := designsByRequirement[row.ID]; len(materials) > 0 {
			urls := overview.Deliverables.DesignURLs
			if urls == nil {
				urls = make([]string, 0, len(materials)+1)
			}
			if overview.Deliverables.DesignURL != "" {
				urls = append(urls, overview.Deliverables.DesignURL)
			}
			artifacts := overview.Deliverables.DesignArtifacts
			if artifacts == nil {
				artifacts = make([]*requirementPoolDesignArtifact, 0, len(materials))
			}
			seen := map[string]bool{}
			for _, url := range urls {
				seen[url] = true
			}
			for _, material := range materials {
				url := requirementhttp.MaterialOriginURL(material)
				if url == "" {
					url = requirementhttp.MaterialOpenURL(material)
				}
				if url == "" {
					continue
				}
				artifacts = append(artifacts, requirementPoolDesignArtifactDTO(material, url))
				if seen[url] {
					continue
				}
				seen[url] = true
				urls = append(urls, url)
			}
			if len(urls) > 0 {
				overview.Deliverables.Design = true
				overview.Deliverables.DesignURL = urls[len(urls)-1]
				overview.Deliverables.DesignURLs = urls
				overview.Deliverables.DesignArtifacts = artifacts
			}
		}
		result[row.ID] = overview
	}
	return nil
}

func requirementPoolDesignArtifactDTO(row requirement.Material, url string) *requirementPoolDesignArtifact {
	name := strings.TrimSpace(row.Name)
	if name == "" {
		name = url
	}
	return &requirementPoolDesignArtifact{
		ID: row.ID, Name: name, ContentType: row.ContentType, Size: row.Size, URL: url, CreatedAt: row.CreatedAt,
	}
}

func requirementPoolMaterialArtifactDTO(row requirement.Material) *requirementPoolTestCaseArtifact {
	kind := "uploaded"
	if row.Source == requirement.MaterialSourceAIGenerated {
		kind = "ai_generated"
	}
	url := requirementhttp.MaterialOpenURL(row)
	if requirementhttp.IsMaterialLink(row) && strings.TrimSpace(row.URL) != "" {
		url = row.URL
	}
	return &requirementPoolTestCaseArtifact{
		ID: row.ID, RequirementID: row.RequirementID, Kind: kind, Name: row.Name,
		ContentType: row.ContentType, Size: row.Size, URL: url,
		AIRunID: row.AIRunID, CreatedAt: row.CreatedAt,
	}
}

func loadRequirementPoolWorkflowVersions(
	db *gorm.DB,
	workspaceID uint,
	instances []workflowgorm.WorkflowInstance,
) (map[uint]workflowgorm.WorkflowTemplateVersion, error) {
	ids := make([]uint, 0, len(instances))
	seen := map[uint]bool{}
	for _, instance := range instances {
		if !seen[instance.TemplateVersionID] {
			seen[instance.TemplateVersionID] = true
			ids = append(ids, instance.TemplateVersionID)
		}
	}
	var rows []workflowgorm.WorkflowTemplateVersion
	if len(ids) > 0 {
		if err := db.Where("workspace_id = ? AND id IN ?", workspaceID, ids).Find(&rows).Error; err != nil {
			return nil, err
		}
	}
	result := make(map[uint]workflowgorm.WorkflowTemplateVersion, len(rows))
	for _, row := range rows {
		result[row.ID] = row
	}
	return result, nil
}

func loadRequirementPoolWorkflowRuns(
	db *gorm.DB,
	workspaceID uint,
	instanceIDs []uint,
) (map[uint][]workflowgorm.WorkflowNodeRun, error) {
	var rows []workflowgorm.WorkflowNodeRun
	if err := db.Where("workspace_id = ? AND instance_id IN ?", workspaceID, instanceIDs).
		Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := map[uint][]workflowgorm.WorkflowNodeRun{}
	for _, row := range rows {
		result[row.InstanceID] = append(result[row.InstanceID], row)
	}
	return result, nil
}

func loadRequirementPoolWorkflowArtifacts(
	db *gorm.DB,
	workspaceID uint,
	instanceIDs []uint,
) (map[uint][]workflowgorm.WorkflowNodeArtifact, error) {
	var rows []workflowgorm.WorkflowNodeArtifact
	if err := db.Select(
		"id", "instance_id", "node_id", "kind", "name", "content_type", "size", "ai_run_id", "created_at",
	).
		Where("workspace_id = ? AND instance_id IN ?", workspaceID, instanceIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	result := map[uint][]workflowgorm.WorkflowNodeArtifact{}
	for _, row := range rows {
		result[row.InstanceID] = append(result[row.InstanceID], row)
	}
	return result, nil
}
