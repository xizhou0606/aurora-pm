package codecontrib

import (
	"strings"
	"testing"
)

func TestAddedLinesFromPatchesSkipsHeadersAndBinary(t *testing.T) {
	if got := AddedLinesFromPatches(""); got != nil {
		t.Fatalf("empty patch must yield no lines, got %v", got)
	}
	got := AddedLinesFromPatches("@@ -1,2 +1,3 @@\n context\n+added one\n-removed\n+++ a/main.go\n+added two\n")
	if len(got) != 2 || got[0] != "added one" || got[1] != "added two" {
		t.Fatalf("added lines wrong: %v", got)
	}
}

func TestSummarizeAddedLinesCountsAndSamples(t *testing.T) {
	added := []string{
		"console.log('debug');",
		"const value = compute();",
		"// TODO: 抽公共函数",
		"console.warn('boundary')", // warn 不在残留标记里
		"",
		"FIXME: 后续优化",
	}
	sample, debugHits, todoHits := SummarizeAddedLines(added)
	if debugHits != 1 || todoHits != 2 {
		t.Fatalf("hits = (debug %d, todo %d), want (1, 2)", debugHits, todoHits)
	}
	if !strings.Contains(sample, "const value = compute();") ||
		!strings.Contains(sample, "console.warn('boundary')") {
		t.Fatalf("sample must keep ordinary added lines: %q", sample)
	}
}

func TestSummarizeAddedLinesTruncatesHugeDiffs(t *testing.T) {
	added := make([]string, 0, sampleMaxLines+50)
	for index := 0; index < sampleMaxLines+50; index++ {
		added = append(added, "line of code that keeps going")
	}
	sample, debugHits, todoHits := SummarizeAddedLines(added)
	if len(sample) > DiffSampleMaxBytes {
		t.Fatalf("sample len = %d, cap %d", len(sample), DiffSampleMaxBytes)
	}
	if strings.Count(sample, "\n") >= sampleMaxLines {
		t.Fatalf("sample must cap line count, got %d", strings.Count(sample, "\n"))
	}
	if debugHits != 0 || todoHits != 0 {
		t.Fatalf("plain lines must not hit, got (%d, %d)", debugHits, todoHits)
	}
}
