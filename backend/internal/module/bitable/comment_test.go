package bitable

import (
	"strings"
	"testing"
)

func TestCommentTextContract(t *testing.T) {
	key := "550e8400-e29b-41d4-a716-446655440000"
	for _, body := range []string{"", " \n\t", strings.Repeat("中", 4001), "nul\x00"} {
		if ValidateComment(CommentInput{Body: body, OperationID: key}) == nil {
			t.Fatal("invalid comment accepted")
		}
	}
	for _, body := range []string{"line1\nline2", strings.Repeat("中", 4000)} {
		if e := ValidateComment(CommentInput{Body: body, OperationID: key}); e != nil {
			t.Fatal(e)
		}
	}
}
func TestCreatePositionOnlyAndRequiredRevision(t *testing.T) {
	revision := uint64(1)
	p := &CreatePosition{RecordID: "550e8400-e29b-41d4-a716-446655440001", Direction: "after", ExpectedRevision: &revision}
	b := CommandBatch{OperationID: "550e8400-e29b-41d4-a716-446655440000", Atomic: true, Commands: []Command{{Kind: "create", Position: p}}}
	if e := ValidateBatch(b); e != nil {
		t.Fatal(e)
	}
	p.ExpectedRevision = nil
	if ValidateBatch(b) == nil {
		t.Fatal("missing anchor revision")
	}
	p.ExpectedRevision = &revision
	b.Commands[0] = Command{Kind: "update", RecordID: "row", ExpectedRevision: &revision, Values: map[string]any{"field": "text"}, Position: p}
	if ValidateBatch(b) == nil {
		t.Fatal("update accepted create-only position")
	}
}
