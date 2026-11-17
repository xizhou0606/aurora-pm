package toolruntime

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
)

func TestPMQualityResolutionReasonSchemasAcceptKnownAliases(t *testing.T) {
	names := []string{"quality.bug.create", "quality.bug.update", "quality.bug.status.update", "quality.bug.batch_create"}
	catalog, err := NewPMCatalog(PMActivation{ExecutableToolNames: names})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			definition, _ := catalog.Find(name)
			if definition.Version != "1.2.0" {
				t.Fatalf("tightened schema version=%q", definition.Version)
			}
			var schema map[string]any
			if err := json.Unmarshal(definition.InputSchema, &schema); err != nil {
				t.Fatal(err)
			}
			properties := schema["properties"].(map[string]any)
			if name == "quality.bug.batch_create" {
				properties = properties["bugs"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
			}
			var reasons []string
			for _, value := range properties["resolution_reason"].(map[string]any)["enum"].([]any) {
				reasons = append(reasons, value.(string))
			}
			if !reflect.DeepEqual(reasons, quality.BugResolutionReasonInputs()) {
				t.Fatalf("enum=%v", reasons)
			}
			compiled, err := compileExecutionSchema(name, definition.InputSchema)
			if err != nil {
				t.Fatal(err)
			}
			for _, reason := range append(quality.BugResolutionReasonInputs(), "随便处理", "invalid_reason") {
				input := map[string]any{"id": float64(1), "requirement_id": float64(11), "title": "test", "status": "review", "resolution_reason": reason}
				switch name {
				case "quality.bug.create":
					delete(input, "id")
				case "quality.bug.status.update":
					delete(input, "requirement_id")
					delete(input, "title")
				case "quality.bug.batch_create":
					delete(input, "id")
					input["client_id"] = "first"
					input = map[string]any{"bugs": []any{input}}
				}
				if accepted := compiled.Validate(input) == nil; accepted != quality.ValidBugResolutionReason(quality.NormalizeBugResolutionReason(reason)) {
					t.Errorf("schema accepted=%v reason=%q", accepted, reason)
				}
			}
		})
	}
	unchanged, _ := catalog.Find("quality.bug.get")
	if unchanged.Version != PMCatalogVersion {
		t.Fatalf("unrelated Tool version changed: %q", unchanged.Version)
	}
}
