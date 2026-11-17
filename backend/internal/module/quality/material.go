package quality

import (
	"encoding/json"
	"net/url"
	"strings"
)

// Material is the same URL-only attachment contract used by the quality UI.
type Material struct {
	URL string `json:"url"`
}

func ValidateMaterials(items []Material) error {
	for _, item := range items {
		u, err := url.Parse(strings.TrimSpace(item.URL))
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return ErrBugMutationInvalid
		}
	}
	return nil
}
func EncodeMaterials(items []Material) string {
	if items == nil {
		items = []Material{}
	}
	raw, _ := json.Marshal(items)
	return string(raw)
}
func DecodeMaterials(raw string) []Material {
	items := []Material{}
	if json.Unmarshal([]byte(raw), &items) != nil || items == nil {
		return []Material{}
	}
	return items
}
