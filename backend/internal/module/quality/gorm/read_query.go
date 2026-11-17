package qualitygorm

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"gorm.io/gorm"
)

type BugReadRepository struct{ db *gorm.DB }

func NewBugReadRepository(db *gorm.DB) *BugReadRepository { return &BugReadRepository{db: db} }

var _ quality.BugReadStore = (*BugReadRepository)(nil)

func (repository *BugReadRepository) ListBugs(ctx context.Context, workspaceID uint, filter quality.BugReadFilter) (quality.BugReadPage, error) {
	if repository == nil || repository.db == nil {
		return quality.BugReadPage{}, quality.ErrBugReadInvalid
	}
	query := repository.db.WithContext(ctx).Model(&QualityBug{}).Where("workspace_id = ?", workspaceID)
	if filter.ProjectSetID > 0 {
		query = query.Where("project_set_id = ?", filter.ProjectSetID)
	}
	if filter.PlanUnitID > 0 {
		query = query.Where("project_id = ?", filter.PlanUnitID)
	}
	if filter.RequirementID > 0 {
		query = query.Where("requirement_id = ?", filter.RequirementID)
	}
	if filter.Query != "" {
		like := "%" + escapeBugReadLike(strings.ToLower(filter.Query)) + "%"
		query = query.Where("LOWER(title) LIKE ? ESCAPE '\\' OR LOWER(bug_key) LIKE ? ESCAPE '\\'", like, like)
	}
	if len(filter.Statuses) > 0 {
		query = query.Where("LOWER(TRIM(status)) IN ?", filter.Statuses)
	}
	if len(filter.Priorities) > 0 {
		query = query.Where("LOWER(TRIM(priority)) IN ?", filter.Priorities)
	}
	if len(filter.SeverityKeys) > 0 {
		query = query.Where("LOWER(TRIM(severity_key)) IN ?", filter.SeverityKeys)
	}
	if len(filter.FoundEnvironments) > 0 {
		query = query.Where("LOWER(TRIM(found_environment)) IN ?", filter.FoundEnvironments)
	}
	if filter.FoundVersion != "" {
		query = query.Where("LOWER(TRIM(found_version)) = ?", filter.FoundVersion)
	}
	if len(filter.BugTypes) > 0 {
		query = query.Where("LOWER(TRIM(bug_type)) IN ?", filter.BugTypes)
	}
	if len(filter.DefectDomains) == 1 {
		if filter.DefectDomains[0] == quality.BugDefectDomainUI {
			query = query.Where("LOWER(TRIM(defect_domain)) = ?", quality.BugDefectDomainUI)
		} else {
			query = query.Where(
				"defect_domain IS NULL OR TRIM(defect_domain) = '' OR LOWER(TRIM(defect_domain)) <> ?",
				quality.BugDefectDomainUI,
			)
		}
	}
	if filter.CurrentOwnerID > 0 {
		reviewerStatuses := []string{"review", "done", "canceled", "cancelled"}
		query = query.Where(
			"((LOWER(TRIM(status)) IN ? AND COALESCE(reviewer_user_id, reporter_user_id) = ?) OR "+
				"((LOWER(TRIM(status)) NOT IN ? OR status IS NULL) AND assignee_user_id = ?))",
			reviewerStatuses, filter.CurrentOwnerID, reviewerStatuses, filter.CurrentOwnerID,
		)
	}
	if filter.AssigneeID > 0 {
		query = query.Where("assignee_user_id = ?", filter.AssigneeID)
	}
	if filter.ReporterUserID > 0 {
		query = query.Where("reporter_user_id = ?", filter.ReporterUserID)
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return quality.BugReadPage{}, err
	}
	var rows []QualityBug
	if err := query.Order("id DESC").Limit(filter.Limit).Offset(filter.Offset).Find(&rows).Error; err != nil {
		return quality.BugReadPage{}, err
	}
	items := make([]quality.BugSnapshot, 0, len(rows))
	for _, row := range rows {
		items = append(items, bugReadSnapshot(row))
	}
	return quality.BugReadPage{Items: items, Total: total}, nil
}

func (repository *BugReadRepository) GetBug(ctx context.Context, workspaceID, bugID uint) (quality.BugSnapshot, error) {
	if repository == nil || repository.db == nil {
		return quality.BugSnapshot{}, quality.ErrBugReadInvalid
	}
	var row QualityBug
	err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, bugID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return quality.BugSnapshot{}, quality.ErrBugReadNotFound
	}
	if err != nil {
		return quality.BugSnapshot{}, err
	}
	return bugReadSnapshot(row), nil
}

func (repository *BugReadRepository) ListBugActivities(ctx context.Context, workspaceID, bugID uint, limit, offset int) (quality.BugActivityPage, error) {
	if repository == nil || repository.db == nil {
		return quality.BugActivityPage{}, quality.ErrBugReadInvalid
	}
	var exists int64
	if err := repository.db.WithContext(ctx).Model(&QualityBug{}).Where("workspace_id = ? AND id = ?", workspaceID, bugID).Count(&exists).Error; err != nil {
		return quality.BugActivityPage{}, err
	}
	if exists != 1 {
		return quality.BugActivityPage{}, quality.ErrBugReadNotFound
	}
	query := repository.db.WithContext(ctx).Model(&QualityBugActivity{}).
		Where("workspace_id = ? AND quality_bug_id = ?", workspaceID, bugID)
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return quality.BugActivityPage{}, err
	}
	var rows []QualityBugActivity
	// 按发生时间倒序展示（id 仅作同秒并列的次序兜底）：活动的写入顺序
	// 可能与 created_at 不一致（自动化补写/回填时间），按 id 排会让
	// 时间线前后跳跃。
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return quality.BugActivityPage{}, err
	}
	items := make([]quality.BugActivitySnapshot, 0, len(rows))
	for _, row := range rows {
		items = append(items, quality.BugActivitySnapshot{
			ID: row.ID, WorkspaceID: row.WorkspaceID, QualityBugID: row.QualityBugID,
			UserID: row.UserID, Action: row.Action, FieldKey: row.FieldKey, OldValue: row.OldValue,
			NewValue: row.NewValue, Summary: row.Summary, CreatedAt: row.CreatedAt,
		})
	}
	return quality.BugActivityPage{Items: items, Total: total}, nil
}

func bugReadSnapshot(row QualityBug) quality.BugSnapshot {
	bugKey := strings.TrimSpace(row.BugKey)
	if bugKey == "" {
		bugKey = "BUG-" + strconv.FormatUint(uint64(row.ID), 10)
	}
	return quality.BugSnapshot{
		ReproductionMaterials: quality.DecodeMaterials(row.ReproductionMaterialsJSON), ResolutionMaterials: quality.DecodeMaterials(row.ResolutionMaterialsJSON),
		ID: row.ID, WorkspaceID: row.WorkspaceID, TeamID: row.TeamID, ProjectSetID: row.ProjectSetID,
		PlanUnitID: row.ProjectID, RequirementID: row.RequirementID, BugKey: bugKey, Title: row.Title,
		Status: strings.ToLower(strings.TrimSpace(row.Status)), Priority: row.Priority, BugType: row.BugType,
		DefectDomain: quality.NormalizeBugDefectDomain(row.DefectDomain),
		SeverityKey:  row.SeverityKey, DueAt: row.DueAt, FoundEnvironment: row.FoundEnvironment,
		OperatingSystem: row.OperatingSystem, Browser: row.Browser, FoundVersion: row.FoundVersion,
		ReproductionSteps: row.ReproductionSteps, ExpectedResult: row.ExpectedResult, ActualResult: row.ActualResult,
		ResolutionReason: row.ResolutionReason, ResolutionNote: row.ResolutionNote,
		AssigneeUserID: row.AssigneeUserID, ReporterUserID: row.ReporterUserID, ReviewerUserID: row.ReviewerUserID,
		SourceExecutionID: row.SourceExecutionID, SourceTestID: row.SourceTestID, ResolvedAt: row.ResolvedAt,
		ReviewedAt: row.ReviewedAt, CompletedAt: row.CompletedAt, CanceledAt: row.CanceledAt,
		StatusChangedAt: row.StatusChangedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func escapeBugReadLike(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	return strings.ReplaceAll(value, "_", "\\_")
}
