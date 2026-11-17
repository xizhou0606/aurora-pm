package qualitygorm

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
)

type SceneOptionRepository struct {
	db *gorm.DB
}

func NewSceneOptionRepository(db *gorm.DB) *SceneOptionRepository {
	return &SceneOptionRepository{db: db}
}

func (repository *SceneOptionRepository) ListCustom(
	workspaceID uint,
	kind quality.SceneOptionKind,
) ([]quality.SceneOptionRecord, error) {
	var rows []QualitySceneOption
	if err := repository.db.Where("workspace_id = ? AND kind = ?", workspaceID, kind).
		Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]quality.SceneOptionRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, quality.SceneOptionRecord{
			ID: row.ID, Name: row.Name, NormalizedName: row.NormalizedName,
		})
	}
	return result, nil
}

func (repository *SceneOptionRepository) ListLegacyNames(
	workspaceID uint,
	kind quality.SceneOptionKind,
) ([]string, error) {
	column := kind.LegacyColumn()
	if column == "" {
		return nil, quality.ErrSceneOptionKindInvalid
	}
	var names []string
	err := repository.db.Model(&QualityBug{}).
		Distinct(column).
		Where("workspace_id = ? AND TRIM("+column+") <> ''", workspaceID).
		Order(column+" ASC").
		Pluck(column, &names).Error
	return names, err
}

func (repository *SceneOptionRepository) UsageCounts(
	workspaceID uint,
	kind quality.SceneOptionKind,
) (map[string]int64, error) {
	column := kind.LegacyColumn()
	if column == "" {
		return nil, quality.ErrSceneOptionKindInvalid
	}
	type usageRow struct {
		NormalizedName string
		UsageCount     int64
	}
	var rows []usageRow
	err := repository.db.Model(&QualityBug{}).
		Select("LOWER(TRIM("+column+")) AS normalized_name, COUNT(*) AS usage_count").
		Where("workspace_id = ? AND TRIM("+column+") <> ''", workspaceID).
		Group("LOWER(TRIM(" + column + "))").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, row := range rows {
		result[row.NormalizedName] = row.UsageCount
	}
	return result, nil
}

func (repository *SceneOptionRepository) NameExists(
	workspaceID uint,
	kind quality.SceneOptionKind,
	normalizedName string,
) (bool, error) {
	var customCount int64
	if err := repository.db.Model(&QualitySceneOption{}).
		Where("workspace_id = ? AND kind = ? AND normalized_name = ?", workspaceID, kind, normalizedName).
		Count(&customCount).Error; err != nil {
		return false, err
	}
	if customCount > 0 {
		return true, nil
	}
	column := kind.LegacyColumn()
	if column == "" {
		return false, quality.ErrSceneOptionKindInvalid
	}
	var legacyCount int64
	if err := repository.db.Model(&QualityBug{}).
		Where("workspace_id = ? AND LOWER(TRIM("+column+")) = ?", workspaceID, normalizedName).
		Count(&legacyCount).Error; err != nil {
		return false, err
	}
	return legacyCount > 0, nil
}

func (repository *SceneOptionRepository) Create(
	workspaceID, createdByUserID uint,
	kind quality.SceneOptionKind,
	name, normalizedName string,
) (quality.SceneOptionRecord, error) {
	row := QualitySceneOption{
		WorkspaceID: workspaceID, Kind: string(kind), Name: name, NormalizedName: normalizedName,
		CreatedByUserID: createdByUserID,
	}
	if err := repository.db.Create(&row).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") ||
			strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return quality.SceneOptionRecord{}, quality.ErrSceneOptionNameDuplicate
		}
		return quality.SceneOptionRecord{}, err
	}
	return quality.SceneOptionRecord{ID: row.ID, Name: row.Name, NormalizedName: row.NormalizedName}, nil
}

func (repository *SceneOptionRepository) FindCustom(
	workspaceID, id uint,
	kind quality.SceneOptionKind,
) (quality.SceneOptionRecord, error) {
	var row QualitySceneOption
	if err := repository.db.Where("workspace_id = ? AND id = ? AND kind = ?", workspaceID, id, kind).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return quality.SceneOptionRecord{}, quality.ErrSceneOptionNotFound
		}
		return quality.SceneOptionRecord{}, err
	}
	return quality.SceneOptionRecord{ID: row.ID, Name: row.Name, NormalizedName: row.NormalizedName}, nil
}

func (repository *SceneOptionRepository) DeleteCustom(
	workspaceID, id uint,
	kind quality.SceneOptionKind,
) error {
	result := repository.db.Where("workspace_id = ? AND id = ? AND kind = ?", workspaceID, id, kind).
		Delete(&QualitySceneOption{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return quality.ErrSceneOptionNotFound
	}
	return nil
}
