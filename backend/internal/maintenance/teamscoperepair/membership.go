package teamscoperepair

import (
	"fmt"
	"sort"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

type membershipCandidate struct {
	FactUserID uint
	UserID     uint
	Missing    bool
}

func (r *Runner) collectMembershipPlan(db *gorm.DB, projectIDs []uint, report *Report) error {
	factIDs, err := r.referencedFactUserIDs(db, projectIDs)
	if err != nil {
		return err
	}
	report.TeamMemberships.ReferencedFactUserIDs = factIDs
	if len(factIDs) == 0 {
		r.membershipPlan = nil
		return nil
	}

	resolved, unresolved, err := r.resolveWorkspaceUsers(db, factIDs)
	if err != nil {
		return err
	}
	report.TeamMemberships.UnresolvedFactUserIDs = unresolved
	userIDs := make([]uint, 0, len(resolved))
	for _, candidate := range resolved {
		userIDs = append(userIDs, candidate.UserID)
	}
	var existingIDs []uint
	if len(userIDs) > 0 {
		if err := db.Model(&identitygorm.TeamMember{}).
			Where("workspace_id = ? AND team_id = ? AND user_id IN ?",
				r.options.WorkspaceID, r.options.TargetTeamID, userIDs).
			Pluck("user_id", &existingIDs).Error; err != nil {
			return fmt.Errorf("load existing team members: %w", err)
		}
	}
	existing := make(map[uint]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		existing[id] = struct{}{}
	}
	missingFacts := make([]uint, 0)
	for index := range resolved {
		_, found := existing[resolved[index].UserID]
		resolved[index].Missing = !found
		if !found {
			missingFacts = append(missingFacts, resolved[index].FactUserID)
		}
	}
	r.membershipPlan = resolved
	report.TeamMemberships.MissingFactUserIDs = missingFacts
	report.TeamMemberships.Existing = int64(len(resolved) - len(missingFacts))
	report.TeamMemberships.WouldInsert = int64(len(missingFacts))
	return nil
}

func (r *Runner) resolveWorkspaceUsers(
	db *gorm.DB, factIDs []uint,
) ([]membershipCandidate, []uint, error) {
	var memberIDs []uint
	if err := db.Model(&identitygorm.WorkspaceMember{}).
		Where("workspace_id = ? AND enabled = ? AND user_id IN ?", r.options.WorkspaceID, true, factIDs).
		Distinct().Pluck("user_id", &memberIDs).Error; err != nil {
		return nil, nil, fmt.Errorf("resolve referenced workspace users: %w", err)
	}
	available := make(map[uint]struct{}, len(memberIDs))
	for _, userID := range memberIDs {
		available[userID] = struct{}{}
	}
	resolved := make([]membershipCandidate, 0, len(factIDs))
	unresolved := make([]uint, 0)
	for _, factID := range factIDs {
		if _, exists := available[factID]; !exists {
			unresolved = append(unresolved, factID)
			continue
		}
		resolved = append(resolved, membershipCandidate{
			FactUserID: factID, UserID: factID,
		})
	}
	return resolved, unresolved, nil
}

func (r *Runner) membershipIssues(db *gorm.DB, projectIDs []uint) (int64, error) {
	report := Report{}
	if err := r.collectMembershipPlan(db, projectIDs, &report); err != nil {
		return 0, err
	}
	return report.TeamMemberships.WouldInsert +
		int64(len(report.TeamMemberships.UnresolvedFactUserIDs)), nil
}

func (r *Runner) referencedFactUserIDs(db *gorm.DB, projectIDs []uint) ([]uint, error) {
	seen := map[uint]struct{}{}
	add := func(values ...uint) {
		for _, value := range values {
			if value > 0 {
				seen[value] = struct{}{}
			}
		}
	}
	pluck := func(table, column, projectColumn string) error {
		var ids []uint
		query := db.Table(table).Where("workspace_id = ?", r.options.WorkspaceID).
			Where(projectColumn+" IN ?", projectIDs).
			Where(column + " IS NOT NULL AND " + column + " > 0")
		if table == "quality_bugs" {
			query = query.Where("deleted_at IS NULL")
		}
		err := query.Pluck(column, &ids).Error
		if err != nil {
			return fmt.Errorf("load %s.%s user references: %w", table, column, err)
		}
		add(ids...)
		return nil
	}
	for _, ref := range []struct{ table, column, projectColumn string }{
		{"project_sets", "owner_user_id", "id"},
		{"plan_units", "owner_user_id", "project_new_id"},
		{"project_permissions", "user_id", "project_new_id"},
		{"requirements", "product_owner_user_id", "project_new_id"},
		{"requirements", "design_owner_user_id", "project_new_id"},
		{"work_tasks", "assignee_id", "project_set_id"},
		{"work_tasks", "reporter_user_id", "project_set_id"},
		{"quality_bugs", "assignee_user_id", "project_set_id"},
		{"quality_bugs", "reporter_user_id", "project_set_id"},
		{"quality_bugs", "reviewer_user_id", "project_set_id"},
		{"project_news", "user_id", "project_set_id"},
	} {
		if err := pluck(ref.table, ref.column, ref.projectColumn); err != nil {
			return nil, err
		}
	}

	var requirements []requirementgorm.Requirement
	if err := db.Select("developer_user_ids").Where(
		"workspace_id = ? AND project_new_id IN ?", r.options.WorkspaceID, projectIDs,
	).Find(&requirements).Error; err != nil {
		return nil, fmt.Errorf("load requirement developer references: %w", err)
	}
	for _, requirement := range requirements {
		add(requirement.DeveloperUserIDs...)
	}

	var planIDs, taskIDs, bugIDs []uint
	if err := db.Model(&projectgorm.PlanUnit{}).Where(
		"workspace_id = ? AND project_new_id IN ?", r.options.WorkspaceID, projectIDs,
	).Pluck("id", &planIDs).Error; err != nil {
		return nil, err
	}
	if len(planIDs) > 0 {
		var owners []uint
		if err := db.Model(&projectgorm.DeliveryNode{}).Where(
			"workspace_id = ? AND plan_unit_id IN ? AND owner_user_id IS NOT NULL AND owner_user_id > 0",
			r.options.WorkspaceID, planIDs,
		).Pluck("owner_user_id", &owners).Error; err != nil {
			return nil, err
		}
		add(owners...)
	}
	if err := db.Model(&taskgorm.WorkTask{}).Where(
		"workspace_id = ? AND project_set_id IN ?", r.options.WorkspaceID, projectIDs,
	).Pluck("id", &taskIDs).Error; err != nil {
		return nil, err
	}
	for _, ref := range []struct{ table, column string }{
		{"work_task_comments", "user_id"}, {"work_task_watchers", "user_id"},
		{"work_task_activities", "user_id"}, {"work_task_attachments", "uploader_user_id"},
	} {
		if len(taskIDs) == 0 {
			break
		}
		var ids []uint
		if err := db.Table(ref.table).Where("workspace_id = ? AND work_task_id IN ?",
			r.options.WorkspaceID, taskIDs).Pluck(ref.column, &ids).Error; err != nil {
			return nil, err
		}
		add(ids...)
	}
	if err := db.Model(&qualitygorm.QualityBug{}).Where(
		"workspace_id = ? AND project_set_id IN ?", r.options.WorkspaceID, projectIDs,
	).Pluck("id", &bugIDs).Error; err != nil {
		return nil, err
	}
	if len(bugIDs) > 0 {
		var ids []uint
		if err := db.Model(&qualitygorm.QualityBugActivity{}).Where(
			"workspace_id = ? AND quality_bug_id IN ?", r.options.WorkspaceID, bugIDs,
		).Pluck("user_id", &ids).Error; err != nil {
			return nil, err
		}
		add(ids...)
	}

	result := make([]uint, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}
