package codecontrib

import "context"

// Collector is the provider port: one implementation per SCM platform turns
// that platform's API into the neutral fact models of this package.
//
// Implementation rules:
//   - Credentials must never be logged, serialized into facts, or embedded in
//     returned error strings (数据安全红线).
//   - ctx cancellation must abort in-flight API calls.
//   - All paging happens behind the cursor so callers cannot accidentally
//     issue unbounded fetch loops.
//
// Business code (binding, aggregation, reporting) only ever sees facts;
// adding a provider never touches downstream code.
type Collector interface {
	// ListCommits pages commits on the repository's target branch whose
	// commit date falls inside the window. Order is unspecified; consumers
	// dedup by SHA. Pass an empty cursor for the first page.
	ListCommits(ctx context.Context, cred Credentials, repo RepoRef,
		window Window, cursor string) (CommitPage, error)

	// ListMergedPullRequests pages requests merged inside the window
	// (provider merge time). Pass an empty cursor for the first page.
	ListMergedPullRequests(ctx context.Context, cred Credentials, repo RepoRef,
		window Window, cursor string) (PRPage, error)

	// VerifyConnection validates the token and returns the account it
	// belongs to; it backs the admin "test connection" flow. Implementations
	// call the provider's current-user endpoint and nothing else.
	VerifyConnection(ctx context.Context, cred Credentials) (Account, error)
}

// CommitPage is one page of commit facts plus an opaque continuation cursor.
// HasMore false means the window is exhausted.
type CommitPage struct {
	Facts      []CommitFact `json:"facts"`
	NextCursor string       `json:"next_cursor,omitempty"`
	HasMore    bool         `json:"has_more"`
}

// PRPage is one page of pull-request facts plus an opaque continuation cursor.
type PRPage struct {
	Facts      []PullRequestFact `json:"facts"`
	NextCursor string            `json:"next_cursor,omitempty"`
	HasMore    bool              `json:"has_more"`
}

// Account is the provider identity a connection's token belongs to, surfaced
// by the admin "test connection" flow before any repository gets bound.
type Account struct {
	Login       string `json:"login"`
	DisplayName string `json:"display_name,omitempty"`
}

// AccessibleRepo is one repository the token can read, offered as a candidate
// in the admin repository binding picker.
type AccessibleRepo struct {
	ExternalID    string `json:"external_id"`
	Path          string `json:"path"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

// RepositoryListing is one page of accessible repositories.
type RepositoryListing struct {
	Repos      []AccessibleRepo `json:"repos"`
	NextCursor string           `json:"next_cursor,omitempty"`
	HasMore    bool             `json:"has_more"`
}

// DirectoryReader lists the repositories a connection's token can access.
// Collector implementations provide it alongside the fact-collection port;
// the sync engine never needs it.
type DirectoryReader interface {
	ListAccessibleRepos(ctx context.Context, cred Credentials, cursor string) (RepositoryListing, error)
}
