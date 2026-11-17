package requirementhttp

import (
	"net/url"
	"strings"

	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
)

func MaterialOriginURL(row requirement.Material) string {
	return strings.TrimSpace(row.URL)
}

func MaterialOpenURL(row requirement.Material) string {
	return materialDownloadPath(row)
}

// IsMaterialLink reports whether the material references an external URL
// instead of a stored object.
func IsMaterialLink(row requirement.Material) bool {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(row.ObjectKey)), "link/") {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(row.ContentType), "text/uri-list")
}

func CanonicalDesignURL(rows []requirement.Material, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	for _, row := range rows {
		if materialRefersTo(row, raw) {
			origin := MaterialOriginURL(row)
			if origin != "" {
				return origin, true
			}
		}
	}
	return "", false
}

func materialRefersTo(row requirement.Material, raw string) bool {
	origin := MaterialOriginURL(row)
	open := MaterialOpenURL(row)
	if raw == origin || raw == open {
		return true
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	path := strings.TrimRight(parsed.Path, "/")
	return path == open || strings.HasSuffix(path, open)
}
