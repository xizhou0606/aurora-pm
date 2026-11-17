package bitable

import (
	"reflect"
	"testing"
)

func TestChoiceColorAndDefaultContract(t *testing.T) {
	base := FieldMutation{Name: "status", Type: FieldSingleSelect, OperationID: "550e8400-e29b-41d4-a716-446655440000", Options: FieldOptions{Choices: []Choice{{ID: "ready", Name: "Ready"}}, DefaultValue: "ready"}}
	for _, color := range []string{"", "neutral", "blue", "green", "yellow", "orange", "red", "purple", "pink"} {
		in := base
		in.Options.Choices = append([]Choice{}, base.Options.Choices...)
		in.Options.Choices[0].Color = color
		if e := ValidateFieldMutation(in); e != nil {
			t.Fatal(color, e)
		}
	}
	for _, tc := range []struct {
		kind  FieldType
		value any
	}{{FieldText, "ready"}, {FieldSingleSelect, "missing"}, {FieldSingleSelect, []string{"ready"}}, {FieldSingleSelect, 42}, {FieldMultipleSelect, "ready"}, {FieldMultipleSelect, []string{"ready", "ready"}}, {FieldMultipleSelect, []string{"missing"}}} {
		in := base
		in.Type = tc.kind
		in.Options.DefaultValue = tc.value
		if e := ValidateFieldMutation(in); e == nil {
			t.Fatalf("invalid default accepted: %+v", tc)
		}
	}
	in := base
	in.Options.Choices = []Choice{{ID: "ready", Name: "Ready", Color: "#ff0000"}}
	if ValidateFieldMutation(in) == nil {
		t.Fatal("arbitrary CSS color accepted")
	}
	in = base
	in.Type = FieldMultipleSelect
	in.Options.DefaultValue = []string{}
	if e := ValidateFieldMutation(in); e != nil {
		t.Fatal("empty multiple default", e)
	}
}
func TestApplyCreateDefaultsCopiesAndPreservesExplicitCells(t *testing.T) {
	fields := []Field{{ID: "single", Type: FieldSingleSelect, Options: FieldOptions{Choices: []Choice{{ID: "a", Name: "A"}}, DefaultValue: "a"}}, {ID: "multi", Type: FieldMultipleSelect, Options: FieldOptions{Choices: []Choice{{ID: "a", Name: "A"}}, DefaultValue: []string{"a"}}}}
	input := map[string]any{"single": nil}
	result, e := ApplyCreateDefaults(fields, input)
	if e != nil {
		t.Fatal(e)
	}
	if result["single"] != nil || !reflect.DeepEqual(result["multi"], []any{"a"}) {
		t.Fatal(result)
	}
	result["multi"].([]any)[0] = "changed"
	if !reflect.DeepEqual(fields[1].Options.DefaultValue, []string{"a"}) || len(input) != 1 {
		t.Fatal("default application mutated caller/schema")
	}
}
