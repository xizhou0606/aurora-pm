package taskgorm

import (
	"context"
	"errors"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

type ReadRepository struct{ db *gorm.DB }

func NewReadRepository(db *gorm.DB) *ReadRepository { return &ReadRepository{db: db} }

func (repository *ReadRepository) GetTask(ctx context.Context, workspaceID uint, lookup task.Lookup) (task.Snapshot, error) {
	query := repository.db.WithContext(ctx).Where("workspace_id = ? AND mutation_state = ?", workspaceID, workTaskStateActive)
	if reference := strings.TrimSpace(lookup.Reference); reference != "" {
		query = query.Where("(task_key = ? OR title = ?)", reference, reference)
	} else {
		query = query.Where("id = ?", lookup.ID)
	}
	if lookup.ProjectSetID > 0 {
		query = query.Where("project_set_id = ?", lookup.ProjectSetID)
	}
	if lookup.PlanUnitID > 0 {
		query = query.Where("project_id = ?", lookup.PlanUnitID)
	}
	if lookup.RequirementID > 0 {
		query = query.Where("requirement_id = ?", lookup.RequirementID)
	}
	var matches []WorkTask
	if err := query.Order("id").Limit(2).Find(&matches).Error; err != nil {
		return task.Snapshot{}, err
	}
	if len(matches) == 0 {
		return task.Snapshot{}, task.ErrTaskNotFound
	}
	if len(matches) > 1 {
		return task.Snapshot{}, task.ErrTaskAmbiguous
	}
	row := matches[0]
	result := taskSnapshot(row)
	result.Children = []task.Reference{}
	if row.ParentID != nil {
		var parent WorkTask
		err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id = ? AND mutation_state = ?", workspaceID, *row.ParentID, workTaskStateActive).First(&parent).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return task.Snapshot{}, err
		}
		if err == nil {
			result.Parent = taskReference(parent)
		}
	}
	var children []WorkTask
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND parent_id = ? AND mutation_state = ?", workspaceID, row.ID, workTaskStateActive).Order("id").Find(&children).Error; err != nil {
		return task.Snapshot{}, err
	}
	for _, child := range children {
		result.Children = append(result.Children, *taskReference(child))
	}
	return result, nil
}

func taskSnapshot(row WorkTask) task.Snapshot {
	return task.Snapshot{
		ID: row.ID, WorkspaceID: row.WorkspaceID, TeamID: row.TeamID, ProjectSetID: row.ProjectSetID,
		PlanUnitID: row.ProjectID, RequirementID: row.RequirementID, TaskClientID: row.TaskClientID, Kind: row.Kind, TaskKey: row.TaskKey,
		Title: row.Title, Description: row.Description, Status: row.Status, Priority: row.Priority,
		AssigneeID: row.AssigneeID, ReporterUserID: row.ReporterUserID, ParentID: row.ParentID,
		EstimatedMinutes: row.EstimatedMinutes, StartsAt: row.StartsAt, DueAt: row.DueAt,
		CompletedAt: row.CompletedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Children: []task.Reference{},
	}
}

func taskReference(row WorkTask) *task.Reference {
	return &task.Reference{ID: row.ID, TaskKey: row.TaskKey, Title: row.Title}
}
