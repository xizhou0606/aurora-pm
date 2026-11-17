package workbenchgorm

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	workbench "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
)

type AccessMenuRepository struct {
	db *gorm.DB
}

func NewAccessMenuRepository(db *gorm.DB) *AccessMenuRepository {
	return &AccessMenuRepository{db: db}
}

func (repository *AccessMenuRepository) ListMenus(ctx context.Context, includeDisabled bool) ([]workbench.Menu, error) {
	query := repository.db.WithContext(ctx).Model(&AccessMenu{}).Order("sort_order ASC, id ASC")
	if !includeDisabled {
		query = query.Where("enabled = ?", true)
	}
	var rows []AccessMenu
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return menusFromRecords(rows), nil
}

func (repository *AccessMenuRepository) ListEnabledMenus(ctx context.Context) ([]workbench.Menu, error) {
	return repository.ListMenus(ctx, false)
}

func (repository *AccessMenuRepository) FindMenu(ctx context.Context, id uint) (workbench.Menu, error) {
	var row AccessMenu
	if err := repository.db.WithContext(ctx).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return workbench.Menu{}, workbench.ErrMenuNotFound
		}
		return workbench.Menu{}, err
	}
	return menuFromRecord(row), nil
}

func (repository *AccessMenuRepository) EnsureSystemMenus(ctx context.Context, inputs []workbench.MenuInput) ([]workbench.Menu, error) {
	result := make([]workbench.Menu, 0, len(inputs))
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, input := range inputs {
			var row AccessMenu
			err := tx.Where("code = ?", input.Code).First(&row).Error
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				row = AccessMenu{
					Code: input.Code, ParentID: input.ParentID, Name: input.Name, MenuType: input.MenuType,
					Path: input.Path, Icon: input.Icon, HostKey: input.HostKey, SortOrder: input.SortOrder,
					Enabled: input.Enabled, SystemManaged: true,
				}
				if err := tx.Create(&row).Error; err != nil {
					if isMenuDuplicateError(err) {
						return workbench.ErrMenuCodeConflict
					}
					return err
				}
			case err != nil:
				return err
			case !row.SystemManaged:
				return workbench.ErrMenuCodeConflict
			default:
				updates := map[string]any{
					"parent_id": input.ParentID, "name": input.Name, "menu_type": input.MenuType,
					"path": input.Path, "icon": input.Icon, "host_key": input.HostKey,
					"sort_order": input.SortOrder, "enabled": input.Enabled, "system_managed": true,
				}
				if err := tx.Model(&row).Updates(updates).Error; err != nil {
					return err
				}
				if err := tx.First(&row, row.ID).Error; err != nil {
					return err
				}
			}
			result = append(result, menuFromRecord(row))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (repository *AccessMenuRepository) CreateMenu(ctx context.Context, input workbench.MenuInput) (workbench.Menu, error) {
	row := AccessMenu{
		Code: input.Code, ParentID: input.ParentID, Name: input.Name, MenuType: input.MenuType,
		Path: input.Path, Icon: input.Icon, HostKey: input.HostKey, SortOrder: input.SortOrder, Enabled: input.Enabled,
	}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		if isMenuDuplicateError(err) {
			return workbench.Menu{}, workbench.ErrMenuCodeConflict
		}
		return workbench.Menu{}, err
	}
	return menuFromRecord(row), nil
}

func (repository *AccessMenuRepository) UpdateMenu(ctx context.Context, id uint, input workbench.MenuUpdate) (workbench.Menu, error) {
	result := workbench.Menu{}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row AccessMenu
		if err := tx.First(&row, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return workbench.ErrMenuNotFound
			}
			return err
		}
		updates := map[string]any{}
		if input.ParentSet {
			updates["parent_id"] = input.ParentID
		}
		if input.Name != nil {
			updates["name"] = *input.Name
		}
		if input.MenuType != nil {
			updates["menu_type"] = *input.MenuType
		}
		if input.Path != nil {
			updates["path"] = *input.Path
		}
		if input.Icon != nil {
			updates["icon"] = *input.Icon
		}
		if input.HostKey != nil {
			updates["host_key"] = *input.HostKey
		}
		if input.SortOrder != nil {
			updates["sort_order"] = *input.SortOrder
		}
		if input.Enabled != nil {
			updates["enabled"] = *input.Enabled
		}
		if len(updates) > 0 {
			if err := tx.Model(&row).Updates(updates).Error; err != nil {
				return err
			}
		}
		if err := tx.First(&row, id).Error; err != nil {
			return err
		}
		result = menuFromRecord(row)
		return nil
	})
	return result, err
}

func (repository *AccessMenuRepository) DeleteMenu(ctx context.Context, id uint) error {
	result := repository.db.WithContext(ctx).Delete(&AccessMenu{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return workbench.ErrMenuNotFound
	}
	return nil
}

func (repository *AccessMenuRepository) HasChildren(ctx context.Context, id uint) (bool, error) {
	var count int64
	err := repository.db.WithContext(ctx).Model(&AccessMenu{}).Where("parent_id = ?", id).Count(&count).Error
	return count > 0, err
}

func (repository *AccessMenuRepository) EnabledMenuIDs(ctx context.Context, ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return []uint{}, nil
	}
	var result []uint
	err := repository.db.WithContext(ctx).Model(&AccessMenu{}).
		Where("id IN ? AND enabled = ?", ids, true).Order("id ASC").Pluck("id", &result).Error
	return result, err
}

func menuFromRecord(row AccessMenu) workbench.Menu {
	return workbench.Menu{
		ID: row.ID, Code: row.Code, ParentID: row.ParentID, Name: row.Name, MenuType: row.MenuType,
		Path: row.Path, Icon: row.Icon, HostKey: row.HostKey, SortOrder: row.SortOrder, Enabled: row.Enabled,
		SystemManaged: row.SystemManaged, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func menusFromRecords(rows []AccessMenu) []workbench.Menu {
	result := make([]workbench.Menu, 0, len(rows))
	for _, row := range rows {
		result = append(result, menuFromRecord(row))
	}
	return result
}

func isMenuDuplicateError(err error) bool {
	value := strings.ToLower(err.Error())
	return strings.Contains(value, "unique") || strings.Contains(value, "duplicate")
}

var _ workbench.MenuRepository = (*AccessMenuRepository)(nil)
