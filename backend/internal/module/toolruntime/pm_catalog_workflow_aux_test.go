package toolruntime

import (
	"encoding/json"
	"testing"
)

func TestWorkflowAuxReadActivationHasStrictSchemasAndPreservesCatalogLedger(t *testing.T) {
	names := []string{
		"workflow.opinion.list",
		"workflow.test_case_artifact.list",
		"workflow.test_report_artifact.list",
	}
	catalog, err := NewPMCatalog(PMActivation{ExecutableToolNames: names})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(catalog.Definitions()); got != 132 {
		t.Fatalf("definitions=%d", got)
	}
	for _, name := range names {
		definition, ok := catalog.Find(name)
		if !ok || definition.Lifecycle != LifecycleActive {
			t.Fatalf("%s definition=%+v ok=%v", name, definition, ok)
		}
		var input, output map[string]any
		if json.Unmarshal(definition.InputSchema, &input) != nil || input["additionalProperties"] != false {
			t.Fatalf("%s input=%s", name, definition.InputSchema)
		}
		if json.Unmarshal(definition.OutputSchema, &output) != nil || output["additionalProperties"] != false {
			t.Fatalf("%s output=%s", name, definition.OutputSchema)
		}
		properties, _ := output["properties"].(map[string]any)
		items, _ := properties["items"].(map[string]any)
		itemSchema, _ := items["items"].(map[string]any)
		if itemSchema["additionalProperties"] != false {
			t.Fatalf("%s item schema=%v", name, itemSchema)
		}
	}
}

func TestWorkflowInstanceContractsRemainCoveredByExistingReadSlice(t *testing.T) {
	catalog, err := NewPMCatalog(PMActivation{ExecutableToolNames: []string{"workflow.instance.list", "workflow.instance.get"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"workflow.instance.list", "workflow.instance.get"} {
		definition, ok := catalog.Find(name)
		if !ok || definition.Lifecycle != LifecycleActive {
			t.Fatalf("existing workflow handler %s is not active", name)
		}
	}
	definition, _ := catalog.Find("workflow.instance.get")
	var output map[string]any
	if json.Unmarshal(definition.OutputSchema, &output) != nil {
		t.Fatal("invalid workflow.instance.get output schema")
	}
	properties, _ := output["properties"].(map[string]any)
	for _, field := range []string{"id", "workspace_id", "current_node_ids", "node_plans"} {
		if _, ok := properties[field]; !ok {
			t.Fatalf("workflow.instance.get missing contract field %s", field)
		}
	}
}
