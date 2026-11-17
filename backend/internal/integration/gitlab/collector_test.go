package gitlab

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

func newTestCollector(t *testing.T, handler http.Handler) *Collector {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewCollector()
}

func credentials(server *httptest.Server) codecontrib.Credentials {
	return codecontrib.Credentials{Token: "glpat-test", BaseURL: server.URL}
}

func ref() codecontrib.RepoRef {
	return codecontrib.RepoRef{
		Provider: codecontrib.ProviderGitLab, ExternalID: "42",
		Path: "ops/pm", DefaultBranch: "main",
	}
}

func window() codecontrib.Window {
	return codecontrib.Window{
		Since: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC),
	}
}

func TestListCommitsNormalizesProviderPayload(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/repository/commits", func(w http.ResponseWriter, r *http.Request) {
		// 采集跨全部 refs（all=true，不限分支），不再按目标分支过滤。
		if r.URL.Query().Get("all") != "true" {
			t.Fatalf("all = %q", r.URL.Query().Get("all"))
		}
		if r.URL.Query().Get("ref_name") != "" {
			t.Fatalf("ref_name = %q, want unset", r.URL.Query().Get("ref_name"))
		}
		_, _ = w.Write([]byte(`[{"id": "s1", "title": "feat: reporting page"}]`))
	})
	mux.HandleFunc("/api/v4/projects/42/repository/commits/s1", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"id": "s1",
			"title": "feat: reporting page",
			"message": "feat: reporting page\n\nCo-authored-by: Bob <Bob@Example.com>",
			"author_name": "Alice", "author_email": "alice@example.com",
			"created_at": "2026-09-01T10:00:00Z",
			"stats": {"additions": 10, "deletions": 2},
			"parent_ids": ["p1", "p2"]
		}`))
	})
	mux.HandleFunc("/api/v4/projects/42/repository/commits/s1/diff", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"new_path": "internal/main.go", "diff": "@@ -1,2 +1,3 @@\n context\n+added\n-removed\n"},
			{"new_path": "assets/logo.png", "diff": ""}
		]`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	page, err := newTestCollector(t, mux).ListCommits(context.Background(), credentials(server), ref(), window(), "")
	if err != nil {
		t.Fatalf("list commits: %v", err)
	}
	if len(page.Facts) != 1 {
		t.Fatalf("facts = %d, want 1", len(page.Facts))
	}
	fact := page.Facts[0]
	if fact.SHA != "s1" || fact.Additions != 10 || fact.Deletions != 2 || !fact.IsMerge() {
		t.Fatalf("commit facts wrong: %+v", fact)
	}
	if len(fact.CoAuthorEmails) != 1 || fact.CoAuthorEmails[0] != "bob@example.com" {
		t.Fatalf("co-authors wrong: %v", fact.CoAuthorEmails)
	}
	// patch 计数：1 行新增 + 1 行删除；空 patch 视为二进制。
	if fact.Files[0].Additions != 1 || fact.Files[0].Deletions != 1 {
		t.Fatalf("patch counts wrong: %+v", fact.Files[0])
	}
	if !fact.Files[1].Binary {
		t.Fatalf("empty patch must read as binary: %+v", fact.Files[1])
	}
}

func TestListMergedPullRequestsNormalizesProviderPayload(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/merge_requests", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != "merged" {
			t.Fatalf("state = %q", r.URL.Query().Get("state"))
		}
		_, _ = w.Write([]byte(`[{"iid": 7, "merged_at": "2026-09-02T08:00:00Z", "updated_at": "2026-09-03T08:00:00Z"}]`))
	})
	mux.HandleFunc("/api/v4/projects/42/merge_requests/7", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"iid": 7, "title": "T", "author": {"username": "alice"},
			"created_at": "2026-09-01T09:00:00Z", "merged_at": "2026-09-02T08:00:00Z",
			"merge_commit_sha": "m1", "target_branch": "main"}`))
	})
	mux.HandleFunc("/api/v4/projects/42/merge_requests/7/diffs", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"new_path": "a.go", "diff": "@@\n+x\n-y\n"}]`))
	})
	mux.HandleFunc("/api/v4/projects/42/merge_requests/7/commits", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id": "s1"}]`))
	})
	mux.HandleFunc("/api/v4/projects/42/merge_requests/7/approvals", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"approved_by": [{"user": {"username": "bob", "name": "Bob"}}]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	page, err := newTestCollector(t, mux).ListMergedPullRequests(context.Background(), credentials(server), ref(), window(), "")
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
	if fact.AuthorLogin != "alice" || fact.Additions != 1 || fact.Deletions != 1 {
		t.Fatalf("author/stats wrong: %+v", fact)
	}
	if len(fact.CommitSHAs) != 1 || fact.CommitSHAs[0] != "s1" {
		t.Fatalf("commit shas wrong: %v", fact.CommitSHAs)
	}
	if len(fact.Reviews) != 1 || fact.Reviews[0].State != codecontrib.ReviewApproved ||
		fact.Reviews[0].ReviewerLogin != "bob" {
		t.Fatalf("reviews wrong: %+v", fact.Reviews)
	}
}

func TestVerifyConnectionAndRepoListing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") != "glpat-test" {
			t.Fatalf("token must ride PRIVATE-TOKEN, got %q", r.Header.Get("PRIVATE-TOKEN"))
		}
		_, _ = w.Write([]byte(`{"username": "pm-bot", "name": "PM Bot"}`))
	})
	mux.HandleFunc("/api/v4/projects", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("membership") != "true" {
			t.Fatalf("membership = %q", r.URL.Query().Get("membership"))
		}
		_, _ = w.Write([]byte(`[{"id": 42, "path_with_namespace": "ops/pm", "default_branch": "main"}]`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	collector := newTestCollector(t, mux)

	account, err := collector.VerifyConnection(context.Background(), credentials(server))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if account.Login != "pm-bot" || account.DisplayName != "PM Bot" {
		t.Fatalf("account wrong: %+v", account)
	}
	listing, err := collector.ListAccessibleRepos(context.Background(), credentials(server), "")
	if err != nil {
		t.Fatalf("list repos: %v", err)
	}
	if len(listing.Repos) != 1 || listing.Repos[0].ExternalID != "42" || listing.Repos[0].Path != "ops/pm" {
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
