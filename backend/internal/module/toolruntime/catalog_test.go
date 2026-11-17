package toolruntime

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestCatalogProjectsIdenticalMCPAndCLIContracts(t *testing.T) {
	definition := validDefinition()
	catalog, err := NewCatalog([]Definition{definition})
	if err != nil {
		t.Fatal(err)
	}

	mcp, err := catalog.Manifest(DeliveryMCP)
	if err != nil {
		t.Fatal(err)
	}
	cli, err := catalog.Manifest(DeliveryCLI)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateDeliveryParity(mcp, cli); err != nil {
		t.Fatalf("canonical projections must have parity: %v", err)
	}
	if len(mcp) != 1 || mcp[0].Name != definition.Name || mcp[0].InputSchemaHash == "" {
		t.Fatalf("unexpected manifest: %+v", mcp)
	}
}

func TestCatalogDoesNotAdvertiseDraftDefinitions(t *testing.T) {
	active := validDefinition()
	draft := validDefinition()
	draft.Name = "project.create"
	draft.Lifecycle = LifecycleDraft
	deprecated := validDefinition()
	deprecated.Name = "project.search"
	deprecated.Lifecycle = LifecycleDeprecated
	deprecated.Deprecation = validDeprecation()

	catalog, err := NewCatalog([]Definition{draft, deprecated, active})
	if err != nil {
		t.Fatal(err)
	}
	if definitions := catalog.Definitions(); len(definitions) != 3 {
		t.Fatalf("catalog definitions=%d, want 3", len(definitions))
	}
	if found, ok := catalog.Find(draft.Name); !ok || found.Lifecycle != LifecycleDraft {
		t.Fatalf("draft definition not retained: %+v, found=%v", found, ok)
	}

	mcp, err := catalog.Manifest(DeliveryMCP)
	if err != nil {
		t.Fatal(err)
	}
	cli, err := catalog.Manifest(DeliveryCLI)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateDeliveryParity(mcp, cli); err != nil {
		t.Fatalf("published definitions must retain parity: %v", err)
	}
	if len(mcp) != 2 || mcp[0].Lifecycle != LifecycleActive || mcp[1].Lifecycle != LifecycleDeprecated {
		t.Fatalf("published manifest=%+v, want active and deprecated only", mcp)
	}
	if mcp[1].Deprecation != deprecated.Deprecation || mcp[1].AuditEvent != deprecated.AuditEvent {
		t.Fatalf("deprecated delivery metadata drifted: %+v", mcp[1])
	}
}

func TestCatalogOwnsImmutableDefinitions(t *testing.T) {
	definition := validDefinition()
	catalog, err := NewCatalog([]Definition{definition})
	if err != nil {
		t.Fatal(err)
	}
	definition.InputSchema[0] = '['

	found, ok := catalog.Find(definition.Name)
	if !ok || !json.Valid(found.InputSchema) {
		t.Fatalf("catalog retained caller-owned schema memory: %s", found.InputSchema)
	}
	found.InputSchema[0] = '['
	again, _ := catalog.Find(definition.Name)
	if !json.Valid(again.InputSchema) {
		t.Fatalf("Find exposed catalog-owned schema memory: %s", again.InputSchema)
	}
}

func TestCatalogRejectsInvalidDefinitionsFailClosed(t *testing.T) {
	tests := map[string]func(*Definition){
		"non canonical name":       func(value *Definition) { value.Name = "project" },
		"invalid version":          func(value *Definition) { value.Version = "v1" },
		"missing summary":          func(value *Definition) { value.Summary = "" },
		"unsupported lifecycle":    func(value *Definition) { value.Lifecycle = "disabled" },
		"missing audit event":      func(value *Definition) { value.AuditEvent = "" },
		"invalid audit event":      func(value *Definition) { value.AuditEvent = "TOOL_INVOKED" },
		"missing identity gate":    func(value *Definition) { value.Authorization.IdentityRequired = false },
		"missing workspace gate":   func(value *Definition) { value.Authorization.WorkspaceRequired = false },
		"missing grant gate":       func(value *Definition) { value.Authorization.GrantRequired = false },
		"invalid grant scope":      func(value *Definition) { value.Authorization.GrantScope = "admin" },
		"invalid input schema":     func(value *Definition) { value.InputSchema = json.RawMessage(`{"type":"array"}`) },
		"invalid output schema":    func(value *Definition) { value.OutputSchema = json.RawMessage(`null`) },
		"unsupported UAT policy":   func(value *Definition) { value.UAT = "always" },
		"read with change set":     func(value *Definition) { value.ChangeSet = ChangeSetRequired },
		"mutation without changes": func(value *Definition) { value.Risk = RiskMutation },
		"unsupported risk":         func(value *Definition) { value.Risk = "medium" },
		"active with deprecation":  func(value *Definition) { value.Deprecation = validDeprecation() },
		"draft with deprecation": func(value *Definition) {
			value.Lifecycle = LifecycleDraft
			value.Deprecation = validDeprecation()
		},
		"deprecated without data": func(value *Definition) { value.Lifecycle = LifecycleDeprecated },
		"deprecated invalid since": func(value *Definition) {
			value.Lifecycle = LifecycleDeprecated
			value.Deprecation = validDeprecation()
			value.Deprecation.SinceVersion = "v1"
		},
		"deprecated missing notice": func(value *Definition) {
			value.Lifecycle = LifecycleDeprecated
			value.Deprecation = validDeprecation()
			value.Deprecation.Notice = ""
		},
		"deprecated bad replacement": func(value *Definition) {
			value.Lifecycle = LifecycleDeprecated
			value.Deprecation = validDeprecation()
			value.Deprecation.Replacement = "PROJECT_LIST"
		},
		"deprecated self replacement": func(value *Definition) {
			value.Lifecycle = LifecycleDeprecated
			value.Deprecation = validDeprecation()
			value.Deprecation.Replacement = value.Name
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			definition := validDefinition()
			mutate(&definition)
			if _, err := NewCatalog([]Definition{definition}); !errors.Is(err, ErrDefinitionInvalid) {
				t.Fatalf("err=%v, want ErrDefinitionInvalid", err)
			}
		})
	}
}

func TestCatalogRejectsDuplicateNamesAndUnknownDelivery(t *testing.T) {
	definition := validDefinition()
	if _, err := NewCatalog([]Definition{definition, definition}); !errors.Is(err, ErrToolDuplicate) {
		t.Fatalf("duplicate err=%v", err)
	}
	catalog, err := NewCatalog([]Definition{definition})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Manifest("http"); !errors.Is(err, ErrDeliveryInvalid) {
		t.Fatalf("delivery err=%v", err)
	}
}

func TestCatalogAcceptsPreOneSemanticVersion(t *testing.T) {
	definition := validDefinition()
	definition.Version = "0.1.0"
	if _, err := NewCatalog([]Definition{definition}); err != nil {
		t.Fatalf("valid semantic version rejected: %v", err)
	}
}

func TestCatalogAcceptsDeprecatedToolWithoutReplacement(t *testing.T) {
	definition := validDefinition()
	definition.Lifecycle = LifecycleDeprecated
	definition.Deprecation = validDeprecation()
	definition.Deprecation.Replacement = ""
	if _, err := NewCatalog([]Definition{definition}); err != nil {
		t.Fatalf("optional replacement rejected: %v", err)
	}
}

func TestDeliveryParityRejectsOneSidedAndDriftedTools(t *testing.T) {
	catalog, err := NewCatalog([]Definition{validDefinition()})
	if err != nil {
		t.Fatal(err)
	}
	mcp, _ := catalog.Manifest(DeliveryMCP)
	if err := ValidateDeliveryParity(mcp, nil); !errors.Is(err, ErrDeliveryParity) {
		t.Fatalf("one-sided parity err=%v", err)
	}

	cli := append([]DeliveryContract(nil), mcp...)
	cli[0].Version = "2.0.0"
	if err := ValidateDeliveryParity(mcp, cli); !errors.Is(err, ErrDeliveryParity) {
		t.Fatalf("version drift parity err=%v", err)
	}

	cli = append([]DeliveryContract(nil), mcp...)
	cli[0].Authorization.GrantScope = "project:write"
	if err := ValidateDeliveryParity(mcp, cli); !errors.Is(err, ErrDeliveryParity) {
		t.Fatalf("gate drift parity err=%v", err)
	}

	cli = append([]DeliveryContract(nil), mcp...)
	cli[0].AuditEvent = "tool.project.read"
	if err := ValidateDeliveryParity(mcp, cli); !errors.Is(err, ErrDeliveryParity) {
		t.Fatalf("audit drift parity err=%v", err)
	}

	deprecated := validDefinition()
	deprecated.Name = "project.search"
	deprecated.Lifecycle = LifecycleDeprecated
	deprecated.Deprecation = validDeprecation()
	deprecatedCatalog, err := NewCatalog([]Definition{deprecated})
	if err != nil {
		t.Fatal(err)
	}
	deprecatedMCP, _ := deprecatedCatalog.Manifest(DeliveryMCP)
	deprecatedCLI := append([]DeliveryContract(nil), deprecatedMCP...)
	deprecatedCLI[0].Deprecation.Replacement = "project.find"
	if err := ValidateDeliveryParity(deprecatedMCP, deprecatedCLI); !errors.Is(err, ErrDeliveryParity) {
		t.Fatalf("deprecation drift parity err=%v", err)
	}
}

func TestSchemaHashIgnoresObjectKeyOrderAndFormatting(t *testing.T) {
	first := validDefinition()
	second := validDefinition()
	second.InputSchema = json.RawMessage(`{ "required": ["workspace_id"], "type": "object" }`)

	firstCatalog, _ := NewCatalog([]Definition{first})
	secondCatalog, _ := NewCatalog([]Definition{second})
	firstManifest, _ := firstCatalog.Manifest(DeliveryMCP)
	secondManifest, _ := secondCatalog.Manifest(DeliveryCLI)
	if firstManifest[0].InputSchemaHash != secondManifest[0].InputSchemaHash {
		t.Fatalf("canonical schema hashes differ: %s != %s", firstManifest[0].InputSchemaHash, secondManifest[0].InputSchemaHash)
	}
}

func TestSchemaHashIgnoresNestedObjectKeyOrderAndFormatting(t *testing.T) {
	first := validDefinition()
	first.InputSchema = json.RawMessage(`{
		"type":"object",
		"properties":{"filter":{"type":"object","properties":{"status":{"type":"string"},"limit":{"type":"integer"}}}}
	}`)
	second := validDefinition()
	second.InputSchema = json.RawMessage(`{"properties":{"filter":{"properties":{"limit":{"type":"integer"},"status":{"type":"string"}},"type":"object"}},"type":"object"}`)

	firstCatalog, err := NewCatalog([]Definition{first})
	if err != nil {
		t.Fatal(err)
	}
	secondCatalog, err := NewCatalog([]Definition{second})
	if err != nil {
		t.Fatal(err)
	}
	firstManifest, _ := firstCatalog.Manifest(DeliveryMCP)
	secondManifest, _ := secondCatalog.Manifest(DeliveryCLI)
	if firstManifest[0].InputSchemaHash != secondManifest[0].InputSchemaHash {
		t.Fatalf("nested canonical schema hashes differ: %s != %s", firstManifest[0].InputSchemaHash, secondManifest[0].InputSchemaHash)
	}
}

func validDefinition() Definition {
	return Definition{
		Name: "project.list", Version: "1.0.0", Summary: "List workspace projects",
		Lifecycle:    LifecycleActive,
		InputSchema:  json.RawMessage(`{"type":"object","required":["workspace_id"]}`),
		OutputSchema: json.RawMessage(`{"type":"object","required":["items"]}`),
		Authorization: AuthorizationContract{
			IdentityRequired: true, WorkspaceRequired: true, GrantRequired: true, GrantScope: "project:read",
		},
		Risk: RiskRead, ChangeSet: ChangeSetForbidden, UAT: UATWithinGrantWindow,
		AuditEvent: "tool.project.list",
	}
}

func validDeprecation() DeprecationContract {
	return DeprecationContract{
		SinceVersion: "1.1.0",
		Replacement:  "project.list",
		Notice:       "Use project.list instead",
	}
}
