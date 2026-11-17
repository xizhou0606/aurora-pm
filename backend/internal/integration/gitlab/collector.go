// Package gitlab turns the GitLab REST API into provider-neutral
// contribution facts. It is the only file in the repository allowed to import
// the official GitLab client; everything downstream consumes
// module/codecontrib models. 自建实例通过 BaseURL 支持。
//
// client-go 的方法不接收 context（使用客户端内部上下文），因此本包只在
// 对外接口上保留 ctx 以满足 codecontrib.Collector 契约。
package gitlab

import (
	"context"
	"strconv"
	"strings"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/codecontrib"
)

// Collector implements codecontrib.Collector and codecontrib.DirectoryReader.
type Collector struct{}

func NewCollector() *Collector { return &Collector{} }

func (collector *Collector) client(cred codecontrib.Credentials) (*gitlab.Client, error) {
	if cred.BaseURL == "" {
		return gitlab.NewClient(cred.Token)
	}
	return gitlab.NewClient(cred.Token, gitlab.WithBaseURL(cred.BaseURL))
}

// ListCommits pages commits across ALL refs（all=true，不限分支）,
// enriching each with the detail (stats) and diff (per-file counts)
// endpoints. GitLab only exposes per-file numbers as unified-diff patches,
// so counts are parsed from them. 全量轮的 window.Since 为零值，此时不带
// since 参数拉取全部历史。
func (collector *Collector) ListCommits(ctx context.Context, cred codecontrib.Credentials,
	ref codecontrib.RepoRef, window codecontrib.Window, cursor string,
) (codecontrib.CommitPage, error) {
	client, err := collector.client(cred)
	if err != nil {
		return codecontrib.CommitPage{}, err
	}
	project := projectRef(ref)
	since := window.Since
	until := window.Until
	options := &gitlab.ListCommitsOptions{
		All:         gitlab.Ptr(true),
		Until:       gitlab.Ptr(until),
		ListOptions: gitlab.ListOptions{Page: int64(cursorPage(cursor)), PerPage: 100},
	}
	if !since.IsZero() {
		options.Since = gitlab.Ptr(since)
	}
	batch, response, err := client.Commits.ListCommits(project, options)
	if err != nil {
		return codecontrib.CommitPage{}, err
	}
	facts := make([]codecontrib.CommitFact, 0, len(batch))
	for _, item := range batch {
		fact, err := collector.commitFact(client, project, item.ID)
		if err != nil {
			return codecontrib.CommitPage{}, err
		}
		fact.Repo = ref
		facts = append(facts, fact)
	}
	return commitPageResult(facts, response.NextPage), nil
}

func (collector *Collector) commitFact(client *gitlab.Client, project, sha string) (codecontrib.CommitFact, error) {
	detail, _, err := client.Commits.GetCommit(project, sha, nil)
	if err != nil {
		return codecontrib.CommitFact{}, err
	}
	createdAt := time.Time{}
	if detail.CreatedAt != nil {
		createdAt = detail.CreatedAt.UTC()
	}
	fact := codecontrib.CommitFact{
		SHA: detail.ID, AuthorName: detail.AuthorName, AuthorEmail: detail.AuthorEmail,
		AuthorAt: createdAt, CommittedAt: createdAt,
		CommitterEmail: detail.CommitterEmail, Subject: firstLine(detail.Title),
	}
	if detail.Stats != nil {
		fact.Additions = int(detail.Stats.Additions)
		fact.Deletions = int(detail.Stats.Deletions)
	}
	if len(detail.ParentIDs) > 1 {
		fact.ParentSHAs = append(fact.ParentSHAs, detail.ParentIDs...)
	}
	fact.CoAuthorEmails = codecontrib.ParseCoAuthorTrailers(detail.Message)
	diffs, _, err := client.Commits.GetCommitDiff(project, sha, nil)
	if err != nil {
		return fact, err
	}
	fact.Files = filesFromDiffs(diffs)
	if len(detail.ParentIDs) <= 1 {
		// 质量 diff 证据只从普通提交提取；merge 的补丁是分支合入的重复视图。
		added := make([]string, 0, 64)
		for _, diff := range diffs {
			added = append(added, codecontrib.AddedLinesFromPatches(diff.Diff)...)
		}
		fact.DiffSample, fact.DebugHits, fact.TodoHits = codecontrib.SummarizeAddedLines(added)
	}
	return fact, nil
}

// ListMergedPullRequests pages merged requests (state=merged) inside the
// window, enriching each with diffs, commits and approvals.
func (collector *Collector) ListMergedPullRequests(ctx context.Context, cred codecontrib.Credentials,
	ref codecontrib.RepoRef, window codecontrib.Window, cursor string,
) (codecontrib.PRPage, error) {
	client, err := collector.client(cred)
	if err != nil {
		return codecontrib.PRPage{}, err
	}
	project := projectRef(ref)
	batch, response, err := client.MergeRequests.ListProjectMergeRequests(project,
		&gitlab.ListProjectMergeRequestsOptions{
			State:        gitlab.Ptr("merged"),
			TargetBranch: gitlab.Ptr(ref.DefaultBranch),
			OrderBy:      gitlab.Ptr("updated_at"),
			Sort:         gitlab.Ptr("desc"),
			ListOptions:  gitlab.ListOptions{Page: int64(cursorPage(cursor)), PerPage: 100},
		})
	if err != nil {
		return codecontrib.PRPage{}, err
	}
	facts := make([]codecontrib.PullRequestFact, 0, len(batch))
	for _, item := range batch {
		if item.MergedAt == nil ||
			item.MergedAt.Before(window.Since) || !item.MergedAt.Before(window.Until) {
			continue
		}
		fact, err := collector.requestFact(client, project, item.IID)
		if err != nil {
			return codecontrib.PRPage{}, err
		}
		fact.Repo = ref
		facts = append(facts, fact)
	}
	if len(batch) > 0 && batch[len(batch)-1].UpdatedAt.Before(window.Since) {
		return codecontrib.PRPage{Facts: facts}, nil
	}
	return prPageResult(facts, response.NextPage), nil
}

func (collector *Collector) requestFact(client *gitlab.Client, project string, iid int64) (codecontrib.PullRequestFact, error) {
	detail, _, err := client.MergeRequests.GetMergeRequest(project, iid, nil)
	if err != nil {
		return codecontrib.PullRequestFact{}, err
	}
	fact := codecontrib.PullRequestFact{
		Number: int(iid), Title: detail.Title, AuthorLogin: detail.Author.Username,
		CreatedAt: detail.CreatedAt.UTC(), MergeSHA: detail.MergeCommitSHA,
		BaseBranch: detail.TargetBranch,
	}
	if detail.MergedAt != nil {
		merged := detail.MergedAt.UTC()
		fact.MergedAt = &merged
	}
	diffs, _, err := client.MergeRequests.ListMergeRequestDiffs(project, iid, nil)
	if err != nil {
		return fact, err
	}
	fact.Files = filesFromMergeRequestDiffs(diffs)
	for _, file := range fact.Files {
		fact.Additions += file.Additions
		fact.Deletions += file.Deletions
	}
	commits, _, err := client.MergeRequests.GetMergeRequestCommits(project, iid, nil)
	if err != nil {
		return fact, err
	}
	for _, commit := range commits {
		fact.CommitSHAs = append(fact.CommitSHAs, commit.ID)
	}
	approvals, _, err := client.MergeRequests.GetMergeRequestApprovals(project, iid)
	if err != nil {
		return fact, err
	}
	for _, approval := range approvals.ApprovedBy {
		if approval.User == nil {
			continue
		}
		submitted := fact.CreatedAt
		if fact.MergedAt != nil {
			submitted = *fact.MergedAt
		}
		fact.Reviews = append(fact.Reviews, codecontrib.ReviewFact{
			ReviewerLogin: approval.User.Username,
			State:         codecontrib.ReviewApproved,
			SubmittedAt:   submitted,
		})
	}
	return fact, nil
}

// VerifyConnection validates the token with the current-user endpoint.
func (collector *Collector) VerifyConnection(ctx context.Context, cred codecontrib.Credentials) (codecontrib.Account, error) {
	client, err := collector.client(cred)
	if err != nil {
		return codecontrib.Account{}, err
	}
	user, _, err := client.Users.CurrentUser()
	if err != nil {
		return codecontrib.Account{}, err
	}
	return codecontrib.Account{Login: user.Username, DisplayName: user.Name}, nil
}

// ListAccessibleRepos pages projects the token is a member of.
func (collector *Collector) ListAccessibleRepos(ctx context.Context, cred codecontrib.Credentials, cursor string) (codecontrib.RepositoryListing, error) {
	client, err := collector.client(cred)
	if err != nil {
		return codecontrib.RepositoryListing{}, err
	}
	projects, response, err := client.Projects.ListProjects(&gitlab.ListProjectsOptions{
		Membership: gitlab.Ptr(true),
		Simple:     gitlab.Ptr(true),
		ListOptions: gitlab.ListOptions{Page: int64(cursorPage(cursor)), PerPage: 100},
	})
	if err != nil {
		return codecontrib.RepositoryListing{}, err
	}
	listing := codecontrib.RepositoryListing{Repos: make([]codecontrib.AccessibleRepo, 0, len(projects))}
	for _, project := range projects {
		listing.Repos = append(listing.Repos, codecontrib.AccessibleRepo{
			ExternalID:    strconv.FormatInt(project.ID, 10),
			Path:          project.PathWithNamespace,
			DefaultBranch: project.DefaultBranch,
		})
	}
	if response.NextPage > 0 {
		listing.HasMore = true
		listing.NextCursor = strconv.FormatInt(response.NextPage, 10)
	}
	return listing, nil
}

func projectRef(ref codecontrib.RepoRef) string {
	if ref.ExternalID != "" {
		return ref.ExternalID
	}
	return ref.Path
}

// filesFromMergeRequestDiffs converts merge-request diff entries into
// per-file counts with the same semantics as commit diffs.
func filesFromMergeRequestDiffs(diffs []*gitlab.MergeRequestDiff) []codecontrib.FileChange {
	files := make([]codecontrib.FileChange, 0, len(diffs))
	for _, diff := range diffs {
		change := codecontrib.FileChange{Path: diff.NewPath}
		if change.Path == "" {
			change.Path = diff.OldPath
		}
		if diff.Diff == "" {
			change.Binary = true
		} else {
			change.Additions, change.Deletions = countDiffLines(diff.Diff)
		}
		files = append(files, change)
	}
	return files
}

// filesFromDiffs converts unified-diff patches into per-file counts. A diff
// without patch text is treated as binary (GitLab reports binary blobs that
// way); "+++"/"---" headers never count.
func filesFromDiffs(diffs []*gitlab.Diff) []codecontrib.FileChange {
	files := make([]codecontrib.FileChange, 0, len(diffs))
	for _, diff := range diffs {
		change := codecontrib.FileChange{Path: diff.NewPath}
		if change.Path == "" {
			change.Path = diff.OldPath
		}
		if diff.Diff == "" {
			change.Binary = true
		} else {
			change.Additions, change.Deletions = countDiffLines(diff.Diff)
		}
		files = append(files, change)
	}
	return files
}

func countDiffLines(patch string) (additions, deletions int) {
	for _, line := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			continue
		case strings.HasPrefix(line, "+"):
			additions++
		case strings.HasPrefix(line, "-"):
			deletions++
		}
	}
	return additions, deletions
}

func firstLine(message string) string {
	message = strings.TrimSpace(message)
	if index := strings.IndexByte(message, '\n'); index >= 0 {
		message = message[:index]
	}
	if len(message) > 500 {
		message = codecontrib.TruncateUTF8(message, 500)
	}
	return message
}

func cursorPage(cursor string) int {
	page, err := strconv.Atoi(cursor)
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func commitPageResult(facts []codecontrib.CommitFact, nextPage int64) codecontrib.CommitPage {
	page := codecontrib.CommitPage{Facts: facts}
	if nextPage > 0 {
		page.HasMore = true
		page.NextCursor = strconv.FormatInt(nextPage, 10)
	}
	return page
}

func prPageResult(facts []codecontrib.PullRequestFact, nextPage int64) codecontrib.PRPage {
	page := codecontrib.PRPage{Facts: facts}
	if nextPage > 0 {
		page.HasMore = true
		page.NextCursor = strconv.FormatInt(nextPage, 10)
	}
	return page
}

var _ codecontrib.Collector = (*Collector)(nil)
var _ codecontrib.DirectoryReader = (*Collector)(nil)
