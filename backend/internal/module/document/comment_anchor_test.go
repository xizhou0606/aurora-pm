package document

import (
	"encoding/json"
	"testing"
)

func TestSelectionSnapshotMatchesCanonicalProseMirrorRangeAndContext(t *testing.T) {
	content := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"这个方案可行"}]}]}`)
	valid := SelectionSnapshotV1{DocumentRevision: 1, From: 3, To: 5, Quote: "方案", Prefix: "这个", Suffix: "可行"}
	if !selectionSnapshotMatches(content, valid) {
		t.Fatal("expected canonical selection snapshot to match")
	}
	wrongQuote := valid
	wrongQuote.Quote = "方安"
	if selectionSnapshotMatches(content, wrongQuote) {
		t.Fatal("mismatched quote must not anchor")
	}
	wrongContext := valid
	wrongContext.Prefix = "其他"
	if selectionSnapshotMatches(content, wrongContext) {
		t.Fatal("mismatched context must not anchor")
	}
}

func TestSelectionSnapshotUsesProseMirrorUTF16Positions(t *testing.T) {
	content := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"A😀B"}]}]}`)
	if !selectionSnapshotMatches(content, SelectionSnapshotV1{DocumentRevision: 1, From: 2, To: 4, Quote: "😀", Prefix: "A", Suffix: "B"}) {
		t.Fatal("emoji must consume two ProseMirror UTF-16 positions")
	}
}

func TestSelectionSnapshotAcrossSchemaV2Containers(t *testing.T) {
	for _, nodeType := range []string{"table", "tableRow", "tableCell", "tableHeader", "taskList", "taskItem", "callout", "codeBlock"} {
		t.Run(nodeType, func(t *testing.T) {
			node := map[string]any{"type": nodeType}
			if size := prosemirrorNodeSize(node); size != 2 {
				t.Fatalf("empty %s size = %d, want 2", nodeType, size)
			}
		})
	}

	content := json.RawMessage(`{"type":"doc","content":[{"type":"codeBlock","content":[{"type":"text","text":"A😀"}]},{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"前选中后"}]}]}]}]},{"type":"paragraph","content":[{"type":"text","text":"正文"}]}]}`)
	for _, snapshot := range []SelectionSnapshotV1{
		{DocumentRevision: 2, From: 10, To: 12, Quote: "选中", Prefix: "前", Suffix: "后"},
		{DocumentRevision: 2, From: 18, To: 20, Quote: "正文"},
	} {
		anchor := CommentAnchor{Kind: CommentAnchorSelectionSnapshotV1, Selection: &snapshot}
		if _, err := normalizeCommentAnchor(anchor, Document{Revision: 2, Content: content}); err != nil {
			t.Fatalf("valid v2 selection %+v rejected: %v", snapshot, err)
		}
		stale := snapshot
		stale.From = 1
		stale.To = 3
		relocated, ok := relocateSelectionSnapshot(content, stale)
		if !ok || relocated.From != snapshot.From || relocated.To != snapshot.To {
			t.Fatalf("relocated %+v, want %+v", relocated, snapshot)
		}
		wrong := snapshot
		wrong.From++
		wrong.To++
		if selectionSnapshotMatches(content, wrong) {
			t.Fatal("wrong v2 position must still be rejected")
		}
	}
}

func TestSelectionSnapshotCountsEmptyTextBlocksAsNonLeafNodes(t *testing.T) {
	content := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"A"}]},{"type":"paragraph"},{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"BC"}]}]}`)
	if !selectionSnapshotMatches(content, SelectionSnapshotV1{
		DocumentRevision: 1,
		From:             6,
		To:               7,
		Quote:            "B",
		Suffix:           "C",
	}) {
		t.Fatal("empty paragraph must still consume two ProseMirror positions")
	}
}

func TestSelectionSnapshotRejectsCrossBlockRange(t *testing.T) {
	content := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"first"}]},{"type":"paragraph","content":[{"type":"text","text":"second"}]}]}`)
	if selectionSnapshotMatches(content, SelectionSnapshotV1{DocumentRevision: 1, From: 1, To: 10, Quote: "firstsecond"}) {
		t.Fatal("snapshot without a stable cross-block mapping must fail closed")
	}
}

func TestNearestCommentAnchorCandidateRejectsDistanceTies(t *testing.T) {
	_, ok := nearestCommentAnchorCandidate([]commentAnchorCandidate{
		{from: 5, to: 9},
		{from: 11, to: 15},
		{from: 40, to: 44},
	}, SelectionSnapshotV1{From: 8, To: 12})
	if ok {
		t.Fatal("equidistant context candidates must remain ambiguous")
	}
}
