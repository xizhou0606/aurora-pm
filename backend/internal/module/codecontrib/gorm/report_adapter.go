package codecontribgorm

import (
	"context"
	"time"

	report "github.com/xizhou0606/aurora-pm/backend/internal/module/report"
)

// ReportReadAdapter adapts the fact store to the report module's read port,
// keeping report imports one-directional (gorm → module types).
type ReportReadAdapter struct {
	facts *FactsRepository
}

func NewReportReadAdapter(facts *FactsRepository) *ReportReadAdapter {
	return &ReportReadAdapter{facts: facts}
}

func (adapter *ReportReadAdapter) ListDailyContributions(
	ctx context.Context, workspaceID uint,
) ([]report.DailyContributionFact, error) {
	rows, err := adapter.facts.ListDailyContributions(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	result := make([]report.DailyContributionFact, 0, len(rows))
	for _, row := range rows {
		result = append(result, report.DailyContributionFact{
			RepoID: row.RepoID, UserID: row.UserID, AuthorKey: row.AuthorKey,
			Day: row.Day, Commits: row.Commits, Additions: row.Additions,
			Deletions: row.Deletions, PRsMerged: row.PRsMerged,
			ReviewsCompleted: row.ReviewsCompleted,
			SourceAdditions:  row.SourceAdditions, TestAdditions: row.TestAdditions,
			DocAdditions: row.DocAdditions, ConfigAdditions: row.ConfigAdditions,
			PRsReviewed: row.PRsReviewed,
		})
	}
	return result, nil
}

// ListMemberCommits projects the window's member-attributed commits (with
// their diff samples and quality hits) onto the report module's fact type.
func (adapter *ReportReadAdapter) ListMemberCommits(
	ctx context.Context, workspaceID uint, since, until time.Time,
) ([]report.MemberCommitFact, error) {
	rows, err := adapter.facts.ListMemberCommits(ctx, workspaceID, since, until)
	if err != nil {
		return nil, err
	}
	result := make([]report.MemberCommitFact, 0, len(rows))
	for _, row := range rows {
		result = append(result, report.MemberCommitFact{
			UserID: row.UserID, SHA: row.SHA, CommittedAt: row.CommittedAt,
			Subject: row.Subject, Additions: row.Additions, Deletions: row.Deletions,
			Files: row.Files, DebugHits: row.DebugHits, TodoHits: row.TodoHits,
			DiffSample: row.DiffSample,
		})
	}
	return result, nil
}

func (adapter *ReportReadAdapter) ListRepoSyncStates(
	ctx context.Context, workspaceID uint,
) ([]report.RepoSyncStatus, error) {
	states, err := adapter.facts.ListRepoSyncStates(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	result := make([]report.RepoSyncStatus, 0, len(states))
	for _, state := range states {
		result = append(result, report.RepoSyncStatus{
			RepoID: state.RepoID, LastSuccessAt: state.LastSuccessAt,
			LastError: state.LastError, ConsecutiveFailures: state.ConsecutiveFailures,
		})
	}
	return result, nil
}

var _ report.ContributionFactsQueries = (*ReportReadAdapter)(nil)
