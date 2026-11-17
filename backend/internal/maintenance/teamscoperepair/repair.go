package teamscoperepair

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Runner struct {
	db             *gorm.DB
	options        Options
	membershipPlan []membershipCandidate
}

type projectSetRow struct {
	ID          uint
	Name        string
	TeamID      uint
	ValidTeamID *uint
}

func New(db *gorm.DB, options Options) (*Runner, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}
	if options.WorkspaceID == 0 {
		return nil, fmt.Errorf("workspace id is required")
	}
	if options.TargetTeamID == 0 {
		return nil, fmt.Errorf("target team id is required")
	}
	seen := map[uint]struct{}{}
	ids := make([]uint, 0, len(options.ProjectSetIDs))
	for _, id := range options.ProjectSetIDs {
		if id == 0 {
			return nil, fmt.Errorf("project set ids must be positive")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	options.ProjectSetIDs = ids
	if !options.DryRun && len(options.ProjectSetIDs) == 0 {
		return nil, fmt.Errorf("apply requires explicit project set ids")
	}
	return &Runner{db: db, options: options}, nil
}

func (r *Runner) Run(ctx context.Context) (Report, error) {
	report := Report{
		DryRun: r.options.DryRun, WorkspaceID: r.options.WorkspaceID,
		TargetTeamID: r.options.TargetTeamID, Tables: map[string]TableReport{},
		IssuesBefore: map[string]int64{}, ProjectedIssuesAfter: map[string]int64{},
		StartedAt: time.Now().UTC(),
	}
	finish := func(err error) (Report, error) {
		report.FinishedAt = time.Now().UTC()
		return report, err
	}
	if err := r.validateTarget(r.db.WithContext(ctx)); err != nil {
		return finish(err)
	}
	if r.options.DryRun {
		if err := r.prepare(r.db.WithContext(ctx), &report, false); err != nil {
			return finish(err)
		}
		return finish(nil)
	}

	attemptedWrite := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.prepare(tx, &report, true); err != nil {
			return err
		}
		if len(report.ProjectSets) == 0 {
			return nil
		}
		attemptedWrite = true
		if err := r.apply(tx, &report); err != nil {
			return err
		}
		ids := reportProjectSetIDs(report.ProjectSets)
		after, err := r.validationIssues(tx, ids, true)
		if err != nil {
			return err
		}
		report.IssuesAfter = after
		membershipIssues, err := r.membershipIssues(tx, ids)
		if err != nil {
			return err
		}
		report.IssuesAfter["team_members"] = membershipIssues
		if totalIssues(report.IssuesAfter) != 0 {
			return fmt.Errorf("team scope validation failed after repair")
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		report.RolledBack = attemptedWrite
		return finish(err)
	}
	report.Applied = len(report.ProjectSets) > 0
	return finish(nil)
}

func (r *Runner) validateTarget(db *gorm.DB) error {
	var workspaceCount int64
	if err := db.Table("workspaces").Where("id = ?", r.options.WorkspaceID).
		Count(&workspaceCount).Error; err != nil {
		return err
	}
	if workspaceCount != 1 {
		return fmt.Errorf("workspace %d not found", r.options.WorkspaceID)
	}
	var teamCount int64
	if err := db.Table("teams").Where(
		"id = ? AND workspace_id = ?", r.options.TargetTeamID, r.options.WorkspaceID,
	).Count(&teamCount).Error; err != nil {
		return err
	}
	if teamCount != 1 {
		return fmt.Errorf("team %d does not belong to workspace %d",
			r.options.TargetTeamID, r.options.WorkspaceID)
	}
	return nil
}

func (r *Runner) prepare(db *gorm.DB, report *Report, lock bool) error {
	projects, err := r.loadCandidates(db, lock)
	if err != nil {
		return err
	}
	report.ProjectSets = make([]ProjectSetReport, 0, len(projects))
	for _, project := range projects {
		report.ProjectSets = append(report.ProjectSets, ProjectSetReport{
			ID: project.ID, Name: project.Name, CurrentTeamID: project.TeamID,
		})
	}
	if len(projects) == 0 {
		report.Ready = true
		return nil
	}
	ids := reportProjectSetIDs(report.ProjectSets)
	if err := r.collectTableCounts(db, ids, report); err != nil {
		return err
	}
	report.IssuesBefore, err = r.validationIssues(db, ids, true)
	if err != nil {
		return err
	}
	report.ProjectedIssuesAfter, err = r.validationIssues(db, ids, false)
	if err != nil {
		return err
	}
	if err := r.collectMembershipPlan(db, ids, report); err != nil {
		return err
	}
	report.IssuesBefore["team_members"] = report.TeamMemberships.WouldInsert +
		int64(len(report.TeamMemberships.UnresolvedFactUserIDs))
	report.ProjectedIssuesAfter["team_members"] = int64(len(report.TeamMemberships.UnresolvedFactUserIDs))
	report.Ready = totalIssues(report.ProjectedIssuesAfter) == 0
	if !report.Ready {
		return fmt.Errorf("selected project hierarchy or team roster has unresolved issues; refusing repair")
	}
	return nil
}

func (r *Runner) loadCandidates(db *gorm.DB, lock bool) ([]projectSetRow, error) {
	query := db.Table("project_sets AS ps").
		Select("ps.id, ps.name, ps.team_id, t.id AS valid_team_id").
		Joins("LEFT JOIN teams t ON t.id = ps.team_id AND t.workspace_id = ps.workspace_id").
		Where("ps.workspace_id = ?", r.options.WorkspaceID)
	if len(r.options.ProjectSetIDs) > 0 {
		query = query.Where("ps.id IN ?", r.options.ProjectSetIDs)
	} else {
		query = query.Where("t.id IS NULL")
	}
	if lock {
		switch db.Dialector.Name() {
		case "postgres":
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "ps"}})
		case "mysql":
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
	}
	var rows []projectSetRow
	if err := query.Order("ps.id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(r.options.ProjectSetIDs) > 0 && len(rows) != len(r.options.ProjectSetIDs) {
		return nil, fmt.Errorf("one or more selected project sets do not belong to workspace %d",
			r.options.WorkspaceID)
	}
	candidates := make([]projectSetRow, 0, len(rows))
	for _, row := range rows {
		if row.ValidTeamID == nil {
			candidates = append(candidates, row)
			continue
		}
		if row.TeamID != r.options.TargetTeamID {
			return nil, fmt.Errorf("project set %d already belongs to valid team %d; refusing reassignment",
				row.ID, row.TeamID)
		}
		if len(r.options.ProjectSetIDs) > 0 {
			candidates = append(candidates, row)
		}
	}
	return candidates, nil
}

func reportProjectSetIDs(projects []ProjectSetReport) []uint {
	ids := make([]uint, 0, len(projects))
	for _, project := range projects {
		ids = append(ids, project.ID)
	}
	return ids
}

func totalIssues(values map[string]int64) int64 {
	var total int64
	for _, value := range values {
		total += value
	}
	return total
}

func (r *Runner) applyMemberships(tx *gorm.DB, report *Report) error {
	for _, candidate := range r.membershipPlan {
		if !candidate.Missing {
			continue
		}
		member := identitygorm.TeamMember{
			WorkspaceID: r.options.WorkspaceID,
			TeamID:      r.options.TargetTeamID,
			UserID:      candidate.UserID,
			Role:        "member",
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&member)
		if result.Error != nil {
			return fmt.Errorf("insert team member for platform user %d: %w", candidate.FactUserID, result.Error)
		}
		report.TeamMemberships.Inserted += result.RowsAffected
	}
	return nil
}
