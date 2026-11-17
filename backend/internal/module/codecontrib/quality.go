package codecontrib

import "strings"

// 逐提交质量证据的提取规则：扫描 diff 的新增行，产出
//   - DiffSample：新增行内容截断样本，报告把它交给 AI 逐提交点评；
//   - DebugHits/TodoHits：调试残留与 TODO 命中计数，构成可审计的质量扣分。
//
// 全部为确定性文本规则，大小写不敏感；空行与超长行截断，防止巨型 diff
// 拖慢采集或撑爆存储。

const (
	// DiffSampleMaxBytes 单提交样本字节上限；sampleLineMaxBytes 单行上限。
	DiffSampleMaxBytes   = 4096
	sampleLineMaxBytes   = 240
	sampleMaxLines       = 200
	diffAddedLinePrefix  = "+"
	diffFileHeaderPrefix = "+++"
)

// debugResidueMarkers 命中即计一次调试残留（每行最多计一次）。
var debugResidueMarkers = []string{
	"console.log", "console.debug", "debugger",
	"fmt.println", "fmt.print", "system.out.print",
}

// todoMarkers 命中即计一次 TODO 类欠账标记。
var todoMarkers = []string{"todo", "fixme", "hack"}

// AddedLinesFromPatches pulls the added lines out of unified-diff patch text.
// Patch headers ("+++") never count; empty patches (binary) contribute nothing.
func AddedLinesFromPatches(patch string) []string {
	if patch == "" {
		return nil
	}
	lines := strings.Split(patch, "\n")
	added := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, diffFileHeaderPrefix) {
			continue
		}
		if strings.HasPrefix(line, diffAddedLinePrefix) {
			added = append(added, line[1:])
		}
	}
	return added
}

// SummarizeAddedLines builds the commit-level quality evidence from added
// lines. The sample keeps insertion order so the AI reads the diff as written.
func SummarizeAddedLines(addedLines []string) (sample string, debugHits, todoHits int) {
	var builder strings.Builder
	sampled := 0
	for _, line := range addedLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if containsAny(lower, debugResidueMarkers) {
			debugHits++
		}
		if containsAny(lower, todoMarkers) {
			todoHits++
		}
		if sampled >= sampleMaxLines || builder.Len() >= DiffSampleMaxBytes {
			continue
		}
		if len(trimmed) > sampleLineMaxBytes {
			trimmed = TruncateUTF8(trimmed, sampleLineMaxBytes)
		}
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(trimmed)
		sampled++
	}
	sample = builder.String()
	if len(sample) > DiffSampleMaxBytes {
		sample = TruncateUTF8(sample, DiffSampleMaxBytes)
	}
	return sample, debugHits, todoHits
}

func containsAny(lower string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
