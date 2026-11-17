package document

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

func (service *Service) normalizeCommentBody(
	ctx context.Context,
	workspaceID uint,
	body string,
) (string, []uint, error) {
	body = strings.TrimSpace(body)
	if body == "" || len(body) > CommentBodyByteLimit {
		return "", nil, ErrInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewBufferString(body))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return "", nil, ErrInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", nil, ErrInvalidInput
	}
	mentions := make([]uint, 0)
	meaningful, err := sanitizeCommentNode(root, &mentions)
	if err != nil || !meaningful {
		return "", nil, ErrInvalidInput
	}
	canonical, _, plainText, err := NormalizeContent(mustMarshalCommentNode(root))
	if err != nil {
		return "", nil, err
	}
	mentions = uniqueIDs(mentions)
	if len(mentions) > CommentMentionLimit || utf8.RuneCountInString(plainText)+len(mentions) > CommentBodyVisibleRuneLimit {
		return "", nil, ErrInvalidInput
	}
	if err := service.mentions.ValidateCommentMentionUserIDs(ctx, workspaceID, mentions); err != nil {
		return "", nil, ErrCommentMentionInvalid
	}
	return string(canonical), mentions, nil
}

func sanitizeCommentNode(node map[string]any, mentions *[]uint) (bool, error) {
	nodeType, ok := node["type"].(string)
	if !ok {
		return false, ErrInvalidInput
	}
	meaningful := false
	switch nodeType {
	case "text":
		value, ok := node["text"].(string)
		if !ok {
			return false, ErrInvalidInput
		}
		meaningful = strings.TrimSpace(value) != ""
	case "mention":
		attrs, ok := node["attrs"].(map[string]any)
		if !ok {
			return false, ErrCommentMentionInvalid
		}
		userID, ok := commentMentionUserID(attrs["id"])
		if !ok {
			return false, ErrCommentMentionInvalid
		}
		canonicalAttrs := map[string]any{"id": strconv.FormatUint(uint64(userID), 10)}
		for _, key := range []string{"label", "username"} {
			if value, exists := attrs[key].(string); exists {
				canonicalAttrs[key] = strings.TrimSpace(value)
			}
		}
		node["attrs"] = canonicalAttrs
		*mentions = append(*mentions, userID)
		meaningful = true
	case "image":
		meaningful = true
	}
	if rawChildren, exists := node["content"]; exists {
		children, ok := rawChildren.([]any)
		if !ok {
			return false, ErrInvalidInput
		}
		for _, rawChild := range children {
			child, ok := rawChild.(map[string]any)
			if !ok {
				return false, ErrInvalidInput
			}
			childMeaningful, err := sanitizeCommentNode(child, mentions)
			if err != nil {
				return false, err
			}
			meaningful = meaningful || childMeaningful
		}
	}
	return meaningful, nil
}

func commentMentionUserID(value any) (uint, bool) {
	var raw string
	switch typed := value.(type) {
	case json.Number:
		raw = typed.String()
	case string:
		raw = strings.TrimSpace(typed)
	default:
		return 0, false
	}
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || parsed == 0 {
		return 0, false
	}
	return uint(parsed), true
}

func mustMarshalCommentNode(node map[string]any) json.RawMessage {
	encoded, err := json.Marshal(node)
	if err != nil {
		return nil
	}
	return encoded
}
