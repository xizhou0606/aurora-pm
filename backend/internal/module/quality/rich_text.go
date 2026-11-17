package quality

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

var richTextBlockTypes = map[string]struct{}{
	"blockquote": {}, "bulletList": {}, "codeBlock": {}, "heading": {},
	"listItem": {}, "orderedList": {}, "paragraph": {},
}

type richTextNode struct {
	Type    string         `json:"type"`
	Text    string         `json:"text"`
	Attrs   richTextAttrs  `json:"attrs"`
	Content []richTextNode `json:"content"`
}

type richTextAttrs struct {
	Label    string `json:"label"`
	Username string `json:"username"`
}

// RichTextPlainText converts the persisted quality rich-text document into
// the stable text representation used by notifications.
func RichTextPlainText(value string) string {
	source := strings.TrimSpace(value)
	if source == "" {
		return ""
	}
	var document richTextNode
	if json.Unmarshal([]byte(source), &document) != nil || document.Type != "doc" {
		return source
	}
	plain := richTextNodePlainText(document)
	lines := strings.Split(strings.ReplaceAll(plain, "\u00a0", " "), "\n")
	for index := range lines {
		lines[index] = strings.TrimSpace(lines[index])
	}
	plain = strings.Join(lines, "\n")
	for strings.Contains(plain, "\n\n\n") {
		plain = strings.ReplaceAll(plain, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(plain)
}

// TextRuneCount returns the visible Unicode character count for plain text.
func TextRuneCount(value string) int {
	return utf8.RuneCountInString(value)
}

// RichTextRuneCount counts the rendered text instead of its serialized JSON.
func RichTextRuneCount(value string) int {
	return TextRuneCount(RichTextPlainText(value))
}

func richTextNodePlainText(node richTextNode) string {
	if node.Type == "hardBreak" {
		return "\n"
	}
	if node.Type == "image" {
		return "\n[图片]\n"
	}
	if node.Type == "mention" {
		label := strings.TrimSpace(node.Attrs.Label)
		if label == "" {
			label = strings.TrimSpace(node.Attrs.Username)
		}
		if label != "" {
			return "@" + label
		}
		return ""
	}
	if node.Text != "" {
		return node.Text
	}
	var text strings.Builder
	for _, child := range node.Content {
		text.WriteString(richTextNodePlainText(child))
	}
	if _, block := richTextBlockTypes[node.Type]; block {
		text.WriteByte('\n')
	}
	return text.String()
}
