package teamscoperepair

import (
	"fmt"

	"gorm.io/gorm"
)

type tableScope struct {
	Name          string
	ProjectColumn string
}

var teamScopeTables = []tableScope{
	{Name: "project_sets", ProjectColumn: "id"},
	{Name: "plan_units", ProjectColumn: "project_new_id"},
	{Name: "requirements", ProjectColumn: "project_new_id"},
	{Name: "project_permissions", ProjectColumn: "project_new_id"},
	{Name: "work_tasks", ProjectColumn: "project_set_id"},
	{Name: "quality_bugs", ProjectColumn: "project_set_id"},
}

func (r *Runner) collectTableCounts(db *gorm.DB, projectIDs []uint, report *Report) error {
	for _, scope := range teamScopeTables {
		query := db.Table(scope.Name).Where("workspace_id = ?", r.options.WorkspaceID).
			Where(fmt.Sprintf("%s IN ?", scope.ProjectColumn), projectIDs)
		var scoped int64
		if err := query.Count(&scoped).Error; err != nil {
			return fmt.Errorf("count %s: %w", scope.Name, err)
		}
		var wouldUpdate int64
		if err := query.Where("team_id <> ?", r.options.TargetTeamID).
			Count(&wouldUpdate).Error; err != nil {
			return fmt.Errorf("count repair rows for %s: %w", scope.Name, err)
		}
		report.Tables[scope.Name] = TableReport{Scoped: scoped, WouldUpdate: wouldUpdate}
	}
	return nil
}

func (r *Runner) apply(tx *gorm.DB, report *Report) error {
	projectIDs := reportProjectSetIDs(report.ProjectSets)
	for _, scope := range teamScopeTables {
		query := tx.Table(scope.Name).Where("workspace_id = ?", r.options.WorkspaceID).
			Where(fmt.Sprintf("%s IN ?", scope.ProjectColumn), projectIDs).
			Where("team_id <> ?", r.options.TargetTeamID)
		result := query.UpdateColumn("team_id", r.options.TargetTeamID)
		if result.Error != nil {
			return fmt.Errorf("update %s: %w", scope.Name, result.Error)
		}
		table := report.Tables[scope.Name]
		table.Updated = result.RowsAffected
		report.Tables[scope.Name] = table
		if result.RowsAffected != table.WouldUpdate {
			return fmt.Errorf("%s changed during repair: expected %d rows, updated %d",
				scope.Name, table.WouldUpdate, result.RowsAffected)
		}
	}
	return r.applyMemberships(tx, report)
}

func (r *Runner) validationIssues(
	db *gorm.DB,
	projectIDs []uint,
	includeTeam bool,
) (map[string]int64, error) {
	checks := []struct {
		name  string
		query *gorm.DB
	}{
		{name: "project_sets", query: r.projectSetIssues(db, projectIDs, includeTeam)},
		{name: "plan_units", query: r.planUnitIssues(db, projectIDs, includeTeam)},
		{name: "requirements", query: r.requirementIssues(db, projectIDs, includeTeam)},
		{name: "project_permissions", query: r.permissionIssues(db, projectIDs, includeTeam)},
		{name: "work_tasks", query: r.taskIssues(db, projectIDs, includeTeam)},
		{name: "quality_bugs", query: r.qualityIssues(db, projectIDs, includeTeam)},
	}
	issues := make(map[string]int64, len(checks))
	for _, check := range checks {
		var count int64
		if err := check.query.Count(&count).Error; err != nil {
			return nil, fmt.Errorf("validate %s: %w", check.name, err)
		}
		issues[check.name] = count
	}
	return issues, nil
}

func (r *Runner) projectSetIssues(db *gorm.DB, ids []uint, includeTeam bool) *gorm.DB {
	query := db.Table("project_sets AS ps").Where("ps.id IN ?", ids).
		Where("ps.workspace_id <> ?", r.options.WorkspaceID)
	if includeTeam {
		query = db.Table("project_sets AS ps").Where("ps.id IN ?", ids).
			Where("(ps.workspace_id <> ? OR ps.team_id <> ?)",
				r.options.WorkspaceID, r.options.TargetTeamID)
	}
	return query
}

func (r *Runner) planUnitIssues(db *gorm.DB, ids []uint, includeTeam bool) *gorm.DB {
	condition := "(ps.workspace_id <> ? OR pu.workspace_id <> ps.workspace_id)"
	args := []any{r.options.WorkspaceID}
	if includeTeam {
		condition = "(ps.workspace_id <> ? OR ps.team_id <> ? OR pu.workspace_id <> ps.workspace_id OR pu.team_id <> ps.team_id)"
		args = []any{r.options.WorkspaceID, r.options.TargetTeamID}
	}
	return db.Table("plan_units AS pu").
		Joins("JOIN project_sets ps ON ps.id = pu.project_new_id").
		Where("ps.id IN ?", ids).Where(condition, args...)
}

func (r *Runner) requirementIssues(db *gorm.DB, ids []uint, includeTeam bool) *gorm.DB {
	condition := `(ps.workspace_id <> ? OR req.workspace_id <> ps.workspace_id OR
		pu.id IS NULL OR pu.workspace_id <> ps.workspace_id OR pu.project_new_id <> ps.id)`
	args := []any{r.options.WorkspaceID}
	if includeTeam {
		condition = `(ps.workspace_id <> ? OR ps.team_id <> ? OR req.workspace_id <> ps.workspace_id OR
			req.team_id <> ps.team_id OR pu.id IS NULL OR pu.workspace_id <> ps.workspace_id OR
			pu.project_new_id <> ps.id OR pu.team_id <> ps.team_id)`
		args = []any{r.options.WorkspaceID, r.options.TargetTeamID}
	}
	return db.Table("requirements AS req").
		Joins("JOIN project_sets ps ON ps.id = req.project_new_id").
		Joins("LEFT JOIN plan_units pu ON pu.id = req.version_task_new_id").
		Where("ps.id IN ?", ids).Where(condition, args...)
}

func (r *Runner) permissionIssues(db *gorm.DB, ids []uint, includeTeam bool) *gorm.DB {
	condition := "(ps.workspace_id <> ? OR permission.workspace_id <> ps.workspace_id)"
	args := []any{r.options.WorkspaceID}
	if includeTeam {
		condition = `(ps.workspace_id <> ? OR ps.team_id <> ? OR
			permission.workspace_id <> ps.workspace_id OR permission.team_id <> ps.team_id)`
		args = []any{r.options.WorkspaceID, r.options.TargetTeamID}
	}
	return db.Table("project_permissions AS permission").
		Joins("JOIN project_sets ps ON ps.id = permission.project_new_id").
		Where("ps.id IN ?", ids).Where(condition, args...)
}

func (r *Runner) taskIssues(db *gorm.DB, ids []uint, includeTeam bool) *gorm.DB {
	condition := `(ps.id IS NULL OR ps.workspace_id <> ? OR task.workspace_id <> ps.workspace_id OR
		pu.id IS NULL OR pu.workspace_id <> ps.workspace_id OR pu.project_new_id <> ps.id OR
		req.id IS NULL OR req.workspace_id <> ps.workspace_id OR req.project_new_id <> ps.id OR
		req.version_task_new_id <> pu.id)`
	args := []any{r.options.WorkspaceID}
	if includeTeam {
		condition = `(ps.id IS NULL OR ps.workspace_id <> ? OR ps.team_id <> ? OR
			task.workspace_id <> ps.workspace_id OR task.team_id <> ps.team_id OR
			pu.id IS NULL OR pu.workspace_id <> ps.workspace_id OR pu.project_new_id <> ps.id OR pu.team_id <> ps.team_id OR
			req.id IS NULL OR req.workspace_id <> ps.workspace_id OR req.project_new_id <> ps.id OR
			req.version_task_new_id <> pu.id OR req.team_id <> ps.team_id)`
		args = []any{r.options.WorkspaceID, r.options.TargetTeamID}
	}
	return db.Table("work_tasks AS task").
		Joins("LEFT JOIN project_sets ps ON ps.id = task.project_set_id").
		Joins("LEFT JOIN plan_units pu ON pu.id = task.project_id").
		Joins("LEFT JOIN requirements req ON req.id = task.requirement_id").
		Where("task.project_set_id IN ?", ids).Where(condition, args...)
}

func (r *Runner) qualityIssues(db *gorm.DB, ids []uint, includeTeam bool) *gorm.DB {
	condition := `(ps.id IS NULL OR ps.workspace_id <> ? OR bug.workspace_id <> ps.workspace_id OR
		pu.id IS NULL OR pu.workspace_id <> ps.workspace_id OR pu.project_new_id <> ps.id OR
		req.id IS NULL OR req.workspace_id <> ps.workspace_id OR req.project_new_id <> ps.id OR
		req.version_task_new_id <> pu.id)`
	args := []any{r.options.WorkspaceID}
	if includeTeam {
		condition = `(ps.id IS NULL OR ps.workspace_id <> ? OR ps.team_id <> ? OR
			bug.workspace_id <> ps.workspace_id OR bug.team_id <> ps.team_id OR
			pu.id IS NULL OR pu.workspace_id <> ps.workspace_id OR pu.project_new_id <> ps.id OR pu.team_id <> ps.team_id OR
			req.id IS NULL OR req.workspace_id <> ps.workspace_id OR req.project_new_id <> ps.id OR
			req.version_task_new_id <> pu.id OR req.team_id <> ps.team_id)`
		args = []any{r.options.WorkspaceID, r.options.TargetTeamID}
	}
	return db.Table("quality_bugs AS bug").
		Joins("LEFT JOIN project_sets ps ON ps.id = bug.project_set_id").
		Joins("LEFT JOIN plan_units pu ON pu.id = bug.project_id").
		Joins("LEFT JOIN requirements req ON req.id = bug.requirement_id").
		Where("bug.project_set_id IN ?", ids).Where(condition, args...)
}
