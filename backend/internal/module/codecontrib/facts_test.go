package codecontrib

import (
	"testing"
	"time"
)

func testTime() time.Time {
	return time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
}

func TestClassifyPath(t *testing.T) {
	tests := []struct {
		name  string
		rules FilterRules
		path  string
		want  FileClass
	}{
		{"lockfile at root", FilterRules{}, "go.sum", FileClassLockfile},
		{"lockfile in subdir", FilterRules{}, "web/package-lock.json", FileClassLockfile},
		{"generated protobuf", FilterRules{}, "internal/api/legacy.pb.go", FileClassGenerated},
		{"vendored tree", FilterRules{}, "third_party/lib/x.go", FileClassVendor},
		{"build output", FilterRules{}, "dist/app.js", FileClassVendor},
		{"plain source", FilterRules{}, "cmd/main.go", FileClassSource},
		{"go test file", FilterRules{}, "internal/api/handler_test.go", FileClassTest},
		{"ts spec file", FilterRules{}, "web/src/app.spec.ts", FileClassTest},
		{"markdown doc", FilterRules{}, "docs/guide.md", FileClassDoc},
		{"yaml config", FilterRules{}, "deploy/config.yaml", FileClassConfig},
		{"extra vendor pattern", FilterRules{ExtraVendorPatterns: []string{"assets/"}}, "assets/logo.svg", FileClassVendor},
		{"extra lockfile pattern", FilterRules{ExtraLockfilePatterns: []string{"*.lock"}}, "flake.lock", FileClassLockfile},
		{"override wins over lockfile", FilterRules{CountAsSource: []string{"go.sum"}}, "go.sum", FileClassSource},
		{"override wins over vendor", FilterRules{CountAsSource: []string{"vendor/kept.go"}}, "vendor/kept.go", FileClassSource},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			change := ClassifyPath(test.rules, test.path)
			if change.Class != test.want {
				t.Fatalf("ClassifyPath(%q) = %q, want %q", test.path, change.Class, test.want)
			}
			if change.Path != test.path {
				t.Fatalf("classified change lost its path: got %q want %q", change.Path, test.path)
			}
		})
	}
}

func TestParseCoAuthorTrailers(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    []string
	}{
		{
			name: "two well-formed trailers",
			message: "Add reporting page\n\n" +
				"Co-authored-by: Alice Zhang <Alice@Example.COM>\n" +
				"Co-authored-by: Bob <bob@example.com>\n" +
				"Signed-off-by: Alice <alice@example.com>",
			want: []string{"alice@example.com", "bob@example.com"},
		},
		{
			name:    "bare email without display name",
			message: "fix: parser\n\nCo-authored-by: carol@example.com",
			want:    []string{"carol@example.com"},
		},
		{
			name:    "lowercase key still matches",
			message: "tweak\n\nco-authored-by: Dave <dave@example.com>",
			want:    []string{"dave@example.com"},
		},
		{
			name:    "trailer without parseable email is dropped",
			message: "work\n\nCo-authored-by: Anonymous Contributor",
			want:    []string{},
		},
		{
			name:    "message without trailers",
			message: "plain commit message\n\nsome body text",
			want:    []string{},
		},
		{
			name:    "trailer look-alike in body is still read structurally",
			message: "see Co-authored-by: Eve <eve@example.com> in history",
			want:    []string{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ParseCoAuthorTrailers(test.message)
			if len(got) != len(test.want) {
				t.Fatalf("ParseCoAuthorTrailers = %v, want %v", got, test.want)
			}
			for index := range test.want {
				if got[index] != test.want[index] {
					t.Fatalf("ParseCoAuthorTrailers[%d] = %q, want %q", index, got[index], test.want[index])
				}
			}
		})
	}
}

func TestMarkPRDelivered(t *testing.T) {
	mergedAt := testTime()
	requests := []PullRequestFact{
		{Number: 7, MergedAt: &mergedAt, MergeSHA: "sha-c3", CommitSHAs: []string{"sha-c1"}},
		{Number: 8, CommitSHAs: []string{"sha-c4"}}, // not merged
	}
	commits := []CommitFact{
		{SHA: "sha-c1"},
		{SHA: "sha-c2"}, // direct push
		{SHA: "sha-c3"}, // squash commit carries the PR diff itself
		{SHA: "sha-c4"},
	}

	MarkPRDelivered(commits, requests)
	MarkPRDelivered(commits, requests) // idempotent under overlap-window replay

	expectations := map[string]struct {
		delivered bool
		number    int
	}{
		"sha-c1": {true, 7},
		"sha-c2": {false, 0},
		"sha-c3": {true, 7},
		"sha-c4": {false, 0},
	}
	for sha, want := range expectations {
		var found *CommitFact
		for index := range commits {
			if commits[index].SHA == sha {
				found = &commits[index]
			}
		}
		if found == nil {
			t.Fatalf("commit %s missing", sha)
		}
		if found.PRDelivered != want.delivered || found.PRNumber != want.number {
			t.Fatalf("commit %s = (delivered=%v, pr=%d), want (delivered=%v, pr=%d)",
				sha, found.PRDelivered, found.PRNumber, want.delivered, want.number)
		}
	}
}

func TestPersonLineFiles(t *testing.T) {
	files := []FileChange{{Path: "main.go", Additions: 10}}
	tests := []struct {
		name   string
		commit CommitFact
		want   bool
	}{
		{"direct push keeps its files", CommitFact{SHA: "a", Files: files}, true},
		{"merge commit carries no diff", CommitFact{SHA: "b", ParentSHAs: []string{"p1", "p2"}, Files: files}, false},
		{"pr-delivered is counted at pr level", CommitFact{SHA: "c", Files: files, PRDelivered: true, PRNumber: 7}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.commit.PersonLineFiles()
			if test.want && len(got) != len(files) {
				t.Fatalf("PersonLineFiles = %v, want the original files", got)
			}
			if !test.want && got != nil {
				t.Fatalf("PersonLineFiles = %v, want nil", got)
			}
		})
	}
}
