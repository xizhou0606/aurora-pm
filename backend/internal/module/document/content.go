package document

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"

	"github.com/google/uuid"
)

var supportedV1NodeTypes = map[string]struct{}{
	"doc": {}, "paragraph": {}, "text": {}, "heading": {},
	"bulletList": {}, "orderedList": {}, "listItem": {},
	"blockquote": {}, "hardBreak": {}, "horizontalRule": {},
	"image": {}, "mention": {},
}

var supportedV2NodeTypes = map[string]struct{}{
	"doc": {}, "paragraph": {}, "text": {}, "heading": {},
	"bulletList": {}, "orderedList": {}, "listItem": {},
	"codeBlock": {}, "blockquote": {}, "hardBreak": {}, "horizontalRule": {},
	"image": {}, "mention": {}, "taskList": {}, "taskItem": {},
	"table": {}, "tableRow": {}, "tableCell": {}, "tableHeader": {},
	"callout": {}, "tableOfContents": {}, "unsupportedBlock": {},
}

var supportedMarkTypes = map[string]struct{}{
	"bold": {}, "italic": {}, "strike": {}, "code": {}, "link": {},
}

func DefaultContent() json.RawMessage {
	return json.RawMessage(`{"type":"doc","content":[{"type":"paragraph"}]}`)
}

func NormalizeContent(raw json.RawMessage) (json.RawMessage, string, string, error) {
	return NormalizeContentForSchema(raw, SchemaVersionV1)
}

func NormalizeContentForSchema(raw json.RawMessage, schemaVersion uint) (json.RawMessage, string, string, error) {
	supportedNodes, ok := supportedNodesForSchema(schemaVersion)
	if !ok {
		return nil, "", "", ErrInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return nil, "", "", ErrInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, "", "", ErrInvalidInput
	}
	if nodeType, _ := root["type"].(string); nodeType != "doc" {
		return nil, "", "", ErrInvalidInput
	}
	text := make([]string, 0)
	if err := validateContentNode(root, &text, schemaVersion, supportedNodes); err != nil {
		return nil, "", "", err
	}
	canonical, err := json.Marshal(root)
	if err != nil {
		return nil, "", "", ErrInvalidInput
	}
	digest := sha256.Sum256(canonical)
	return canonical, hex.EncodeToString(digest[:]), strings.TrimSpace(strings.Join(text, " ")), nil
}

func supportedNodesForSchema(schemaVersion uint) (map[string]struct{}, bool) {
	switch schemaVersion {
	case SchemaVersionV1:
		return supportedV1NodeTypes, true
	case SchemaVersionV2:
		return supportedV2NodeTypes, true
	case SchemaVersionV3:
		nodes := make(map[string]struct{}, len(supportedV2NodeTypes)+1)
		for key := range supportedV2NodeTypes {
			nodes[key] = struct{}{}
		}
		nodes["bitable"] = struct{}{}
		return nodes, true
	default:
		return nil, false
	}
}

func validateContentNode(
	node map[string]any,
	text *[]string,
	schemaVersion uint,
	supportedNodes map[string]struct{},
) error {
	nodeType, ok := node["type"].(string)
	if !ok {
		return ErrInvalidInput
	}
	if _, ok := supportedNodes[nodeType]; !ok {
		return ErrInvalidInput
	}
	if nodeType == "bitable" {
		if err := validateBitableReference(node); err != nil {
			return err
		}
	}
	if nodeType == "heading" {
		attrs, _ := node["attrs"].(map[string]any)
		level, ok := attrs["level"].(json.Number)
		if !ok || !supportedHeadingLevel(level.String(), schemaVersion) {
			return ErrInvalidInput
		}
	}
	if nodeType == "text" {
		value, ok := node["text"].(string)
		if !ok {
			return ErrInvalidInput
		}
		if strings.TrimSpace(value) != "" {
			*text = append(*text, value)
		}
	}
	if rawMarks, exists := node["marks"]; exists {
		marks, ok := rawMarks.([]any)
		if !ok {
			return ErrInvalidInput
		}
		for _, rawMark := range marks {
			mark, ok := rawMark.(map[string]any)
			if !ok {
				return ErrInvalidInput
			}
			markType, _ := mark["type"].(string)
			if _, ok := supportedMarkTypes[markType]; !ok {
				return ErrInvalidInput
			}
		}
	}
	if rawChildren, exists := node["content"]; exists {
		children, ok := rawChildren.([]any)
		if !ok {
			return ErrInvalidInput
		}
		for _, rawChild := range children {
			child, ok := rawChild.(map[string]any)
			if !ok || validateContentNode(child, text, schemaVersion, supportedNodes) != nil {
				return ErrInvalidInput
			}
		}
	}
	return nil
}

func supportedHeadingLevel(level string, schemaVersion uint) bool {
	if level == "1" || level == "2" || level == "3" {
		return true
	}
	return schemaVersion >= SchemaVersionV2 && (level == "4" || level == "5" || level == "6")
}

// A reference is not a grant or a copy of source data. Authorizing the source is
// the bitable API's responsibility on every read/write; the document stores only IDs.
func validateBitableReference(node map[string]any) error {
	attrs, ok := node["attrs"].(map[string]any)
	if !ok || len(attrs) != 5 {
		return ErrInvalidInput
	}
	version, ok := attrs["schemaVersion"].(json.Number)
	if !ok || version.String() != "1" {
		return ErrInvalidInput
	}
	for _, key := range []string{"resourceId", "tableId", "viewId"} {
		id, ok := attrs[key].(string)
		if !ok {
			return ErrInvalidInput
		}
		if _, err := uuid.Parse(id); err != nil {
			return ErrInvalidInput
		}
	}
	height, ok := attrs["height"].(json.Number)
	if !ok {
		return ErrInvalidInput
	}
	h, err := height.Int64()
	if err != nil || h < 240 || h > 900 {
		return ErrInvalidInput
	}
	if _, exists := node["content"]; exists {
		return ErrInvalidInput
	}
	return nil
}
