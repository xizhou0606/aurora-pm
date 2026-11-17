package native

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"testing"
)

func TestTypedComparisonsDoNotConfuseDisplayAndIdentity(t *testing.T) {
	field := bitable.Field{Type: bitable.FieldUser}
	a := []bitable.UserRef{{ID: 10, Name: "old"}, {ID: 20, Name: "second"}}
	b := []bitable.UserRef{{ID: 20, Name: "changed"}, {ID: 10, Name: "new"}}
	if !equalValue(field, a, b) {
		t.Fatal("user equality must compare canonical ID sets")
	}
	if equalValue(field, a, []bitable.UserRef{{ID: 30, Name: "old"}}) {
		t.Fatal("display name is not identity")
	}
	date := bitable.Field{Type: bitable.FieldDate}
	if compareValue(date, "2026-09-07T16:00:00+08:00", "2026-09-07T08:00:00Z") != 0 {
		t.Fatal("same instant differs")
	}
	if compareValue(date, "2026-09-07T15:00:00+08:00", "2026-09-07T08:00:00Z") >= 0 {
		t.Fatal("offset timestamp sorted lexically")
	}
	if emptyValue(false) || emptyValue(float64(0)) {
		t.Fatal("false and zero are not empty")
	}
	if !emptyValue([]any{}) || !emptyValue(nil) {
		t.Fatal("empty values not recognized")
	}
}
