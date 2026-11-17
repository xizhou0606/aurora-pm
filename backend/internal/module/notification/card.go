package notification

import (
	"encoding/json"
	"strings"
)

const CardTitle = "【PM】通知"

// InteractiveCardJSON wraps PM-owned plain text in a Feishu card so every
// scene reuses the same PM bot chrome. It must not use One2All branding.
func InteractiveCardJSON(body string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", nil
	}
	card := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"template": "blue",
			"title":    map[string]any{"tag": "plain_text", "content": CardTitle},
		},
		"elements": []any{
			map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": linkifyCardBody(body),
				},
			},
		},
	}
	raw, err := json.Marshal(card)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func linkifyCardBody(body string) string {
	lines := strings.Split(body, "\n")
	for index, line := range lines {
		label, url, ok := strings.Cut(line, "：")
		if !ok || strings.TrimSpace(label) != "链接" {
			continue
		}
		url = strings.TrimSpace(url)
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			continue
		}
		lines[index] = "链接：[" + url + "](" + url + ")"
	}
	return strings.Join(lines, "\n")
}
