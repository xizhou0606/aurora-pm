package github

import (
	"strings"
	"unicode/utf8"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/codecontrib"
)

const commitDetail = `{
  "sha": "s1",
  "parents": [{"sha": "p1"}, {"sha": "p2"}],
  "commit": {
    "message": "feat: reporting page\n\nCo-authored-by: Bob <Bob@Example.com>\nSigned-off-by: Alice <alice@example.com>",
    "author": {"name": "Alice", "email": "alice@example.com", "date": "2026-09-01T10:00:00Z"},
    "committer": {"email": "ci@example.com", "date": "2026-09-01T10:05:00Z"}
  },
  "stats": {"additions": 10, "deletions": 2},
  "files": [
    {"filename": "internal/main.go", "additions": 8, "deletions": 2},
    {"filename": "assets/logo.png"}
  ]
}`

func newTestCollector(t *testing.T, handler http.Handler) *Collector {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewCollector(server.URL)
}

func TestListCommitsNormalizesProviderPayload(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/ops/pm/commits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"sha": "s1"}]`))
	})
	mux.HandleFunc("/api/v3/repos/ops/pm/commits/s1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(commitDetail))
	})
	collector := newTestCollector(t, mux)

	window := codecontrib.Window{
		Since: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC),
	}
	page, err := collector.ListCommits(context.Background(), codecontrib.Credentials{Token: "t"},
		codecontrib.RepoRef{Provider: codecontrib.ProviderGitHub, Path: "ops/pm", DefaultBranch: "main"},
		window, "")
	if err != nil {
		t.Fatalf("list commits: %v", err)
	}
	if len(page.Facts) != 1 {
		t.Fatalf("facts = %d, want 1", len(page.Facts))
	}
	fact := page.Facts[0]
	if fact.SHA != "s1" || fact.Additions != 10 || fact.Deletions != 2 {
		t.Fatalf("commit stats wrong: %+v", fact)
	}
	if !fact.IsMerge() {
		t.Fatal("two parents must read as merge commit")
	}
	if len(fact.CoAuthorEmails) != 1 || fact.CoAuthorEmails[0] != "bob@example.com" {
		t.Fatalf("co-author trailers wrong: %v", fact.CoAuthorEmails)
	}
	if len(fact.Files) != 2 || !fact.Files[1].Binary || fact.Files[1].Additions != 0 {
		t.Fatalf("file facts wrong: %+v", fact.Files)
	}
	if fact.Subject != "feat: reporting page" || fact.AuthorLogin != "" {
		t.Fatalf("subject/author wrong: %q", fact.Subject)
	}
}

func TestListMergedPullRequestsNormalizesProviderPayload(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/ops/pm/pulls", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != "closed" {
			t.Fatalf("expected closed state, got %q", r.URL.Query().Get("state"))
		}
		_, _ = w.Write([]byte(`[{"number": 7, "merged_at": "2026-09-02T08:00:00Z", "updated_at": "2026-09-03T08:00:00Z"}]`))
	})
	mux.HandleFunc("/api/v3/repos/ops/pm/pulls/7", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"number": 7, "title": "T", "additions": 5, "deletions": 1,
			"merge_commit_sha": "m1", "base": {"ref": "main"}, "user": {"login": "alice"},
			"created_at": "2026-09-01T09:00:00Z", "merged_at": "2026-09-02T08:00:00Z"}`))
	})
	mux.HandleFunc("/api/v3/repos/ops/pm/pulls/7/commits", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"sha": "s1"}, {"sha": "s2"}]`))
	})
	mux.HandleFunc("/api/v3/repos/ops/pm/pulls/7/reviews", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"user": {"login": "bob"}, "state": "APPROVED", "submitted_at": "2026-09-01T12:00:00Z"}]`))
	})
	mux.HandleFunc("/api/v3/repos/ops/pm/pulls/7/files", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"filename": "a.go", "additions": 5, "deletions": 1}]`))
	})
	collector := newTestCollector(t, mux)

	window := codecontrib.Window{
		Since: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC),
	}
	page, err := collector.ListMergedPullRequests(context.Background(), codecontrib.Credentials{Token: "t"},
		codecontrib.RepoRef{Provider: codecontrib.ProviderGitHub, Path: "ops/pm", DefaultBranch: "main"},
		window, "")
	if err != nil {
		t.Fatalf("list prs: %v", err)
	}
	if len(page.Facts) != 1 {
		t.Fatalf("facts = %d, want 1", len(page.Facts))
	}
	fact := page.Facts[0]
	if fact.Number != 7 || !fact.Merged() || fact.MergeSHA != "m1" || fact.BaseBranch != "main" {
		t.Fatalf("request facts wrong: %+v", fact)
	}
	if fact.AuthorLogin != "alice" || fact.Additions != 5 || fact.Deletions != 1 {
		t.Fatalf("author/stats wrong: %+v", fact)
	}
	if len(fact.CommitSHAs) != 2 || fact.CommitSHAs[1] != "s2" {
		t.Fatalf("commit shas wrong: %v", fact.CommitSHAs)
	}
	if len(fact.Reviews) != 1 || fact.Reviews[0].State != codecontrib.ReviewApproved ||
		fact.Reviews[0].ReviewerLogin != "bob" {
		t.Fatalf("reviews wrong: %+v", fact.Reviews)
	}
}

func TestVerifyConnectionAndRepoListing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer t0ken" {
			t.Fatalf("token must ride the Authorization header, got %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"login": "pm-bot", "name": "PM Bot"}`))
	})
	mux.HandleFunc("/api/v3/user/repos", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id": 99, "full_name": "ops/pm", "default_branch": "main"}]`))
	})
	collector := newTestCollector(t, mux)

	account, err := collector.VerifyConnection(context.Background(), codecontrib.Credentials{Token: "t0ken"})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if account.Login != "pm-bot" || account.DisplayName != "PM Bot" {
		t.Fatalf("account wrong: %+v", account)
	}
	listing, err := collector.ListAccessibleRepos(context.Background(), codecontrib.Credentials{Token: "t0ken"}, "")
	if err != nil {
		t.Fatalf("list repos: %v", err)
	}
	if len(listing.Repos) != 1 || listing.Repos[0].ExternalID != "99" || listing.Repos[0].Path != "ops/pm" {
		t.Fatalf("listing wrong: %+v", listing.Repos)
	}
}

// 提交标题按字节截断时不得把多字节字符切成非法 UTF-8，否则 Postgres 拒绝
// 写入整批采集数据（线上故障：invalid byte sequence for encoding "UTF8"）。
func TestFirstLineTruncatesOnUTF8Boundary(t *testing.T) {
	subject := strings.Repeat("x", 499) + "好"
	got := firstLine(subject)
	if !utf8.ValidString(got) {
		t.Fatalf("firstLine produced invalid UTF-8: %q", got)
	}
	if got != strings.Repeat("x", 499) {
		t.Fatalf("unexpected firstLine result: %q", got)
	}
}
