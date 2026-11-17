package bitable

import "testing"

func TestBatchSafety(t *testing.T) {
	contract := CellContract{RecordID: "record-1", FieldID: "field-1", BaseRevision: 2, LeaseID: "550e8400-e29b-41d4-a716-446655440001", FencingToken: 3, ConnectionID: "connection-1", EditorInstanceID: "editor-1"}
	base := CommandBatch{OperationID: "550e8400-e29b-41d4-a716-446655440000", ExpectedSchemaRevision: 1, Atomic: true, Commands: []Command{{Kind: "update", RecordID: "record-1", Values: map[string]any{"field-1": "你好"}, CellContracts: []CellContract{contract}}}}
	if err := ValidateBatch(base); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*CommandBatch){
		"partial":          func(b *CommandBatch) { b.Atomic = false },
		"empty":            func(b *CommandBatch) { b.Commands = nil },
		"id":               func(b *CommandBatch) { b.OperationID = "x" },
		"schema":           func(b *CommandBatch) { b.ExpectedSchemaRevision = MaxSafeRevision + 1 },
		"duplicate":        func(b *CommandBatch) { b.Commands = append(b.Commands, b.Commands[0]) },
		"missing-contract": func(b *CommandBatch) { b.Commands[0].CellContracts = nil },
		"row-cas-on-cell":  func(b *CommandBatch) { v := uint64(2); b.Commands[0].ExpectedRevision = &v },
		"delete-values":    func(b *CommandBatch) { b.Commands[0].Kind = "delete" },
	} {
		t.Run(name, func(t *testing.T) {
			b := base
			b.Commands = append([]Command(nil), base.Commands...)
			change(&b)
			if ValidateBatch(b) == nil {
				t.Fatal("unsafe batch accepted")
			}
		})
	}
	if err := ValidateBatch(CommandBatch{OperationID: base.OperationID, Atomic: true, Commands: []Command{{Kind: "create", Values: map[string]any{}}}}); err != nil {
		t.Fatal("empty row must be creatable", err)
	}
}

func TestQueryBoundsAndSearch(t *testing.T) {
	q := Query{ViewID: "view-1", Limit: 200, Offset: 10000, Filters: []Filter{{FieldID: "field-1", Operator: "empty"}}, Sorts: []Sort{{FieldID: "field-1", Direction: "asc"}}}
	if err := ValidateQuery(q); err != nil {
		t.Fatal(err)
	}
	if ValidateSearch(Search{Query: q, Text: "跨页"}) != nil {
		t.Fatal("valid search rejected")
	}
	if ValidateSearch(Search{Query: q, Text: " "}) == nil {
		t.Fatal("empty search accepted")
	}
	q.Offset++
	if ValidateQuery(q) == nil {
		t.Fatal("offset budget ignored")
	}
	q.Offset--
	q.Filters[0].Value = "x"
	if ValidateQuery(q) == nil {
		t.Fatal("empty operator with value accepted")
	}
}
