package document

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"unicode/utf16"
)

func normalizeCommentAnchor(anchor CommentAnchor, current Document) (CommentAnchor, error) {
	switch anchor.Kind {
	case CommentAnchorDocument:
		if anchor.Selection != nil {
			return CommentAnchor{}, ErrInvalidInput
		}
		anchor.State = CommentAnchorStateDocument
	case CommentAnchorSelectionSnapshotV1:
		selection := anchor.Selection
		if selection == nil || selection.DocumentRevision == 0 || selection.DocumentRevision != current.Revision ||
			selection.To <= selection.From || strings.TrimSpace(selection.Quote) == "" ||
			len(selection.Quote) > 4_000 || len(selection.Prefix) > 2_000 || len(selection.Suffix) > 2_000 ||
			!selectionSnapshotMatches(current.Content, *selection) {
			return CommentAnchor{}, ErrInvalidInput
		}
		anchor.State = CommentAnchorStateAnchored
	default:
		return CommentAnchor{}, ErrInvalidInput
	}
	return anchor, nil
}

func resolveCommentAnchor(anchor CommentAnchor, current Document) CommentAnchor {
	if anchor.Kind == CommentAnchorDocument {
		anchor.State = CommentAnchorStateDocument
		return anchor
	}
	if anchor.Kind != CommentAnchorSelectionSnapshotV1 || anchor.Selection == nil || anchor.Selection.DocumentRevision == 0 {
		anchor.State = CommentAnchorStateOrphaned
		return anchor
	}
	selection := *anchor.Selection
	if selection.DocumentRevision != current.Revision || !selectionSnapshotMatches(current.Content, selection) {
		var matched bool
		selection, matched = relocateSelectionSnapshot(current.Content, selection)
		if !matched {
			anchor.State = CommentAnchorStateOrphaned
			return anchor
		}
	}
	selection.DocumentRevision = current.Revision
	anchor.Selection = &selection
	anchor.State = CommentAnchorStateAnchored
	return anchor
}

// selectionSnapshotMatches supports the stable subset that can be verified
// from canonical ProseMirror JSON without a Yjs RelativePosition. The range
// must stay inside one paragraph or heading. Cross-block or opaque inline-node
// selections are rejected rather than risking a highlight on the wrong text.
func selectionSnapshotMatches(content json.RawMessage, snapshot SelectionSnapshotV1) bool {
	root, ok := decodeCommentAnchorDocument(content)
	if !ok {
		return false
	}
	return textBlockSnapshotMatches(root, 0, snapshot)
}

func decodeCommentAnchorDocument(content json.RawMessage) (map[string]any, bool) {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return nil, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF || root["type"] != "doc" {
		return nil, false
	}
	return root, true
}

func textBlockSnapshotMatches(node map[string]any, nodeStart uint, snapshot SelectionSnapshotV1) bool {
	nodeType, _ := node["type"].(string)
	content, _ := node["content"].([]any)
	if nodeType == "paragraph" || nodeType == "heading" {
		contentStart := nodeStart + 1
		contentEnd := contentStart + prosemirrorContentSize(content)
		if snapshot.From >= contentStart && snapshot.To <= contentEnd {
			quote, ok := inlineTextRange(content, contentStart, snapshot.From, snapshot.To)
			if !ok || quote != snapshot.Quote {
				return false
			}
			if snapshot.Prefix != "" {
				prefixSize := utf16Size(snapshot.Prefix)
				if prefixSize > snapshot.From-contentStart {
					return false
				}
				prefix, ok := inlineTextRange(content, contentStart, snapshot.From-prefixSize, snapshot.From)
				if !ok || prefix != snapshot.Prefix {
					return false
				}
			}
			if snapshot.Suffix != "" {
				suffixSize := utf16Size(snapshot.Suffix)
				if snapshot.To+suffixSize > contentEnd {
					return false
				}
				suffix, ok := inlineTextRange(content, contentStart, snapshot.To, snapshot.To+suffixSize)
				if !ok || suffix != snapshot.Suffix {
					return false
				}
			}
			return true
		}
	}
	childStart := nodeStart
	if nodeType != "doc" {
		childStart++
	}
	for _, rawChild := range content {
		child, ok := rawChild.(map[string]any)
		if !ok {
			return false
		}
		childSize := prosemirrorNodeSize(child)
		if snapshot.From >= childStart && snapshot.To <= childStart+childSize &&
			textBlockSnapshotMatches(child, childStart, snapshot) {
			return true
		}
		childStart += childSize
	}
	return false
}

func inlineTextRange(content []any, contentStart, from, to uint) (string, bool) {
	position := contentStart
	var result strings.Builder
	for _, rawChild := range content {
		child, ok := rawChild.(map[string]any)
		if !ok {
			return "", false
		}
		size := prosemirrorNodeSize(child)
		childEnd := position + size
		if to <= position {
			break
		}
		if from < childEnd && to > position {
			typeName, _ := child["type"].(string)
			switch typeName {
			case "text":
				text, ok := child["text"].(string)
				if !ok {
					return "", false
				}
				startOffset := maxUint(from, position) - position
				endOffset := minUint(to, childEnd) - position
				piece, ok := sliceUTF16(text, startOffset, endOffset)
				if !ok {
					return "", false
				}
				result.WriteString(piece)
			case "mention":
				if from > position || to < childEnd {
					return "", false
				}
				attrs, _ := child["attrs"].(map[string]any)
				label := firstNonemptyString(attrs["label"], attrs["username"], attrs["id"])
				if label == "" {
					return "", false
				}
				result.WriteString("@" + label)
			case "hardBreak":
				if from > position || to < childEnd {
					return "", false
				}
				result.WriteByte('\n')
			default:
				return "", false
			}
		}
		position = childEnd
	}
	return result.String(), result.Len() > 0
}

func prosemirrorContentSize(content []any) uint {
	var size uint
	for _, rawChild := range content {
		if child, ok := rawChild.(map[string]any); ok {
			size += prosemirrorNodeSize(child)
		}
	}
	return size
}

func prosemirrorNodeSize(node map[string]any) uint {
	nodeType, _ := node["type"].(string)
	if nodeType == "text" {
		text, _ := node["text"].(string)
		return uint(len(utf16.Encode([]rune(text))))
	}
	content, _ := node["content"].([]any)
	switch nodeType {
	case "paragraph", "heading", "bulletList", "orderedList", "listItem", "blockquote",
		"codeBlock", "table", "tableRow", "tableCell", "tableHeader", "taskList", "taskItem", "callout":
		// ProseMirror non-leaf nodes always contribute their opening and closing
		// tokens, even when an empty node omits `content` from its JSON form.
		return prosemirrorContentSize(content) + 2
	}
	return 1
}

func sliceUTF16(value string, from, to uint) (string, bool) {
	encoded := utf16.Encode([]rune(value))
	if from > to || int(to) > len(encoded) {
		return "", false
	}
	return string(utf16.Decode(encoded[from:to])), true
}

func utf16Size(value string) uint {
	return uint(len(utf16.Encode([]rune(value))))
}

func firstNonemptyString(values ...any) string {
	for _, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func maxUint(left, right uint) uint {
	if left > right {
		return left
	}
	return right
}

func minUint(left, right uint) uint {
	if left < right {
		return left
	}
	return right
}
