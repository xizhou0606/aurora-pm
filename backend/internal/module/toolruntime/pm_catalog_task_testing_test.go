package toolruntime

import (
	"encoding/json"
	"testing"
)

func TestTaskTestingReadActivationHasStrictSchemasAndPreservesCatalogLedger(t *testing.T) {
	names := []string{
		"task.testing.definition.get", "task.testing.context.get", "task.testing.execution_run.list",
		"task.workflow.kind.list", "task.workflow.status.list", "task.workflow.transition.list",
	}
	catalog, err := NewPMCatalog(PMActivation{ExecutableToolNames: names})
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Definitions()) != 132 || len(PMSourceLedger()) != 132 {
		t.Fatalf("definitions=%d ledger=%d", len(catalog.Definitions()), len(PMSourceLedger()))
	}
	for _, name := range names {
		definition, ok := catalog.Find(name)
		if !ok || definition.Lifecycle != LifecycleActive {
			t.Fatalf("definition %s=%+v exists=%t", name, definition, ok)
		}
		var input, output map[string]any
		if json.Unmarshal(definition.InputSchema, &input) != nil || input["additionalProperties"] != false {
			t.Fatalf("%s input=%s", name, definition.InputSchema)
		}
		if json.Unmarshal(definition.OutputSchema, &output) != nil || output["additionalProperties"] != false {
			t.Fatalf("%s output=%s", name, definition.OutputSchema)
		}
		if name == "task.testing.definition.get" || name == "task.testing.context.get" || name == "task.testing.execution_run.list" {
			if _, ok := input["anyOf"].([]any); !ok {
				t.Fatalf("%s must require task_ref or id: %s", name, definition.InputSchema)
			}
		}
		if name == "task.workflow.kind.list" || name == "task.workflow.status.list" || name == "task.workflow.transition.list" {
			dependent, ok := input["dependentRequired"].(map[string]any)
			if !ok {
				t.Fatalf("%s must declare project_id dependency: %s", name, definition.InputSchema)
			}
			required, ok := dependent["project_id"].([]any)
			if !ok || len(required) != 1 || required[0] != "team_id" {
				t.Fatalf("%s project_id dependency=%v", name, dependent["project_id"])
			}
		}
	}
}
