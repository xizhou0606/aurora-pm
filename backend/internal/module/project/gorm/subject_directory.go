package projectgorm

import (
	"context"
	"strings"

	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

type ProjectSetSubjectRepository struct{ db *gorm.DB }

func NewProjectSetSubjectRepository(db *gorm.DB) *ProjectSetSubjectRepository {
	return &ProjectSetSubjectRepository{db: db}
}

func (repository *ProjectSetSubjectRepository) SearchProjectSetSubjectRecords(
	ctx context.Context, workspaceID uint, search string, page, pageSize int,
) (project.ProjectSetSubjectRecordPage, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 {
		return project.ProjectSetSubjectRecordPage{}, project.ErrProjectSetSubjectInvalid
	}
	query := repository.activeProjectSets(ctx, workspaceID)
	if value := strings.ToLower(strings.TrimSpace(search)); value != "" {
		pattern := "%" + projectSubjectLike(value) + "%"
		query = query.Where("LOWER(name) LIKE ? ESCAPE '\\' OR LOWER(code) LIKE ? ESCAPE '\\'", pattern, pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return project.ProjectSetSubjectRecordPage{}, err
	}
	var rows []ProjectSet
	if err := query.Select("id, name, code").Order("name ASC, code ASC, id ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return project.ProjectSetSubjectRecordPage{}, err
	}
	return project.ProjectSetSubjectRecordPage{
		Items: projectSetSubjectRecords(rows), Page: page, PageSize: pageSize, Total: total,
	}, nil
}

func (repository *ProjectSetSubjectRepository) ResolveProjectSetSubjectRecords(
	ctx context.Context, workspaceID uint, ids []uint,
) ([]project.ProjectSetSubjectRecord, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 {
		return nil, project.ErrProjectSetSubjectInvalid
	}
	if len(ids) == 0 {
		return []project.ProjectSetSubjectRecord{}, nil
	}
	var rows []ProjectSet
	if err := repository.activeProjectSets(ctx, workspaceID).Select("id, name, code").
		Where("id IN ?", ids).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return projectSetSubjectRecords(rows), nil
}

func (repository *ProjectSetSubjectRepository) MatchProjectSetSubjectRecords(
	ctx context.Context, workspaceID, userID uint, ids []uint,
) (map[uint]bool, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || userID == 0 {
		return nil, project.ErrProjectSetSubjectInvalid
	}
	result := make(map[uint]bool, len(ids))
	for _, id := range ids {
		result[id] = false
	}
	if len(ids) == 0 {
		return result, nil
	}
	var matched []uint
	err := repository.activeProjectSets(ctx, workspaceID).
		Where("id IN ?", ids).
		Where(`created_by_user_id = ? OR EXISTS (
			SELECT 1 FROM plan_units pu
			WHERE pu.workspace_id = project_sets.workspace_id
			  AND pu.project_new_id = project_sets.id
			  AND (pu.owner_user_id = ? OR EXISTS (
				SELECT 1 FROM plan_unit_participants pup
				WHERE pup.workspace_id = pu.workspace_id
				  AND pup.plan_unit_id = pu.id
				  AND pup.user_id = ?
			  ))
		)`, userID, userID, userID).
		Pluck("id", &matched).Error
	if err != nil {
		return nil, err
	}
	for _, id := range matched {
		result[id] = true
	}
	return result, nil
}

func (repository *ProjectSetSubjectRepository) activeProjectSets(ctx context.Context, workspaceID uint) *gorm.DB {
	return repository.db.WithContext(ctx).Model(&ProjectSet{}).
		Where("workspace_id = ? AND status = ? AND mutation_state = ?", workspaceID, "active", "")
}

func projectSetSubjectRecords(rows []ProjectSet) []project.ProjectSetSubjectRecord {
	result := make([]project.ProjectSetSubjectRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, project.ProjectSetSubjectRecord{ID: row.ID, Name: row.Name, Code: row.Code})
	}
	return result
}

func projectSubjectLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

var _ project.ProjectSetSubjectStore = (*ProjectSetSubjectRepository)(nil)
