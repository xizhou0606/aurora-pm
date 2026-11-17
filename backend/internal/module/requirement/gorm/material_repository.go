package requirementgorm

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
)

type MaterialRepository struct {
	db *gorm.DB
}

func NewMaterialRepository(db *gorm.DB) *MaterialRepository {
	return &MaterialRepository{db: db}
}

func (repository *MaterialRepository) Create(input requirement.MaterialInput) (requirement.Material, error) {
	row := RequirementMaterial{
		WorkspaceID: input.WorkspaceID, RequirementID: input.RequirementID, Kind: input.Kind,
		Source: input.Source, Name: input.Name, ContentType: input.ContentType, Size: input.Size,
		ObjectKey: input.ObjectKey, URL: input.URL, AIRunID: input.AIRunID, UploaderUserID: input.UploaderUserID,
	}
	if err := repository.db.Create(&row).Error; err != nil {
		if existing, lookupErr := repository.GetByObjectKey(input.WorkspaceID, input.ObjectKey); lookupErr == nil {
			return existing, nil
		}
		return requirement.Material{}, err
	}
	return materialFromRow(row), nil
}

func (repository *MaterialRepository) GetByObjectKey(workspaceID uint, objectKey string) (requirement.Material, error) {
	var row RequirementMaterial
	err := repository.db.Where("workspace_id = ? AND object_key = ?", workspaceID, strings.TrimSpace(objectKey)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return requirement.Material{}, requirement.ErrMaterialNotFound
	}
	return materialFromRow(row), err
}

func (repository *MaterialRepository) List(workspaceID, requirementID uint, kind string) ([]requirement.Material, error) {
	query := repository.db.Where("workspace_id = ? AND requirement_id = ?", workspaceID, requirementID)
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	var rows []RequirementMaterial
	if err := query.Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]requirement.Material, 0, len(rows))
	for _, row := range rows {
		result = append(result, materialFromRow(row))
	}
	return result, nil
}

func (repository *MaterialRepository) Get(workspaceID, requirementID, materialID uint) (requirement.Material, error) {
	var row RequirementMaterial
	err := repository.db.Where(
		"id = ? AND workspace_id = ? AND requirement_id = ?", materialID, workspaceID, requirementID,
	).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return requirement.Material{}, requirement.ErrMaterialNotFound
	}
	return materialFromRow(row), err
}

func (repository *MaterialRepository) Delete(workspaceID, requirementID, materialID uint) error {
	result := repository.db.Where(
		"id = ? AND workspace_id = ? AND requirement_id = ?", materialID, workspaceID, requirementID,
	).Delete(&RequirementMaterial{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return requirement.ErrMaterialNotFound
	}
	return nil
}

func (repository *MaterialRepository) ListForRequirements(workspaceID uint, requirementIDs []uint, kind string) ([]requirement.Material, error) {
	if len(requirementIDs) == 0 {
		return nil, nil
	}
	query := repository.db.Where("workspace_id = ? AND requirement_id IN ?", workspaceID, requirementIDs)
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	var rows []RequirementMaterial
	if err := query.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]requirement.Material, 0, len(rows))
	for _, row := range rows {
		result = append(result, materialFromRow(row))
	}
	return result, nil
}

func (repository *MaterialRepository) Count(workspaceID, requirementID uint, kind string) (int64, error) {
	var count int64
	err := repository.db.Model(&RequirementMaterial{}).
		Where("workspace_id = ? AND requirement_id = ? AND kind = ?", workspaceID, requirementID, kind).
		Count(&count).Error
	return count, err
}

func (repository *MaterialRepository) ListTestCases(workspaceID uint, filter requirement.LibraryFilter) ([]requirement.LibraryMaterial, error) {
	query := repository.db.Table("requirement_materials AS materials").
		Select(
			"materials.*, requirements.title AS requirement_title, requirements.project_new_id AS project_set_id, "+
				"requirements.version_task_new_id AS plan_unit_id, project_sets.name AS project_name, plan_units.title AS plan_unit_title",
		).
		Joins("JOIN requirements ON requirements.id = materials.requirement_id AND requirements.workspace_id = materials.workspace_id").
		Joins("LEFT JOIN project_sets ON project_sets.id = requirements.project_new_id AND project_sets.workspace_id = materials.workspace_id").
		Joins("LEFT JOIN plan_units ON plan_units.id = requirements.version_task_new_id AND plan_units.workspace_id = materials.workspace_id").
		Where("materials.workspace_id = ? AND materials.kind = ?", workspaceID, requirement.MaterialKindTestCase)
	if filter.ProjectID > 0 {
		query = query.Where("requirements.project_new_id = ?", filter.ProjectID)
	}
	if filter.PlanUnitID > 0 {
		query = query.Where("requirements.version_task_new_id = ?", filter.PlanUnitID)
	}
	if needle := strings.TrimSpace(filter.Query); needle != "" {
		like := "%" + needle + "%"
		query = query.Where(
			"materials.name LIKE ? OR requirements.title LIKE ? OR project_sets.name LIKE ? OR plan_units.title LIKE ?",
			like, like, like, like,
		)
	}
	if filter.AccessScope.SQL != "" {
		query = query.Where(filter.AccessScope.SQL, filter.AccessScope.Args...)
	}
	var rows []libraryMaterialRow
	if err := query.Order("materials.id DESC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]requirement.LibraryMaterial, 0, len(rows))
	for _, row := range rows {
		result = append(result, requirement.LibraryMaterial{
			Material:         materialFromRow(row.RequirementMaterial),
			RequirementTitle: row.RequirementTitle,
			ProjectSetID:     row.ProjectSetID,
			PlanUnitID:       row.PlanUnitID,
			ProjectName:      row.ProjectName,
			PlanUnitTitle:    row.PlanUnitTitle,
		})
	}
	return result, nil
}

type libraryMaterialRow struct {
	RequirementMaterial
	RequirementTitle string
	ProjectSetID     uint
	PlanUnitID       uint
	ProjectName      string
	PlanUnitTitle    string
}

func materialFromRow(row RequirementMaterial) requirement.Material {
	return requirement.Material{
		ID: row.ID, WorkspaceID: row.WorkspaceID, RequirementID: row.RequirementID,
		Kind: row.Kind, Source: row.Source, Name: row.Name, ContentType: row.ContentType,
		Size: row.Size, ObjectKey: row.ObjectKey, URL: row.URL, AIRunID: row.AIRunID,
		UploaderUserID: row.UploaderUserID, CreatedAt: row.CreatedAt,
	}
}
