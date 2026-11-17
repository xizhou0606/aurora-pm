package codecontribgorm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/codecontrib"
	"gorm.io/gorm"
)

// FactsRepository implements the binding and fact stores. Collection replays
// the same windows repeatedly, so every upsert is idempotent by natural key,
// and binding upserts never downgrade admin-confirmed rows.
type FactsRepository struct{ db *gorm.DB }

func NewFactsRepository(db *gorm.DB) *FactsRepository { return &FactsRepository{db: db} }

func (repository *FactsRepository) UpsertBinding(ctx context.Context, row codecontrib.BindingRow) error {
	var existing AuthorBinding
	err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND provider = ? AND key_kind = ? AND author_key = ?",
			row.WorkspaceID, string(row.Provider), row.KeyKind, row.AuthorKey).
		First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		insert := bindingRow(row)
		return repository.db.WithContext(ctx).Create(&insert).Error
	}
	if err != nil {
		return err
	}
	if existing.Status == string(codecontrib.BindingMatched) &&
		existing.MatchedVia == string(codecontrib.MatchAdminConfirmed) {
		return nil // 管理员确认行永不降级
	}
	updates := map[string]any{
		"status": string(row.Status), "matched_via": string(row.MatchedVia),
		"display_name": row.DisplayName, "user_id": row.UserID,
		"suggested_user_id": row.SuggestedUserID, "updated_at": time.Now(),
	}
	return repository.db.WithContext(ctx).Model(&AuthorBinding{}).
		Where("id = ?", existing.ID).Updates(updates).Error
}

func (repository *FactsRepository) ListBindings(ctx context.Context, workspaceID uint, statuses []codecontrib.BindingStatus, limit int) ([]codecontrib.BindingRow, error) {
	query := repository.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if len(statuses) > 0 {
		converted := make([]string, 0, len(statuses))
		for _, status := range statuses {
			converted = append(converted, string(status))
		}
		query = query.Where("status IN ?", converted)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	var rows []AuthorBinding
	if err := query.Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]codecontrib.BindingRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, bindingModel(row))
	}
	repository.fillDisplayNames(ctx, workspaceID, result)
	return result, nil
}

// fillDisplayNames backfills empty Git author names from the commit facts so
// 确认队列显示人名而不是裸邮箱/登录名——旧数据行与尚未再次出现的作者
// 都能立即受益；同名多写时取最近一次提交的名字。
func (repository *FactsRepository) fillDisplayNames(ctx context.Context, workspaceID uint, rows []codecontrib.BindingRow) {
	emails, logins := make([]string, 0, 8), make([]string, 0, 8)
	for _, row := range rows {
		if row.DisplayName != "" || row.AuthorKey == "" {
			continue
		}
		if row.KeyKind == "email" {
			emails = append(emails, row.AuthorKey)
		} else if row.KeyKind == "login" {
			logins = append(logins, row.AuthorKey)
		}
	}
	if len(emails) == 0 && len(logins) == 0 {
		return
	}
	type nameKey struct {
		Author string
		Name   string
		At     time.Time
	}
	latest := func(pairs []nameKey) map[string]string {
		names := make(map[string]string, len(pairs))
		last := make(map[string]time.Time, len(pairs))
		for _, pair := range pairs {
			if pair.Author == "" || pair.Name == "" {
				continue
			}
			if at, seen := last[pair.Author]; seen && !pair.At.After(at) {
				continue
			}
			names[pair.Author] = pair.Name
			last[pair.Author] = pair.At
		}
		return names
	}
	byEmail, byLogin := map[string]string{}, map[string]string{}
	if len(emails) > 0 {
		var emailRows []nameKey
		if err := repository.db.WithContext(ctx).Model(&Commit{}).
			Select("author_email AS author, author_name AS name, committed_at AS at").
			Where("workspace_id = ? AND author_email IN ?", workspaceID, emails).
			Scan(&emailRows).Error; err != nil {
			return
		}
		byEmail = latest(emailRows)
	}
	if len(logins) > 0 {
		var loginRows []nameKey
		if err := repository.db.WithContext(ctx).Model(&Commit{}).
			Select("author_login AS author, author_name AS name, committed_at AS at").
			Where("workspace_id = ? AND author_login IN ?", workspaceID, logins).
			Scan(&loginRows).Error; err != nil {
			return
		}
		byLogin = latest(loginRows)
	}
	for index := range rows {
		if rows[index].DisplayName != "" {
			continue
		}
		switch rows[index].KeyKind {
		case "email":
			rows[index].DisplayName = byEmail[rows[index].AuthorKey]
		case "login":
			rows[index].DisplayName = byLogin[rows[index].AuthorKey]
		}
	}
}

func (repository *FactsRepository) ConfirmBinding(ctx context.Context, workspaceID, id, userID, actorID uint, at time.Time) (codecontrib.BindingRow, error) {
	var row AuthorBinding
	err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return codecontrib.BindingRow{}, errNotFound
	}
	if err != nil {
		return codecontrib.BindingRow{}, err
	}
	updates := map[string]any{
		"status": string(codecontrib.BindingMatched), "user_id": userID,
		"suggested_user_id": nil, "matched_via": string(codecontrib.MatchAdminConfirmed),
		"confirmed_by": actorID, "confirmed_at": at, "updated_at": time.Now(),
	}
	if err := repository.db.WithContext(ctx).Model(&AuthorBinding{}).
		Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		return codecontrib.BindingRow{}, err
	}
	return repository.getBinding(ctx, workspaceID, id)
}

func (repository *FactsRepository) RejectBinding(ctx context.Context, workspaceID, id, actorID uint) (codecontrib.BindingRow, error) {
	var row AuthorBinding
	err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return codecontrib.BindingRow{}, errNotFound
	}
	if err != nil {
		return codecontrib.BindingRow{}, err
	}
	updates := map[string]any{
		"status": string(codecontrib.BindingUnmatched), "user_id": nil,
		"suggested_user_id": nil, "matched_via": "",
		"confirmed_by": actorID, "confirmed_at": time.Now(), "updated_at": time.Now(),
	}
	if err := repository.db.WithContext(ctx).Model(&AuthorBinding{}).
		Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		return codecontrib.BindingRow{}, err
	}
	return repository.getBinding(ctx, workspaceID, id)
}

func (repository *FactsRepository) getBinding(ctx context.Context, workspaceID, id uint) (codecontrib.BindingRow, error) {
	var row AuthorBinding
	err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).First(&row).Error
	if err != nil {
		return codecontrib.BindingRow{}, err
	}
	return bindingModel(row), nil
}

func (repository *FactsRepository) ListBindingRows(ctx context.Context, workspaceID uint) ([]codecontrib.AuthorBinding, error) {
	var rows []AuthorBinding
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND user_id IS NOT NULL", workspaceID).
		Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]codecontrib.AuthorBinding, 0, len(rows))
	for _, row := range rows {
		if row.UserID == nil {
			continue
		}
		result = append(result, codecontrib.AuthorBinding{
			AuthorEmail: authorEmailKey(row), AuthorLogin: authorLoginKey(row),
			UserID: *row.UserID, MatchedVia: codecontrib.MatchVia(row.MatchedVia),
		})
	}
	return result, nil
}

func authorEmailKey(row AuthorBinding) string {
	if row.KeyKind == "email" {
		return row.AuthorKey
	}
	return ""
}

func authorLoginKey(row AuthorBinding) string {
	if row.KeyKind == "login" {
		return row.AuthorKey
	}
	return ""
}

func (repository *FactsRepository) UpsertCommits(ctx context.Context, workspaceID, repoID uint, facts []codecontrib.CommitFact) error {
	for _, fact := range facts {
		row, err := commitRow(workspaceID, repoID, fact)
		if err != nil {
			return err
		}
		var existing Commit
		err = repository.db.WithContext(ctx).
			Where("repo_id = ? AND sha = ?", repoID, fact.SHA).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := repository.db.WithContext(ctx).Model(&Commit{}).
			Where("id = ?", existing.ID).Updates(map[string]any{
			"author_email": row.AuthorEmail, "author_name": row.AuthorName,
			"author_login": row.AuthorLogin, "committer_email": row.CommitterEmail,
			"co_author_emails_json": row.CoAuthorEmailsJSON, "subject": row.Subject,
			"additions": row.Additions, "deletions": row.Deletions,
			"is_merge": row.IsMerge, "pr_delivered": row.PRDelivered,
			"pr_number": row.PRNumber, "files_json": row.FilesJSON,
			"diff_sample": row.DiffSample, "debug_hits": row.DebugHits,
			"todo_hits":  row.TodoHits,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (repository *FactsRepository) UpsertPullRequests(ctx context.Context, workspaceID, repoID uint, facts []codecontrib.PullRequestFact) error {
	for _, fact := range facts {
		row, err := pullRequestRow(workspaceID, repoID, fact)
		if err != nil {
			return err
		}
		var existing PullRequest
		err = repository.db.WithContext(ctx).
			Where("repo_id = ? AND number = ?", repoID, fact.Number).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := repository.db.WithContext(ctx).Model(&PullRequest{}).
			Where("id = ?", existing.ID).Updates(map[string]any{
			"title": row.Title, "author_login": row.AuthorLogin,
			"author_email": row.AuthorEmail, "merged_at": row.MergedAt,
			"merge_sha": row.MergeSHA, "base_branch": row.BaseBranch,
			"additions": row.Additions, "deletions": row.Deletions,
			"commit_shas_json": row.CommitSHAsJSON, "reviews_json": row.ReviewsJSON,
			"files_json": row.FilesJSON, "updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (repository *FactsRepository) ReplaceDailyWindow(ctx context.Context, workspaceID, repoID uint, days []string, rows []codecontrib.DailyContributionRow) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(days) > 0 {
			if err := tx.Where("workspace_id = ? AND repo_id = ? AND day IN ?",
				workspaceID, repoID, days).Delete(&DailyContribution{}).Error; err != nil {
				return err
			}
		}
		for _, row := range rows {
			record := DailyContribution{
				WorkspaceID: row.WorkspaceID, RepoID: row.RepoID, UserID: row.UserID,
				AuthorKey: row.AuthorKey, Day: row.Day, Commits: row.Commits,
				Additions: row.Additions, Deletions: row.Deletions,
				PRsMerged: row.PRsMerged, ReviewsCompleted: row.ReviewsCompleted,
				SourceAdditions: row.SourceAdditions, TestAdditions: row.TestAdditions,
				DocAdditions: row.DocAdditions, ConfigAdditions: row.ConfigAdditions,
				PRsReviewed: row.PRsReviewed,
			}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (repository *FactsRepository) ListDailyContributions(ctx context.Context, workspaceID uint) ([]codecontrib.DailyContributionRow, error) {
	var rows []DailyContribution
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).Order("day").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]codecontrib.DailyContributionRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, codecontrib.DailyContributionRow{
			WorkspaceID: row.WorkspaceID, RepoID: row.RepoID, UserID: row.UserID,
			AuthorKey: row.AuthorKey, Day: row.Day, Commits: row.Commits,
			Additions: row.Additions, Deletions: row.Deletions,
			PRsMerged: row.PRsMerged, ReviewsCompleted: row.ReviewsCompleted,
			SourceAdditions: row.SourceAdditions, TestAdditions: row.TestAdditions,
			DocAdditions: row.DocAdditions, ConfigAdditions: row.ConfigAdditions,
			PRsReviewed: row.PRsReviewed,
		})
	}
	return result, nil
}

// ListMemberCommits maps one window's commits onto platform member IDs via
// the workspace's matched bindings so reports can hand every commit (with its
// diff sample and quality hits) to the AI for per-commit review. Merge commits
// carry no diff evidence and are skipped.
func (repository *FactsRepository) ListMemberCommits(
	ctx context.Context, workspaceID uint, since, until time.Time,
) ([]codecontrib.MemberCommitRow, error) {
	var rows []Commit
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND committed_at >= ? AND committed_at < ?", workspaceID, since, until).
		Order("committed_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	emails, logins, err := repository.memberBindingMaps(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	result := make([]codecontrib.MemberCommitRow, 0, len(rows))
	for _, row := range rows {
		if row.IsMerge {
			continue
		}
		var files []codecontrib.FileChange
		_ = json.Unmarshal([]byte(row.FilesJSON), &files)
		result = append(result, codecontrib.MemberCommitRow{
			UserID:      resolveCommitMember(emails, logins, row),
			SHA:         row.SHA,
			CommittedAt: row.CommittedAt,
			Subject:     row.Subject,
			Additions:   row.Additions,
			Deletions:   row.Deletions,
			Files:       len(files),
			DebugHits:   row.DebugHits,
			TodoHits:    row.TodoHits,
			DiffSample:  row.DiffSample,
		})
	}
	return result, nil
}

func resolveCommitMember(emails, logins map[string]uint, row Commit) uint {
	if row.AuthorEmail != "" {
		if id, ok := emails[strings.ToLower(row.AuthorEmail)]; ok {
			return id
		}
	}
	if row.AuthorLogin != "" {
		if id, ok := logins[strings.ToLower(row.AuthorLogin)]; ok {
			return id
		}
	}
	return 0
}

// memberBindingMaps loads the matched email/login → member ID maps once per
// read; unmatched authors stay UserID 0 and reporting excludes them.
func (repository *FactsRepository) memberBindingMaps(
	ctx context.Context, workspaceID uint,
) (map[string]uint, map[string]uint, error) {
	var rows []AuthorBinding
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND status = ? AND user_id IS NOT NULL", workspaceID, "matched").
		Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	emails := make(map[string]uint, len(rows))
	logins := make(map[string]uint, len(rows))
	for _, row := range rows {
		switch row.KeyKind {
		case "email":
			emails[strings.ToLower(row.AuthorKey)] = *row.UserID
		case "login":
			logins[strings.ToLower(row.AuthorKey)] = *row.UserID
		}
	}
	return emails, logins, nil
}

func (repository *FactsRepository) ListCommitsInWindow(ctx context.Context, workspaceID, repoID uint, window codecontrib.Window) ([]codecontrib.CommitFact, error) {
	var rows []Commit
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND repo_id = ? AND committed_at >= ? AND committed_at < ?",
			workspaceID, repoID, window.Since, window.Until).
		Order("committed_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]codecontrib.CommitFact, 0, len(rows))
	for _, row := range rows {
		fact, err := commitModel(row)
		if err != nil {
			return nil, err
		}
		result = append(result, fact)
	}
	return result, nil
}

func (repository *FactsRepository) ListMergedPullRequestsInWindow(ctx context.Context, workspaceID, repoID uint, window codecontrib.Window) ([]codecontrib.PullRequestFact, error) {
	var rows []PullRequest
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND repo_id = ? AND merged_at IS NOT NULL AND merged_at >= ? AND merged_at < ?",
			workspaceID, repoID, window.Since, window.Until).
		Order("merged_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]codecontrib.PullRequestFact, 0, len(rows))
	for _, row := range rows {
		fact, err := pullRequestModel(row)
		if err != nil {
			return nil, err
		}
		result = append(result, fact)
	}
	return result, nil
}

func (repository *FactsRepository) GetSyncRun(ctx context.Context, repoID uint) (codecontrib.RepoSyncState, bool, error) {
	var row SyncRun
	err := repository.db.WithContext(ctx).Where("repo_id = ?", repoID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return codecontrib.RepoSyncState{}, false, nil
	}
	if err != nil {
		return codecontrib.RepoSyncState{}, false, err
	}
	return syncModel(row), true, nil
}

func (repository *FactsRepository) SaveSyncRun(ctx context.Context, state codecontrib.RepoSyncState) error {
	row := syncRow(state)
	var existing SyncRun
	err := repository.db.WithContext(ctx).Where("repo_id = ?", state.RepoID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return repository.db.WithContext(ctx).Create(&row).Error
	}
	if err != nil {
		return err
	}
	row.ID = existing.ID
	return repository.db.WithContext(ctx).Save(&row).Error
}

func (repository *FactsRepository) ListRepoSyncStates(ctx context.Context, workspaceID uint) ([]codecontrib.RepoSyncState, error) {
	var rows []SyncRun
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).Order("repo_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]codecontrib.RepoSyncState, 0, len(rows))
	for _, row := range rows {
		result = append(result, syncModel(row))
	}
	return result, nil
}

var _ codecontrib.BindingStore = (*FactsRepository)(nil)
var _ codecontrib.FactStore = (*FactsRepository)(nil)

func bindingRow(model codecontrib.BindingRow) AuthorBinding {
	return AuthorBinding{
		WorkspaceID: model.WorkspaceID, Provider: string(model.Provider),
		KeyKind: model.KeyKind, AuthorKey: model.AuthorKey, DisplayName: model.DisplayName,
		UserID: model.UserID, SuggestedUserID: model.SuggestedUserID,
		Status: string(model.Status), MatchedVia: string(model.MatchedVia),
		ConfirmedBy: model.ConfirmedBy, ConfirmedAt: model.ConfirmedAt,
	}
}

func bindingModel(row AuthorBinding) codecontrib.BindingRow {
	return codecontrib.BindingRow{
		ID: row.ID, WorkspaceID: row.WorkspaceID,
		Provider: codecontrib.Provider(row.Provider), KeyKind: row.KeyKind,
		AuthorKey: row.AuthorKey, DisplayName: row.DisplayName,
		UserID: row.UserID, SuggestedUserID: row.SuggestedUserID,
		Status: codecontrib.BindingStatus(row.Status), MatchedVia: codecontrib.MatchVia(row.MatchedVia),
		ConfirmedBy: row.ConfirmedBy, ConfirmedAt: row.ConfirmedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func commitRow(workspaceID, repoID uint, fact codecontrib.CommitFact) (Commit, error) {
	coAuthors, err := json.Marshal(fact.CoAuthorEmails)
	if err != nil {
		return Commit{}, err
	}
	files, err := json.Marshal(fact.Files)
	if err != nil {
		return Commit{}, err
	}
	return Commit{
		WorkspaceID: workspaceID, RepoID: repoID, SHA: fact.SHA,
		AuthorEmail: fact.AuthorEmail, AuthorName: fact.AuthorName,
		AuthorLogin: fact.AuthorLogin, CommitterEmail: fact.CommitterEmail,
		CoAuthorEmailsJSON: string(coAuthors), AuthorAt: fact.AuthorAt,
		CommittedAt: fact.CommittedAt, Subject: fact.Subject,
		Additions: fact.Additions, Deletions: fact.Deletions,
		IsMerge: fact.IsMerge(), PRDelivered: fact.PRDelivered, PRNumber: fact.PRNumber,
		FilesJSON:  string(files),
		DiffSample: fact.DiffSample, DebugHits: fact.DebugHits, TodoHits: fact.TodoHits,
	}, nil
}

func commitModel(row Commit) (codecontrib.CommitFact, error) {
	fact := codecontrib.CommitFact{
		SHA: row.SHA, AuthorEmail: row.AuthorEmail, AuthorName: row.AuthorName,
		AuthorLogin: row.AuthorLogin, CommitterEmail: row.CommitterEmail,
		AuthorAt: row.AuthorAt, CommittedAt: row.CommittedAt, Subject: row.Subject,
		Additions: row.Additions, Deletions: row.Deletions,
		PRDelivered: row.PRDelivered, PRNumber: row.PRNumber,
		DiffSample: row.DiffSample, DebugHits: row.DebugHits, TodoHits: row.TodoHits,
	}
	if row.IsMerge {
		fact.ParentSHAs = []string{"p1", "p2"} // merge 标记的持久形态；精确父指针不参与统计
	}
	if err := json.Unmarshal([]byte(row.CoAuthorEmailsJSON), &fact.CoAuthorEmails); err != nil {
		return fact, err
	}
	if err := json.Unmarshal([]byte(row.FilesJSON), &fact.Files); err != nil {
		return fact, err
	}
	return fact, nil
}

func pullRequestRow(workspaceID, repoID uint, fact codecontrib.PullRequestFact) (PullRequest, error) {
	commits, err := json.Marshal(fact.CommitSHAs)
	if err != nil {
		return PullRequest{}, err
	}
	reviews, err := json.Marshal(fact.Reviews)
	if err != nil {
		return PullRequest{}, err
	}
	files, err := json.Marshal(fact.Files)
	if err != nil {
		return PullRequest{}, err
	}
	return PullRequest{
		WorkspaceID: workspaceID, RepoID: repoID, Number: int64(fact.Number),
		Title: fact.Title, AuthorLogin: fact.AuthorLogin, AuthorEmail: fact.AuthorEmail,
		CreatedAt: fact.CreatedAt, MergedAt: fact.MergedAt, MergeSHA: fact.MergeSHA,
		BaseBranch: fact.BaseBranch, Additions: fact.Additions, Deletions: fact.Deletions,
		CommitSHAsJSON: string(commits), ReviewsJSON: string(reviews), FilesJSON: string(files),
	}, nil
}

func pullRequestModel(row PullRequest) (codecontrib.PullRequestFact, error) {
	fact := codecontrib.PullRequestFact{
		Number: int(row.Number), Title: row.Title, AuthorLogin: row.AuthorLogin,
		AuthorEmail: row.AuthorEmail, CreatedAt: row.CreatedAt, MergedAt: row.MergedAt,
		MergeSHA: row.MergeSHA, BaseBranch: row.BaseBranch,
		Additions: row.Additions, Deletions: row.Deletions,
	}
	if err := json.Unmarshal([]byte(row.CommitSHAsJSON), &fact.CommitSHAs); err != nil {
		return fact, err
	}
	if err := json.Unmarshal([]byte(row.ReviewsJSON), &fact.Reviews); err != nil {
		return fact, err
	}
	if err := json.Unmarshal([]byte(row.FilesJSON), &fact.Files); err != nil {
		return fact, err
	}
	return fact, nil
}

func syncModel(row SyncRun) codecontrib.RepoSyncState {
	return codecontrib.RepoSyncState{
		RepoID: row.RepoID, WorkspaceID: row.WorkspaceID,
		LastSyncedAt: row.LastSyncedAt, LastSuccessAt: row.LastSuccessAt,
		LastError: row.LastError, ConsecutiveFailures: row.ConsecutiveFailures,
		FullSyncedAt: row.FullSyncedAt,
	}
}

func syncRow(state codecontrib.RepoSyncState) SyncRun {
	return SyncRun{
		WorkspaceID: state.WorkspaceID, RepoID: state.RepoID,
		LastSyncedAt: state.LastSyncedAt, LastSuccessAt: state.LastSuccessAt,
		LastError: state.LastError, ConsecutiveFailures: state.ConsecutiveFailures,
		FullSyncedAt: state.FullSyncedAt,
		UpdatedAt:    time.Now(),
	}
}
