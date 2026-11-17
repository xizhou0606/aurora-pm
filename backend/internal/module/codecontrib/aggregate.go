package codecontrib

import (
	"sort"
	"time"
)

// Aggregation turns reconciled facts into person×repo×day rows.
//
// 口径（v1，全部确定性、无 AI）：
//   - 行数只来自 PersonLineFiles（merge 与 PR 交付提交不计）与已合入 PR 的
//     请求级 diff；文件按 Class 过滤：Source/Test/Doc/Config 计入，
//     Generated/Vendor/Lockfile/Binary 不计入个人行数。
//   - 提交按提交日归属（工作区时区），合入 PR 按 merged_at 归属，评审按
//     提交时间归属。
//   - 机器人身份完全跳过；未匹配身份进入 user_id=0 的"未归属"桶，绝不
//     并入任何成员。
//   - 共同作者策略 primary：行数归主作者；split 为保留字段，v1 与 primary
//     同义，避免引入分数行数。
type AggregationResult struct {
	Rows        []DailyContributionRow
	TouchedDays []string
}

type aggregationBucketKey struct {
	userID    uint
	authorKey string
	day       string
}

type aggregationBucket struct {
	commits, additions, deletions, prs, reviews int
	// 行数构成与"带评审合入"数：四类合计恒等于 additions。
	sourceAdditions, testAdditions, docAdditions, configAdditions int
	prsReviewed                                                   int
}

// BuildDailyRows aggregates one repository's facts over the collected window.
// resolve must be the deterministic binding pipeline closure; each distinct
// identity is resolved at most once per run.
func BuildDailyRows(repoID uint, commits []CommitFact, requests []PullRequestFact,
	resolve func(AuthorIdentity) Resolution, config CollectionConfig, at time.Time,
) AggregationResult {
	location := aggregationLocation(config.Timezone)
	dayOf := func(t time.Time) string { return t.In(location).Format("2006-01-02") }

	buckets := make(map[aggregationBucketKey]*aggregationBucket)
	order := make([]aggregationBucketKey, 0, 8)
	touched := make(map[string]struct{}, 8)
	bucketFor := func(key aggregationBucketKey, add func(*aggregationBucket)) {
		bucket, exists := buckets[key]
		if !exists {
			bucket = &aggregationBucket{}
			buckets[key] = bucket
			order = append(order, key)
		}
		add(bucket)
	}

	resolutionCache := make(map[AuthorIdentity]Resolution, 8)
	resolveOnce := func(identity AuthorIdentity) Resolution {
		if cached, ok := resolutionCache[identity]; ok {
			return cached
		}
		resolution := resolve(identity)
		resolutionCache[identity] = resolution
		return resolution
	}
	// attribute sends one unit of work into the right bucket; bot identities
	// are dropped entirely, unmatched ones go to the unattributed bucket.
	attribute := func(identity AuthorIdentity, day string, add func(*aggregationBucket)) {
		switch resolution := resolveOnce(identity); resolution.Status {
		case BindingBot:
			return
		case BindingMatched:
			bucketFor(aggregationBucketKey{userID: resolution.UserID, day: day}, add)
		default:
			bucketFor(aggregationBucketKey{authorKey: unattributedKey(identity), day: day}, add)
		}
	}

	for _, commit := range commits {
		if commit.IsMerge() || commit.PRDelivered {
			continue
		}
		day := dayOf(commit.CommittedAt)
		touched[day] = struct{}{}
		additions, deletions := countableLines(commit.Files)
		attribute(commitAuthorIdentity(commit), day, func(b *aggregationBucket) {
			b.commits++
			b.additions += additions
			b.deletions += deletions
			addClassifiedAdditions(commit.Files, b)
		})
	}

	for _, request := range requests {
		if !request.Merged() {
			continue
		}
		mergedDay := dayOf(*request.MergedAt)
		touched[mergedDay] = struct{}{}
		additions, deletions := countableLines(request.Files)
		reviewed := hasApproval(request.Reviews)
		attribute(prAuthorIdentity(request), mergedDay, func(b *aggregationBucket) {
			b.prs++
			if reviewed {
				b.prsReviewed++
			}
			b.additions += additions
			b.deletions += deletions
			addClassifiedAdditions(request.Files, b)
		})
		for _, review := range request.Reviews {
			if review.State != ReviewApproved && review.State != ReviewChangesRequested {
				continue // 评论数量不代表有效评审
			}
			reviewDay := dayOf(review.SubmittedAt)
			touched[reviewDay] = struct{}{}
			attribute(AuthorIdentity{Login: review.ReviewerLogin, Email: review.ReviewerEmail}, reviewDay,
				func(b *aggregationBucket) { b.reviews++ })
		}
	}

	rows := make([]DailyContributionRow, 0, len(order))
	for _, key := range order {
		value := buckets[key]
		rows = append(rows, DailyContributionRow{
			RepoID: repoID, UserID: key.userID, AuthorKey: key.authorKey, Day: key.day,
			Commits: value.commits, Additions: value.additions, Deletions: value.deletions,
			PRsMerged: value.prs, ReviewsCompleted: value.reviews,
			SourceAdditions: value.sourceAdditions, TestAdditions: value.testAdditions,
			DocAdditions: value.docAdditions, ConfigAdditions: value.configAdditions,
			PRsReviewed: value.prsReviewed,
		})
	}
	days := make([]string, 0, len(touched))
	for dayKey := range touched {
		days = append(days, dayKey)
	}
	sort.Strings(days)
	return AggregationResult{Rows: rows, TouchedDays: days}
}

func commitAuthorIdentity(commit CommitFact) AuthorIdentity {
	return AuthorIdentity{Email: commit.AuthorEmail, Name: commit.AuthorName, Login: commit.AuthorLogin}
}

func prAuthorIdentity(request PullRequestFact) AuthorIdentity {
	return AuthorIdentity{Email: request.AuthorEmail, Login: request.AuthorLogin}
}

// unattributedKey keeps unattributed buckets explainable per raw identity.
func unattributedKey(identity AuthorIdentity) string {
	if identity.Email != "" {
		return identity.Email
	}
	return identity.Login
}

func countableLines(files []FileChange) (additions, deletions int) {
	for _, file := range files {
		if file.Binary || !personCountable(file.Class) {
			continue
		}
		additions += file.Additions
		deletions += file.Deletions
	}
	return additions, deletions
}

func personCountable(class FileClass) bool {
	switch class {
	case FileClassSource, FileClassTest, FileClassDoc, FileClassConfig:
		return true
	default:
		return false
	}
}

// addClassifiedAdditions splits one change set's person-countable additions
// into the four work types; their sum always equals the bucket's additions.
func addClassifiedAdditions(files []FileChange, bucket *aggregationBucket) {
	for _, file := range files {
		if file.Binary {
			continue
		}
		switch file.Class {
		case FileClassSource:
			bucket.sourceAdditions += file.Additions
		case FileClassTest:
			bucket.testAdditions += file.Additions
		case FileClassDoc:
			bucket.docAdditions += file.Additions
		case FileClassConfig:
			bucket.configAdditions += file.Additions
		}
	}
}

// hasApproval reports whether one merged request carried at least one
// approval — the review-coverage numerator (评审覆盖).
func hasApproval(reviews []ReviewFact) bool {
	for _, review := range reviews {
		if review.State == ReviewApproved {
			return true
		}
	}
	return false
}

func aggregationLocation(name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	location, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return location
}
