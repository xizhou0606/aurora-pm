package httpapi

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var taskCommentUsernameMention = regexp.MustCompile(`@([\p{L}\p{N}_.-]+)`)

func taskCommentMentionsUser(body string, factUserID uint, username string) bool {
	if factUserID == 0 {
		return false
	}
	for _, id := range taskCommentMentionIDs(body) {
		if id == factUserID {
			return true
		}
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return false
	}
	for _, match := range taskCommentUsernameMention.FindAllStringSubmatch(body, -1) {
		if len(match) > 1 && strings.EqualFold(strings.TrimSpace(match[1]), username) {
			return true
		}
	}
	return false
}

func taskCommentMentionIDs(body string) []uint {
	body = strings.TrimPrefix(strings.TrimSpace(body), "\ufeff")
	if !strings.HasPrefix(body, "{") && !strings.HasPrefix(body, "[") {
		return nil
	}
	var root any
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.UseNumber()
	if decoder.Decode(&root) != nil {
		return nil
	}
	seen := map[uint]struct{}{}
	var result []uint
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			if kind, _ := typed["type"].(string); kind == "mention" {
				if attrs, ok := typed["attrs"].(map[string]any); ok {
					for _, key := range []string{"id", "userId", "user_id"} {
						id := taskCommentMentionID(attrs[key])
						if id > 0 {
							if _, exists := seen[id]; !exists {
								seen[id] = struct{}{}
								result = append(result, id)
							}
							break
						}
					}
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(root)
	return result
}

func taskCommentMentionID(value any) uint {
	var raw string
	switch typed := value.(type) {
	case string:
		raw = typed
	case json.Number:
		raw = typed.String()
	case float64:
		if typed <= 0 || typed != float64(uint(typed)) {
			return 0
		}
		return uint(typed)
	default:
		return 0
	}
	id, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil || id == 0 {
		return 0
	}
	return uint(id)
}
