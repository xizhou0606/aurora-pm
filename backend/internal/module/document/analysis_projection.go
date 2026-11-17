package document

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

var ErrAnalysisContentLimit = errors.New("站内文档正文或结构超过解析上限，请拆分文档后重试")
var ErrAnalysisUnsupported = errors.New("站内文档包含暂不支持的内容块，请转换后重试")

const AnalysisProjectionVersion = "rich-text-v1"
const analysisMaxBytes = 2 * 1024 * 1024

type AnalysisProjection struct {
	Text             string   `json:"text"`
	ContentHash      string   `json:"content_hash"`
	ProjectionHash   string   `json:"projection_hash"`
	ConverterVersion string   `json:"converter_version"`
	BlockRefs        []string `json:"block_refs"`
	Unexpanded       []string `json:"unexpanded"`
}

type analysisNode struct {
	Type    string         `json:"type"`
	Text    string         `json:"text"`
	Attrs   map[string]any `json:"attrs"`
	Marks   []analysisMark `json:"marks"`
	Content []analysisNode `json:"content"`
}
type analysisMark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs"`
}
type analysisRenderer struct {
	refs, omitted []string
	nodes         int
}

// ProjectAnalysisText preserves source structure without fetching referenced resources.
// The hash identifies the canonical source JSON, never the lossy text projection.
func ProjectAnalysisText(raw json.RawMessage, schemaVersion uint, revision uint) (AnalysisProjection, error) {
	if len(raw) > analysisMaxBytes {
		return AnalysisProjection{}, ErrAnalysisContentLimit
	}
	if revision == 0 {
		return AnalysisProjection{}, ErrInvalidInput
	}
	var root analysisNode
	if err := json.Unmarshal(raw, &root); err != nil {
		return AnalysisProjection{}, ErrInvalidInput
	}
	renderer := &analysisRenderer{}
	if err := renderer.check(root, 0); err != nil {
		return AnalysisProjection{}, err
	}
	_, sourceHash, _, err := NormalizeContentForSchema(raw, schemaVersion)
	if err != nil {
		return AnalysisProjection{}, err
	}
	text := strings.TrimSpace(renderer.render(root, fmt.Sprintf("revision:%d/$", revision), 0))
	if len(text) > analysisMaxBytes {
		return AnalysisProjection{}, ErrAnalysisContentLimit
	}
	digest := sha256.Sum256([]byte(text))
	return AnalysisProjection{Text: text, ContentHash: sourceHash, ProjectionHash: hex.EncodeToString(digest[:]), ConverterVersion: AnalysisProjectionVersion, BlockRefs: renderer.refs, Unexpanded: renderer.omitted}, nil
}

func (r *analysisRenderer) check(n analysisNode, depth int) error {
	r.nodes++
	if depth > 64 || r.nodes > 20000 {
		return ErrAnalysisContentLimit
	}
	if n.Type == "unsupportedBlock" {
		return ErrAnalysisUnsupported
	}
	for _, key := range []string{"colspan", "rowspan", "start"} {
		if raw, exists := n.Attrs[key]; exists {
			number, ok := raw.(float64)
			if !ok || number < 1 || number > 100000 || math.Trunc(number) != number {
				return ErrInvalidInput
			}
		}
	}
	if n.Type == "taskItem" {
		if raw, exists := n.Attrs["checked"]; exists {
			if _, ok := raw.(bool); !ok {
				return ErrInvalidInput
			}
		}
	}
	for _, child := range n.Content {
		if err := r.check(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (r *analysisRenderer) render(n analysisNode, path string, depth int) string {
	if n.Type != "text" {
		r.refs = append(r.refs, path)
	}
	var parts []string
	for i, child := range n.Content {
		parts = append(parts, r.render(child, fmt.Sprintf("%s/content/%d", path, i), depth+1))
	}
	body := strings.Join(parts, "")
	switch n.Type {
	case "text":
		for _, mark := range n.Marks {
			if mark.Type == "link" {
				r.omitted = append(r.omitted, path+":link")
			}
		}
		return analysisMarkedText(n)
	case "doc":
		return body
	case "paragraph":
		return body + "\n\n"
	case "heading":
		return strings.Repeat("#", analysisNumber(n.Attrs, "level", 1)) + " " + body + "\n\n"
	case "hardBreak":
		return "\n"
	case "horizontalRule":
		return "\n---\n\n"
	case "bulletList", "orderedList", "taskList":
		var items []string
		for i, child := range n.Content {
			prefix := "- "
			if n.Type == "orderedList" {
				prefix = fmt.Sprintf("%d. ", analysisNumber(n.Attrs, "start", 1)+i)
			}
			if n.Type == "taskList" {
				prefix = "- [ ] "
				if checked, _ := child.Attrs["checked"].(bool); checked {
					prefix = "- [x] "
				}
			}
			items = append(items, prefix+strings.ReplaceAll(strings.TrimSpace(parts[i]), "\n", "\n  "))
		}
		return strings.Join(items, "\n") + "\n\n"
	case "listItem", "taskItem":
		return body
	case "callout":
		tone, _ := n.Attrs["tone"].(string)
		if tone == "" {
			tone = "info"
		}
		return "[callout tone=" + tone + "]\n" + body + "[/callout]\n\n"
	case "blockquote":
		return "> " + strings.ReplaceAll(strings.TrimSpace(body), "\n", "\n> ") + "\n\n"
	case "codeBlock":
		fence := "```"
		for strings.Contains(body, fence) {
			fence += "`"
		}
		language, _ := n.Attrs["language"].(string)
		language = strings.Map(func(c rune) rune {
			if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
				return c
			}
			return -1
		}, language)
		return fence + language + "\n" + body + "\n" + fence + "\n\n"
	case "table":
		return "[table]\n" + body + "[/table]\n\n"
	case "tableRow":
		return "[row]\n" + body + "[/row]\n"
	case "tableCell", "tableHeader":
		return fmt.Sprintf("[%s colspan=%d rowspan=%d]\n%s\n[/%s]\n", n.Type, analysisNumber(n.Attrs, "colspan", 1), analysisNumber(n.Attrs, "rowspan", 1), strings.TrimSpace(body), n.Type)
	case "mention":
		label, _ := n.Attrs["label"].(string)
		if label == "" {
			label = "mention"
		}
		return "@" + label
	case "image", "bitable", "tableOfContents":
		r.omitted = append(r.omitted, path+":"+n.Type)
		label, _ := n.Attrs["alt"].(string)
		return "[unexpanded " + n.Type + ": " + label + "]\n\n"
	}
	return body
}

func analysisNumber(attrs map[string]any, key string, fallback int) int {
	value, ok := attrs[key].(float64)
	if !ok || value < 1 || value > 100000 {
		return fallback
	}
	return int(value)
}
