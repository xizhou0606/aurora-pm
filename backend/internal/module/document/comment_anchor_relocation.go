package document

import (
	"encoding/json"
	"strings"
	"unicode/utf16"
)

type commentAnchorTextBlock struct {
	text       string
	boundaries map[int]uint
}

type commentAnchorCandidate struct {
	from          uint
	to            uint
	prefixMatches bool
	suffixMatches bool
}

// relocateSelectionSnapshot keeps snapshot anchors useful across ordinary
// edits outside the quoted range. It only accepts an unambiguous text match;
// duplicate quotes without enough preserved context fail closed.
func relocateSelectionSnapshot(content json.RawMessage, snapshot SelectionSnapshotV1) (SelectionSnapshotV1, bool) {
	root, ok := decodeCommentAnchorDocument(content)
	if !ok {
		return snapshot, false
	}
	blocks := make([]commentAnchorTextBlock, 0)
	collectCommentAnchorTextBlocks(root, 0, &blocks)
	candidates := make([]commentAnchorCandidate, 0)
	for _, block := range blocks {
		candidates = append(candidates, commentAnchorCandidates(block, snapshot)...)
	}
	if len(candidates) == 1 && candidates[0].prefixMatches && candidates[0].suffixMatches {
		snapshot.From = candidates[0].from
		snapshot.To = candidates[0].to
		return snapshot, true
	}
	contextCandidates := make([]commentAnchorCandidate, 0)
	for _, block := range blocks {
		contextCandidates = append(contextCandidates, commentAnchorContextCandidates(block, snapshot)...)
	}
	if len(contextCandidates) == 1 {
		snapshot.From = contextCandidates[0].from
		snapshot.To = contextCandidates[0].to
		return snapshot, true
	}
	if len(contextCandidates) > 1 && (snapshot.Prefix != "" || snapshot.Suffix != "") {
		if nearest, ok := nearestCommentAnchorCandidate(contextCandidates, snapshot); ok {
			snapshot.From = nearest.from
			snapshot.To = nearest.to
			return snapshot, true
		}
	}
	// When one side of the nearby context itself changed, a unique unchanged
	// quote is still safer than orphaning. Context relocation is attempted
	// first so insertions at either edge of the selected range expand it.
	if len(candidates) == 1 {
		snapshot.From = candidates[0].from
		snapshot.To = candidates[0].to
		return snapshot, true
	}
	contextual := make([]commentAnchorCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.prefixMatches && candidate.suffixMatches {
			contextual = append(contextual, candidate)
		}
	}
	if len(contextual) != 1 {
		return snapshot, false
	}
	snapshot.From = contextual[0].from
	snapshot.To = contextual[0].to
	return snapshot, true
}

func nearestCommentAnchorCandidate(candidates []commentAnchorCandidate, snapshot SelectionSnapshotV1) (commentAnchorCandidate, bool) {
	var selected commentAnchorCandidate
	var selectedDistance uint
	hasSelected := false
	unique := false
	for _, candidate := range candidates {
		distance := absoluteUintDifference(candidate.from, snapshot.From) + absoluteUintDifference(candidate.to, snapshot.To)
		if !hasSelected || distance < selectedDistance {
			selected, selectedDistance, hasSelected, unique = candidate, distance, true, true
			continue
		}
		if distance == selectedDistance {
			unique = false
		}
	}
	return selected, hasSelected && unique
}

func absoluteUintDifference(left, right uint) uint {
	if left > right {
		return left - right
	}
	return right - left
}

func commentAnchorContextCandidates(block commentAnchorTextBlock, snapshot SelectionSnapshotV1) []commentAnchorCandidate {
	if snapshot.Prefix == "" && snapshot.Suffix == "" {
		return nil
	}
	starts := commentAnchorContextBoundaries(block, snapshot.Prefix, true)
	ends := commentAnchorContextBoundaries(block, snapshot.Suffix, false)
	result := make([]commentAnchorCandidate, 0)
	for _, start := range starts {
		for _, end := range ends {
			// Adjacent context means the selected range was fully deleted.
			if end <= start {
				continue
			}
			result = append(result, commentAnchorCandidate{
				from:          start,
				to:            end,
				prefixMatches: true,
				suffixMatches: true,
			})
		}
	}
	return result
}

func commentAnchorContextBoundaries(block commentAnchorTextBlock, context string, prefix bool) []uint {
	if context == "" {
		boundary := 0
		if !prefix {
			boundary = len(block.text)
		}
		position, ok := block.boundaries[boundary]
		if !ok {
			return nil
		}
		return []uint{position}
	}
	result := make([]uint, 0)
	searchFrom := 0
	for searchFrom <= len(block.text) {
		relative := strings.Index(block.text[searchFrom:], context)
		if relative < 0 {
			break
		}
		start := searchFrom + relative
		boundary := start
		if prefix {
			boundary += len(context)
		}
		if position, ok := block.boundaries[boundary]; ok {
			result = append(result, position)
		}
		searchFrom = start + 1
	}
	return result
}

func collectCommentAnchorTextBlocks(node map[string]any, nodeStart uint, result *[]commentAnchorTextBlock) {
	nodeType, _ := node["type"].(string)
	content, _ := node["content"].([]any)
	if nodeType == "paragraph" || nodeType == "heading" {
		if block, ok := buildCommentAnchorTextBlock(content, nodeStart+1); ok {
			*result = append(*result, block)
		}
		return
	}
	childStart := nodeStart
	if nodeType != "doc" {
		childStart++
	}
	for _, rawChild := range content {
		child, ok := rawChild.(map[string]any)
		if !ok {
			return
		}
		collectCommentAnchorTextBlocks(child, childStart, result)
		childStart += prosemirrorNodeSize(child)
	}
}

func buildCommentAnchorTextBlock(content []any, contentStart uint) (commentAnchorTextBlock, bool) {
	var text strings.Builder
	boundaries := map[int]uint{0: contentStart}
	position := contentStart
	for _, rawChild := range content {
		child, ok := rawChild.(map[string]any)
		if !ok {
			return commentAnchorTextBlock{}, false
		}
		nodeType, _ := child["type"].(string)
		size := prosemirrorNodeSize(child)
		switch nodeType {
		case "text":
			value, ok := child["text"].(string)
			if !ok {
				return commentAnchorTextBlock{}, false
			}
			for _, character := range value {
				text.WriteRune(character)
				position += uint(len(utf16.Encode([]rune{character})))
				boundaries[text.Len()] = position
			}
		case "mention":
			attrs, _ := child["attrs"].(map[string]any)
			label := firstNonemptyString(attrs["label"], attrs["username"], attrs["id"])
			if label == "" {
				return commentAnchorTextBlock{}, false
			}
			text.WriteString("@" + label)
			position += size
			boundaries[text.Len()] = position
		case "hardBreak":
			text.WriteByte('\n')
			position += size
			boundaries[text.Len()] = position
		default:
			return commentAnchorTextBlock{}, false
		}
	}
	return commentAnchorTextBlock{text: text.String(), boundaries: boundaries}, true
}

func commentAnchorCandidates(block commentAnchorTextBlock, snapshot SelectionSnapshotV1) []commentAnchorCandidate {
	if snapshot.Quote == "" {
		return nil
	}
	result := make([]commentAnchorCandidate, 0)
	searchFrom := 0
	for searchFrom <= len(block.text) {
		relative := strings.Index(block.text[searchFrom:], snapshot.Quote)
		if relative < 0 {
			break
		}
		start := searchFrom + relative
		end := start + len(snapshot.Quote)
		from, fromValid := block.boundaries[start]
		to, toValid := block.boundaries[end]
		if fromValid && toValid && to > from {
			result = append(result, commentAnchorCandidate{
				from:          from,
				to:            to,
				prefixMatches: snapshot.Prefix == "" || strings.HasSuffix(block.text[:start], snapshot.Prefix),
				suffixMatches: snapshot.Suffix == "" || strings.HasPrefix(block.text[end:], snapshot.Suffix),
			})
		}
		searchFrom = start + 1
	}
	return result
}
