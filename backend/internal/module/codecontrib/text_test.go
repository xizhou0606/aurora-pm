package codecontrib

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateUTF8KeepsShortAndExactStrings(t *testing.T) {
	cases := []struct {
		input    string
		max      int
		expected string
	}{
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abc", 10, "abc"},
		{"汉字", 6, "汉字"},
		{"汉字测试", 0, "汉字测试"},
	}
	for _, testCase := range cases {
		if got := TruncateUTF8(testCase.input, testCase.max); got != testCase.expected {
			t.Fatalf("TruncateUTF8(%q, %d) = %q, want %q", testCase.input, testCase.max, got, testCase.expected)
		}
	}
}

func TestTruncateUTF8DropsSplitMultibyteTail(t *testing.T) {
	// 字节 240 切点正好落在“好”字中间：必须剪掉残缺字节而不是写坏数据。
	line := strings.Repeat("x", 239) + "好"
	got := TruncateUTF8(line, 240)
	if !utf8.ValidString(got) {
		t.Fatalf("truncated string is not valid UTF-8: %q", got)
	}
	if got != strings.Repeat("x", 239) {
		t.Fatalf("unexpected truncation result: %q", got)
	}

	// 四字节 emoji 被切掉 3 个字节时同样只保留完整前缀。
	emoji := strings.Repeat("y", 6) + "🎉" + strings.Repeat("z", 4)
	got = TruncateUTF8(emoji, 8)
	if !utf8.ValidString(got) {
		t.Fatalf("truncated string is not valid UTF-8: %q", got)
	}
	if got != strings.Repeat("y", 6) {
		t.Fatalf("unexpected truncation result: %q", got)
	}
}

// 复现线上故障：总样本 4096 字节切点落在多字节字符中间，Postgres 拒绝
// 写入导致整仓同步无限失败。采样结果必须始终是合法 UTF-8。
func TestSummarizeAddedLinesTotalCapKeepsValidUTF8(t *testing.T) {
	// 每行 120 字节，“好”字位于字节 101-103；按行分隔累积后，
	// 4096 切点恰好落进第 34 行的“好”字中间。
	line := strings.Repeat("x", 101) + "好" + strings.Repeat("x", 16)
	lines := make([]string, 0, 40)
	for range 40 {
		lines = append(lines, line)
	}
	sample, _, _ := SummarizeAddedLines(lines)
	if !utf8.ValidString(sample) {
		t.Fatalf("sample is not valid UTF-8 at total cap: %q", sample[4000:])
	}
	if len(sample) > DiffSampleMaxBytes {
		t.Fatalf("sample length %d exceeds cap %d", len(sample), DiffSampleMaxBytes)
	}
}

func TestSummarizeAddedLinesLineCapKeepsValidUTF8(t *testing.T) {
	line := strings.Repeat("x", 239) + "好"
	sample, _, _ := SummarizeAddedLines([]string{line})
	if !utf8.ValidString(sample) {
		t.Fatalf("sample is not valid UTF-8 at line cap: %q", sample)
	}
	if sample != strings.Repeat("x", 239) {
		t.Fatalf("unexpected line truncation result: %q", sample)
	}
}
