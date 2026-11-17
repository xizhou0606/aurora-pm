package quality

import "strings"

const (
	BugDefectDomainFunctional = "functional"
	BugDefectDomainUI         = "ui"
)

// NormalizeBugDefectDomain keeps legacy rows compatible: only an explicit UI
// value is UI, while empty or unknown persisted values are read as functional.
func NormalizeBugDefectDomain(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), BugDefectDomainUI) {
		return BugDefectDomainUI
	}
	return BugDefectDomainFunctional
}

func ValidBugDefectDomain(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == BugDefectDomainFunctional || value == BugDefectDomainUI
}
