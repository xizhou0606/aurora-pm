// Package github turns the GitHub REST API into provider-neutral
// contribution facts. It is the only file in the repository allowed to import
// go-github; everything downstream consumes module/codecontrib models.
package github

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v72/github"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/codecontrib"
)

// Collector implements codecontrib.Collector and codecontrib.DirectoryReader.
type Collector struct {
	base *github.Client
}

// NewCollector builds the shared client shell; authentication is attached per
// call so one collector instance serves every connection.
func NewCollector(baseURL string) *Collector {
	client := github.NewClient(nil)
	if baseURL != "" {
		if _, err := url.Parse(baseURL); err == nil {
			client, _ = client.WithEnterpriseURLs(baseURL, baseURL)
		}
	}
	return &Collector{base: client}
}

func (collector *Collector) client(token string) *github.Client {
	return collector.base.WithAuthToken(token)
}

// ListCommits pages commits on the target branch and enriches each with the
// per-commit stats/files detail call.
func (collector *Collector) ListCommits(ctx context.Context, cred codecontrib.Credentials,
	ref codecontrib.RepoRef, window codecontrib.Window, cursor string,
) (codecontrib.CommitPage, error) {
	owner, repository := splitPath(ref.Path)
	if owner == "" || repository == "" {
		return codecontrib.CommitPage{}, codecontrib.ErrAdminInvalid
	}
	client := collector.client(cred.Token)
	page := cursorPage(cursor)
	options := &github.CommitsListOptions{
		SHA:   ref.DefaultBranch,
		Since: window.Since,
		Until: window.Until,
		ListOptions: github.ListOptions{Page: page, PerPage: 100},
	}
	batch, response, err := client.Repositories.ListCommits(ctx, owner, repository, options)
	if err != nil {
		return codecontrib.CommitPage{}, err
	}
	facts := make([]codecontrib.CommitFact, 0, len(batch))
	for _, item := range batch {
		fact, err := collector.commitFact(ctx, client, owner, repository, item)
		if err != nil {
			return codecontrib.CommitPage{}, err
		}
		fact.Repo = ref
		facts = append(facts, fact)
	}
	return pageResult(facts, response.NextPage), nil
}

func (collector *Collector) commitFact(ctx context.Context, client *github.Client,
	owner, repository string, item *github.RepositoryCommit,
) (codecontrib.CommitFact, error) {
	detail, _, err := client.Repositories.GetCommit(ctx, owner, repository, item.GetSHA(), nil)
	if err != nil {
		return codecontrib.CommitFact{}, err
	}
	fact := codecontrib.CommitFact{
		SHA:        detail.GetSHA(),
		AuthorName: detail.GetCommit().GetAuthor().GetName(),
		AuthorEmail: detail.GetCommit().GetAuthor().GetEmail(),
		AuthorAt:   detail.GetCommit().GetAuthor().GetDate().Time.UTC(),
		CommittedAt: detail.GetCommit().GetCommitter().GetDate().Time.UTC(),
		CommitterEmail: detail.GetCommit().GetCommitter().GetEmail(),
		AuthorLogin: detail.GetAuthor().GetLogin(),
		Subject:     firstLine(detail.GetCommit().GetMessage()),
		Additions:   detail.GetStats().GetAdditions(),
		Deletions:   detail.GetStats().GetDeletions(),
	}
	for _, parent := range detail.Parents {
		fact.ParentSHAs = append(fact.ParentSHAs, parent.GetSHA())
	}
	fact.CoAuthorEmails = codecontrib.ParseCoAuthorTrailers(detail.GetCommit().GetMessage())
	added := make([]string, 0, 64)
	for _, file := range detail.Files {
		fact.Files = append(fact.Files, codecontrib.FileChange{
			Path:      file.GetFilename(),
			Additions: file.GetAdditions(),
			Deletions: file.GetDeletions(),
			Binary:    file.GetPatch() == "",
		})
		// 质量 diff 证据只从普通提交提取；merge 的补丁是分支合入的重复视图。
		if len(detail.Parents) <= 1 {
			added = append(added, codecontrib.AddedLinesFromPatches(file.GetPatch())...)
		}
	}
	fact.DiffSample, fact.DebugHits, fact.TodoHits = codecontrib.SummarizeAddedLines(added)
	return fact, nil
}

// ListMergedPullRequests pages merged requests inside the window and enriches
// each with diff stats, commits, files and reviews.
func (collector *Collector) ListMergedPullRequests(ctx context.Context, cred codecontrib.Credentials,
	ref codecontrib.RepoRef, window codecontrib.Window, cursor string,
) (codecontrib.PRPage, error) {
	owner, repository := splitPath(ref.Path)
	if owner == "" || repository == "" {
		return codecontrib.PRPage{}, codecontrib.ErrAdminInvalid
	}
	client := collector.client(cred.Token)
	options := &github.PullRequestListOptions{
		State: "closed", Sort: "updated", Direction: "desc",
		Base: ref.DefaultBranch,
		ListOptions: github.ListOptions{Page: cursorPage(cursor), PerPage: 100},
	}
	batch, response, err := client.PullRequests.List(ctx, owner, repository, options)
	if err != nil {
		return codecontrib.PRPage{}, err
	}
	facts := make([]codecontrib.PullRequestFact, 0, len(batch))
	for _, item := range batch {
		mergedAt := item.GetMergedAt()
		if item.MergedAt == nil || mergedAt.Before(window.Since) || !mergedAt.Before(window.Until) {
			continue
		}
		fact, err := collector.requestFact(ctx, client, owner, repository, item.GetNumber())
		if err != nil {
			return codecontrib.PRPage{}, err
		}
		fact.Repo = ref
		facts = append(facts, fact)
	}
	// 列表按 updated 倒序：整页都早于窗口起点即可提前结束，减少无效翻页。
	if len(batch) > 0 && batch[len(batch)-1].GetUpdatedAt().Before(window.Since) {
		return codecontrib.PRPage{Facts: facts}, nil
	}
	return pageResultPR(facts, response.NextPage), nil
}

func (collector *Collector) requestFact(ctx context.Context, client *github.Client,
	owner, repository string, number int,
) (codecontrib.PullRequestFact, error) {
	detail, _, err := client.PullRequests.Get(ctx, owner, repository, number)
	if err != nil {
		return codecontrib.PullRequestFact{}, err
	}
	fact := codecontrib.PullRequestFact{
		Number: number, Title: detail.GetTitle(),
		AuthorLogin: detail.GetUser().GetLogin(),
		CreatedAt:   detail.GetCreatedAt().UTC(),
		MergedAt:    mergedTime(detail.MergedAt),
		MergeSHA:    detail.GetMergeCommitSHA(),
		BaseBranch:  detail.GetBase().GetRef(),
		Additions:   detail.GetAdditions(),
		Deletions:   detail.GetDeletions(),
	}
	commitOptions := &github.ListOptions{PerPage: 100}
	for {
		commits, response, err := client.PullRequests.ListCommits(ctx, owner, repository, number, commitOptions)
		if err != nil {
			return fact, err
		}
		for _, commit := range commits {
			fact.CommitSHAs = append(fact.CommitSHAs, commit.GetSHA())
		}
		if response.NextPage == 0 {
			break
		}
		commitOptions.Page = response.NextPage
	}
	reviewOptions := &github.ListOptions{PerPage: 100}
	for {
		reviews, response, err := client.PullRequests.ListReviews(ctx, owner, repository, number, reviewOptions)
		if err != nil {
			return fact, err
		}
		for _, review := range reviews {
			fact.Reviews = append(fact.Reviews, codecontrib.ReviewFact{
				ReviewerLogin: review.GetUser().GetLogin(),
				State:         reviewState(review.GetState()),
				SubmittedAt:   review.GetSubmittedAt().UTC(),
			})
		}
		if response.NextPage == 0 {
			break
		}
		reviewOptions.Page = response.NextPage
	}
	fileOptions := &github.ListOptions{PerPage: 100}
	for {
		files, response, err := client.PullRequests.ListFiles(ctx, owner, repository, number, fileOptions)
		if err != nil {
			return fact, err
		}
		for _, file := range files {
			fact.Files = append(fact.Files, codecontrib.FileChange{
				Path: file.GetFilename(), Additions: file.GetAdditions(),
				Deletions: file.GetDeletions(), Binary: file.GetPatch() == "",
			})
		}
		if response.NextPage == 0 {
			break
		}
		fileOptions.Page = response.NextPage
	}
	return fact, nil
}

// VerifyConnection validates the token with the current-user endpoint.
func (collector *Collector) VerifyConnection(ctx context.Context, cred codecontrib.Credentials) (codecontrib.Account, error) {
	user, _, err := collector.client(cred.Token).Users.Get(ctx, "")
	if err != nil {
		return codecontrib.Account{}, err
	}
	return codecontrib.Account{Login: user.GetLogin(), DisplayName: user.GetName()}, nil
}

// ListAccessibleRepos pages repositories the token can read for the picker.
func (collector *Collector) ListAccessibleRepos(ctx context.Context, cred codecontrib.Credentials, cursor string) (codecontrib.RepositoryListing, error) {
	options := &github.RepositoryListOptions{
		Affiliation: "owner,collaborator,organization_member",
		ListOptions: github.ListOptions{Page: cursorPage(cursor), PerPage: 100},
	}
	repos, response, err := collector.client(cred.Token).Repositories.List(ctx, "", options)
	if err != nil {
		return codecontrib.RepositoryListing{}, err
	}
	listing := codecontrib.RepositoryListing{Repos: make([]codecontrib.AccessibleRepo, 0, len(repos))}
	for _, repository := range repos {
		listing.Repos = append(listing.Repos, codecontrib.AccessibleRepo{
			ExternalID:    strconv.FormatInt(repository.GetID(), 10),
			Path:          repository.GetFullName(),
			DefaultBranch: repository.GetDefaultBranch(),
		})
	}
	if response.NextPage > 0 {
		listing.HasMore = true
		listing.NextCursor = strconv.Itoa(response.NextPage)
	}
	return listing, nil
}

func splitPath(path string) (owner, repository string) {
	parts := strings.SplitN(strings.TrimSpace(path), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", ""
	}
	return parts[0], parts[1]
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

func reviewState(state string) codecontrib.ReviewState {
	switch strings.ToUpper(state) {
	case "APPROVED":
		return codecontrib.ReviewApproved
	case "CHANGES_REQUESTED":
		return codecontrib.ReviewChangesRequested
	default:
		return codecontrib.ReviewCommented
	}
}

func mergedTime(at *github.Timestamp) *time.Time {
	if at == nil {
		return nil
	}
	utc := at.Time.UTC()
	return &utc
}

func cursorPage(cursor string) int {
	page, err := strconv.Atoi(cursor)
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func pageResult(facts []codecontrib.CommitFact, nextPage int) codecontrib.CommitPage {
	page := codecontrib.CommitPage{Facts: facts}
	if nextPage > 0 {
		page.HasMore = true
		page.NextCursor = strconv.Itoa(nextPage)
	}
	return page
}

func pageResultPR(facts []codecontrib.PullRequestFact, nextPage int) codecontrib.PRPage {
	page := codecontrib.PRPage{Facts: facts}
	if nextPage > 0 {
		page.HasMore = true
		page.NextCursor = strconv.Itoa(nextPage)
	}
	return page
}

var _ codecontrib.Collector = (*Collector)(nil)
var _ codecontrib.DirectoryReader = (*Collector)(nil)
