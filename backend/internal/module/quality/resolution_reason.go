package quality

import "strings"

var bugResolutionReasons = []struct {
	code    string
	aliases []string
}{
	{code: ""},
	{code: "as_designed", aliases: []string{"符合设计", "设计如此"}},
	{code: "duplicate", aliases: []string{"重复问题", "重复Bug"}},
	{code: "external", aliases: []string{"外部原因"}},
	{code: "fixed", aliases: []string{"已修复", "已解决"}},
	{code: "cannot_reproduce", aliases: []string{"无法复现", "无法重现"}},
	{code: "postponed", aliases: []string{"延期处理"}},
	{code: "wont_fix", aliases: []string{"不予修复", "不予解决"}},
}

// BugResolutionReasons is shared by HTTP, Tool schemas and mutation validation.
// Empty clears a reason outside review/done; those states require a reason.
func BugResolutionReasons() []string {
	values := make([]string, 0, len(bugResolutionReasons))
	for _, reason := range bugResolutionReasons {
		values = append(values, reason.code)
	}
	return values
}

// BugResolutionReasonInputs includes the aliases accepted by Tool adapters.
// Domain writes still validate only canonical codes after normalization.
func BugResolutionReasonInputs() []string {
	values := BugResolutionReasons()
	for _, reason := range bugResolutionReasons {
		values = append(values, reason.aliases...)
	}
	return values
}

func ValidBugResolutionReason(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, reason := range BugResolutionReasons() {
		if value == reason {
			return true
		}
	}
	return false
}

// NormalizeBugResolutionReason maps known Tool inputs and historical labels
// to canonical codes. Unknown values remain invalid rather than being guessed.
func NormalizeBugResolutionReason(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, reason := range bugResolutionReasons {
		if value == reason.code {
			return reason.code
		}
		for _, alias := range reason.aliases {
			if strings.EqualFold(value, alias) {
				return reason.code
			}
		}
	}
	return value
}
