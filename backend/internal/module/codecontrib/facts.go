// Package codecontrib owns the SCM code-contribution domain: provider-neutral
// fact collection, deterministic committer-to-user binding, and per-day
// aggregation feeding the report module.
//
// Layering contract: this package holds the fact models and the pure logic
// (path classification, co-author trailers, PR reconciliation, the binding
// pipeline). Collector implementations turn GitHub/GitLab APIs into these
// neutral facts; persistence, scheduling and HTTP surfaces live in the gorm,
// worker and http subpackages so the pure core stays testable without
// credentials, databases or network access.
//
// 统计口径的红线：提交人身份字段是不可变的原始事实，绑定到 PM 用户只发生在
// 解析管线里；任何环节都不允许改写事实记录本身。
package codecontrib

import (
	"strings"
	"time"
)

// Provider identifies a supported source-control platform. Adding one is a
// new collector implementation plus a port definition — downstream code never
// branches on the value.
type Provider string

const (
	ProviderGitHub Provider = "github"
	ProviderGitLab Provider = "gitlab"
)

// RepoRef identifies one repository in a provider-neutral way. ExternalID is
// the provider's stable numeric identity (immune to renames); Path is the
// human-readable "owner/repo" (GitHub) or "group/subgroup/project" (GitLab)
// used for display and admin configuration.
type RepoRef struct {
	Provider      Provider `json:"provider"`
	ExternalID    string   `json:"external_id"`
	Path          string   `json:"path"`
	DefaultBranch string   `json:"default_branch,omitempty"`
}

// Window is a half-open time range [Since, Until) in UTC. Collectors fetch
// facts whose authoritative timestamp (commit date, merge date) falls inside;
// period boundary and timezone handling happens at aggregation, not here.
type Window struct {
	Since time.Time
	Until time.Time
}

// Credentials carries one connection's read-only access token. Values only
// flow from the connection store into collectors: they must never be logged,
// serialized into facts, or embedded in returned error strings.
type Credentials struct {
	Token   string
	BaseURL string // empty means the provider's public default instance
}

// CommitFact is one commit's normalized facts. The identity fields are raw
// provider facts (commit author/committer, plus account login when the API
// exposes one); PRDelivered/PRNumber are the only fields later machinery is
// allowed to set, via MarkPRDelivered.
type CommitFact struct {
	Provider       Provider     `json:"provider"`
	Repo           RepoRef      `json:"repo"`
	SHA            string       `json:"sha"`
	ParentSHAs     []string     `json:"parent_shas,omitempty"`
	AuthorName     string       `json:"author_name,omitempty"`
	AuthorEmail    string       `json:"author_email,omitempty"`
	AuthorLogin    string       `json:"author_login,omitempty"`
	CommitterEmail string       `json:"committer_email,omitempty"`
	AuthorAt       time.Time    `json:"author_at"`
	CommittedAt    time.Time    `json:"committed_at"`
	CoAuthorEmails []string     `json:"co_author_emails,omitempty"`
	Subject        string       `json:"subject,omitempty"`
	Additions      int          `json:"additions"`
	Deletions      int          `json:"deletions"`
	Files          []FileChange `json:"files,omitempty"`

	// 质量证据（采集时从 diff 新增行提取）：DiffSample 是新增行内容截断
	// 样本；DebugHits/TodoHits 命中 console.log/debugger 与 TODO/FIXME。
	DiffSample string `json:"diff_sample,omitempty"`
	DebugHits  int    `json:"debug_hits,omitempty"`
	TodoHits   int    `json:"todo_hits,omitempty"`

	// PRDelivered marks a commit whose diff reached the target branch through
	// a merged pull request. Its line stats are carried by that PR exactly
	// once, so person-level line totals must skip it.
	PRDelivered bool `json:"pr_delivered,omitempty"`
	PRNumber    int  `json:"pr_number,omitempty"`
}

// IsMerge reports whether the commit is a merge commit. Merge commits carry
// no diff of their own; line stats come from the merged content instead.
func (commit CommitFact) IsMerge() bool { return len(commit.ParentSHAs) > 1 }

// PersonLineFiles returns the files that count toward person line totals:
// merge commits contribute nothing, and PR-delivered commits are accounted at
// the PR level (PR 与 commit 增删行不能相加).
func (commit CommitFact) PersonLineFiles() []FileChange {
	if commit.IsMerge() || commit.PRDelivered {
		return nil
	}
	return commit.Files
}

// ReviewState is the normalized review outcome. GitLab approvals and GitHub
// review states both collapse into these three values so aggregation never
// needs provider-specific branches.
type ReviewState string

const (
	ReviewApproved         ReviewState = "approved"
	ReviewChangesRequested ReviewState = "changes_requested"
	ReviewCommented        ReviewState = "commented"
)

// ReviewFact is one completed review action on a pull request. Review counts
// feed collaboration facts only; comment volume alone never implies quality.
type ReviewFact struct {
	ReviewerLogin string      `json:"reviewer_login,omitempty"`
	ReviewerEmail string      `json:"reviewer_email,omitempty"`
	State         ReviewState `json:"state"`
	SubmittedAt   time.Time   `json:"submitted_at"`
}

// PullRequestFact is one provider pull/merge request. MergedAt nil means the
// request has not landed on the target branch. Additions/Deletions are the
// request-level diff stats — the single attribution basis for its line counts.
type PullRequestFact struct {
	Provider    Provider     `json:"provider"`
	Repo        RepoRef      `json:"repo"`
	Number      int          `json:"number"`
	Title       string       `json:"title,omitempty"`
	AuthorLogin string       `json:"author_login,omitempty"`
	AuthorEmail string       `json:"author_email,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	MergedAt    *time.Time   `json:"merged_at,omitempty"`
	MergeSHA    string       `json:"merge_sha,omitempty"`
	BaseBranch  string       `json:"base_branch,omitempty"`
	Additions   int          `json:"additions"`
	Deletions   int          `json:"deletions"`
	Files       []FileChange `json:"files,omitempty"`
	CommitSHAs  []string     `json:"commit_shas,omitempty"`
	Reviews     []ReviewFact `json:"reviews,omitempty"`
}

// Merged reports whether the request landed on the target branch.
func (request PullRequestFact) Merged() bool { return request.MergedAt != nil }

// MarkPRDelivered reconciles commits against merged pull requests so line
// stats attribute at the PR level exactly once. A commit is flagged when it
// belongs to a merged request — either listed in its CommitSHAs, or being the
// squash/merge commit itself (MergeSHA). Direct pushes stay unflagged and keep
// their own line stats. Reconciliation is idempotent: overlap windows replay
// the same data without changing the outcome.
//
// Rebase-merged requests expose rewritten SHAs per provider; collectors must
// fill CommitSHAs from the request API, not from local git lore, so this
// function stays provider-agnostic.
func MarkPRDelivered(commits []CommitFact, requests []PullRequestFact) {
	bySHA := make(map[string]int, len(commits))
	for index := range commits {
		bySHA[commits[index].SHA] = index
	}
	for _, request := range requests {
		if !request.Merged() {
			continue
		}
		shas := append([]string{request.MergeSHA}, request.CommitSHAs...)
		for _, sha := range shas {
			if sha == "" {
				continue
			}
			if index, ok := bySHA[sha]; ok {
				commits[index].PRDelivered = true
				commits[index].PRNumber = request.Number
			}
		}
	}
}

// ParseCoAuthorTrailers extracts co-author emails from `Co-authored-by:`
// trailers. Commit messages are untrusted material: only the structural
// trailer is read and the surrounding text is never interpreted. Entries
// without a parseable email are dropped; emails are lowercased so the binding
// pipeline sees one canonical form per author.
func ParseCoAuthorTrailers(message string) []string {
	emails := make([]string, 0, 2)
	for _, line := range strings.Split(message, "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), ":")
		if !found || !strings.EqualFold(strings.TrimSpace(key), "co-authored-by") {
			continue
		}
		if email := extractEmail(value); email != "" {
			emails = append(emails, email)
		}
	}
	return emails
}

// extractEmail pulls the address out of "Name <email>" shapes, falling back to
// a bare single-token address. Anything else is rejected.
func extractEmail(raw string) string {
	value := strings.TrimSpace(raw)
	if open := strings.LastIndex(value, "<"); open >= 0 {
		if close := strings.LastIndex(value, ">"); close > open {
			value = value[open+1 : close]
		}
	}
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, " \t") || !strings.Contains(value, "@") {
		return ""
	}
	return strings.ToLower(value)
}
