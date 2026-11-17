package taskgorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MemberHandoverRepository uses an outer project membership transaction.
// Task activities, audit, and outbox reuse the canonical batch update path.
type MemberHandoverRepository struct {
	db           *gorm.DB
	dependencies BatchMutationDependencyFactory
}

func NewMemberHandoverRepository(db *gorm.DB, dependencies BatchMutationDependencyFactory) *MemberHandoverRepository {
	return &MemberHandoverRepository{db: db, dependencies: dependencies}
}

func (r *MemberHandoverRepository) query(ctx context.Context, ws, unit uint, users []uint) *gorm.DB {
	return r.db.WithContext(ctx).Model(&WorkTask{}).
		Where("workspace_id = ? AND project_id = ? AND assignee_id IN ? AND status NOT IN ?", ws, unit, users, []string{"completed", "closed", "cancelled"}).Order("id ASC")
}

func (r *MemberHandoverRepository) ListMemberTasks(ctx context.Context, ws, unit uint, users []uint) ([]task.MemberTask, error) {
	rows := []task.MemberTask{}
	if ws == 0 || unit == 0 {
		return nil, task.ErrBatchMutationInvalid
	}
	if len(users) == 0 {
		return rows, nil
	}
	err := r.query(ctx, ws, unit, users).Select("id, task_key, title, status, assignee_id, updated_at").Find(&rows).Error
	return rows, err
}

func (r *MemberHandoverRepository) ResolveMemberTasks(ctx context.Context, ws, unit, actor uint, users []uint, resolution task.MemberTaskResolution) error {
	if ws == 0 || unit == 0 || actor == 0 || r.dependencies == nil {
		return task.ErrBatchMutationInvalid
	}
	if len(users) == 0 || len(resolution.Tasks) == 0 {
		return task.ErrMemberTasksChanged
	}
	var rows []WorkTask
	if err := r.query(ctx, ws, unit, users).Clauses(clause.Locking{Strength: "UPDATE"}).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) != len(resolution.Tasks) {
		return task.ErrMemberTasksChanged
	}
	expected := make(map[uint]task.MemberTask, len(resolution.Tasks))
	for _, row := range resolution.Tasks {
		if _, exists := expected[row.ID]; exists {
			return task.ErrMemberTasksChanged
		}
		expected[row.ID] = row
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		before, exists := expected[row.ID]
		if !exists || row.AssigneeID == nil || *row.AssigneeID != before.AssigneeID || !row.UpdatedAt.Equal(before.UpdatedAt) {
			return task.ErrMemberTasksChanged
		}
		ids = append(ids, row.ID)
	}
	dependencies, err := r.dependencies(r.db)
	if err != nil {
		return err
	}
	if dependencies.Users == nil || dependencies.Effects == nil || dependencies.Notifications == nil {
		return task.ErrBatchMutationUnavailable
	}
	_, err = applyTaskBatchUpdate(ctx, r.db, dependencies, task.BatchMutation{
		Operation: task.BatchUpdate, WorkspaceID: ws, PrincipalID: actor,
		Selector: task.BatchTaskSelector{IDs: ids, PlanUnitID: unit},
		Updates:  task.BatchTaskPatch{AssigneeID: task.Optional[uint]{Set: true, Value: resolution.AssigneeID}},
	})
	return err
}
