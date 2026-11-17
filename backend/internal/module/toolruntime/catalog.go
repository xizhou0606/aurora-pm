package toolruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	ErrDefinitionInvalid = errors.New("tool definition is invalid")
	ErrToolDuplicate     = errors.New("tool name is duplicated")
	ErrDeliveryInvalid   = errors.New("tool delivery channel is invalid")
	ErrDeliveryParity    = errors.New("MCP and CLI tool delivery contracts differ")

	toolNamePattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]*(\.[a-z][a-z0-9_-]*)+$`)
	versionPattern    = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	grantScopePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]*:[a-z][a-z0-9._-]*$`)
)

// Catalog is an immutable, validated registry. Both supported transports are
// projections of this one registry rather than separately maintained lists.
type Catalog struct {
	definitions []Definition
	byName      map[string]Definition
}

func NewCatalog(definitions []Definition) (*Catalog, error) {
	catalog := &Catalog{
		definitions: make([]Definition, 0, len(definitions)),
		byName:      make(map[string]Definition, len(definitions)),
	}
	for _, definition := range definitions {
		if err := validateDefinition(definition); err != nil {
			return nil, err
		}
		if _, exists := catalog.byName[definition.Name]; exists {
			return nil, fmt.Errorf("%w: %s", ErrToolDuplicate, definition.Name)
		}
		definition = cloneDefinition(definition)
		catalog.definitions = append(catalog.definitions, definition)
		catalog.byName[definition.Name] = definition
	}
	sort.Slice(catalog.definitions, func(i, j int) bool {
		return catalog.definitions[i].Name < catalog.definitions[j].Name
	})
	return catalog, nil
}

func (catalog *Catalog) Definitions() []Definition {
	result := make([]Definition, 0, len(catalog.definitions))
	for _, definition := range catalog.definitions {
		result = append(result, cloneDefinition(definition))
	}
	return result
}

func (catalog *Catalog) Find(name string) (Definition, bool) {
	definition, ok := catalog.byName[name]
	return cloneDefinition(definition), ok
}

// Manifest accepts a channel only to reject unsupported delivery paths. MCP
// and CLI receive identical records from the same definitions by construction.
func (catalog *Catalog) Manifest(channel DeliveryChannel) ([]DeliveryContract, error) {
	if channel != DeliveryMCP && channel != DeliveryCLI {
		return nil, ErrDeliveryInvalid
	}
	result := make([]DeliveryContract, 0, len(catalog.definitions))
	for _, definition := range catalog.definitions {
		if definition.Lifecycle == LifecycleDraft {
			continue
		}
		inputHash, _ := schemaDigest(definition.InputSchema)
		outputHash, _ := schemaDigest(definition.OutputSchema)
		result = append(result, DeliveryContract{
			Name: definition.Name, Version: definition.Version, Summary: definition.Summary,
			Lifecycle: definition.Lifecycle, Deprecation: definition.Deprecation,
			InputSchemaHash: inputHash, OutputSchemaHash: outputHash,
			Authorization: definition.Authorization, Risk: definition.Risk,
			ChangeSet: definition.ChangeSet, UAT: definition.UAT, AuditEvent: definition.AuditEvent,
		})
	}
	return result, nil
}

// ValidateDeliveryParity is intended for adapter and CI contract tests. It
// fails closed for missing, duplicated, or different delivery records.
func ValidateDeliveryParity(mcp, cli []DeliveryContract) error {
	mcpByName, err := indexedContracts(mcp)
	if err != nil {
		return err
	}
	cliByName, err := indexedContracts(cli)
	if err != nil {
		return err
	}
	if len(mcpByName) != len(cliByName) {
		return fmt.Errorf("%w: tool counts differ", ErrDeliveryParity)
	}
	for name, mcpContract := range mcpByName {
		cliContract, exists := cliByName[name]
		if !exists {
			return fmt.Errorf("%w: %s is missing from CLI", ErrDeliveryParity, name)
		}
		if mcpContract != cliContract {
			return fmt.Errorf("%w: %s contract drift", ErrDeliveryParity, name)
		}
	}
	return nil
}

func indexedContracts(contracts []DeliveryContract) (map[string]DeliveryContract, error) {
	result := make(map[string]DeliveryContract, len(contracts))
	for _, contract := range contracts {
		if strings.TrimSpace(contract.Name) == "" {
			return nil, fmt.Errorf("%w: delivery contract name is required", ErrDeliveryParity)
		}
		if _, exists := result[contract.Name]; exists {
			return nil, fmt.Errorf("%w: duplicate delivery contract %s", ErrDeliveryParity, contract.Name)
		}
		result[contract.Name] = contract
	}
	return result, nil
}

func validateDefinition(definition Definition) error {
	switch {
	case !toolNamePattern.MatchString(definition.Name):
		return invalidDefinition(definition.Name, "name must be a dotted canonical identifier")
	case !versionPattern.MatchString(definition.Version):
		return invalidDefinition(definition.Name, "version must be semantic major.minor.patch")
	case strings.TrimSpace(definition.Summary) == "":
		return invalidDefinition(definition.Name, "summary is required")
	case definition.Lifecycle != LifecycleDraft && definition.Lifecycle != LifecycleActive && definition.Lifecycle != LifecycleDeprecated:
		return invalidDefinition(definition.Name, "lifecycle is unsupported")
	case !toolNamePattern.MatchString(definition.AuditEvent):
		return invalidDefinition(definition.Name, "audit event must be a dotted canonical identifier")
	case !definition.Authorization.IdentityRequired:
		return invalidDefinition(definition.Name, "identity gate is required")
	case !definition.Authorization.WorkspaceRequired:
		return invalidDefinition(definition.Name, "workspace gate is required")
	case !definition.Authorization.GrantRequired:
		return invalidDefinition(definition.Name, "external grant gate is required")
	case !grantScopePattern.MatchString(definition.Authorization.GrantScope):
		return invalidDefinition(definition.Name, "grant scope is invalid")
	case definition.UAT != UATBlocked && definition.UAT != UATWithinGrantWindow:
		return invalidDefinition(definition.Name, "UAT policy is unsupported")
	}
	if err := validateDeprecation(definition); err != nil {
		return err
	}
	if _, err := schemaDigest(definition.InputSchema); err != nil {
		return invalidDefinition(definition.Name, "input schema must be an object JSON Schema")
	}
	if _, err := schemaDigest(definition.OutputSchema); err != nil {
		return invalidDefinition(definition.Name, "output schema must be an object JSON Schema")
	}
	switch definition.Risk {
	case RiskRead:
		if definition.ChangeSet != ChangeSetForbidden {
			return invalidDefinition(definition.Name, "read tools cannot create change sets")
		}
	case RiskMutation, RiskHighImpact:
		if definition.ChangeSet != ChangeSetRequired {
			return invalidDefinition(definition.Name, "mutating tools require change sets")
		}
	default:
		return invalidDefinition(definition.Name, "risk level is unsupported")
	}
	return nil
}

func validateDeprecation(definition Definition) error {
	metadata := definition.Deprecation
	if definition.Lifecycle != LifecycleDeprecated {
		if metadata != (DeprecationContract{}) {
			return invalidDefinition(definition.Name, "deprecation metadata is allowed only for deprecated tools")
		}
		return nil
	}
	if !versionPattern.MatchString(metadata.SinceVersion) || strings.TrimSpace(metadata.Notice) == "" {
		return invalidDefinition(definition.Name, "deprecated tools require valid since_version and notice metadata")
	}
	if metadata.Replacement != "" {
		if !toolNamePattern.MatchString(metadata.Replacement) {
			return invalidDefinition(definition.Name, "replacement tool name is invalid")
		}
		if metadata.Replacement == definition.Name {
			return invalidDefinition(definition.Name, "replacement tool cannot reference itself")
		}
	}
	return nil
}

func schemaDigest(raw json.RawMessage) (string, error) {
	var value any
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return "", ErrDefinitionInvalid
	}
	schema, ok := value.(map[string]any)
	if !ok {
		return "", ErrDefinitionInvalid
	}
	schemaType, ok := schema["type"].(string)
	if !ok || schemaType != "object" {
		return "", ErrDefinitionInvalid
	}
	canonical, err := json.Marshal(schema)
	if err != nil {
		return "", ErrDefinitionInvalid
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func invalidDefinition(name, reason string) error {
	if name == "" {
		name = "<unnamed>"
	}
	return fmt.Errorf("%w: %s: %s", ErrDefinitionInvalid, name, reason)
}

func cloneDefinition(definition Definition) Definition {
	definition.InputSchema = append(json.RawMessage(nil), definition.InputSchema...)
	definition.OutputSchema = append(json.RawMessage(nil), definition.OutputSchema...)
	return definition
}
