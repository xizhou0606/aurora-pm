package workflowtestcase

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	SkillName       = "qa-test-case-design"
	SkillRepository = "https://github.com/QA19816-glitch/QA-agent"
	SkillPath       = "codex-skills/skills/qa-test-case-design"

	// Sample hashes used by tests/fixtures. Runtime accepts any build-synced
	// commit/packageSha256 that matches the provenance contract shape.
	SampleSkillCommit        = "d75606531f6ce414bbcd95fb203033ae86653852"
	SampleSkillPackageSHA256 = "07e9002dfe456988164f237f4cc33acce1a203564d1c778e94b3fe2bbc43e935"

	StructuredResource = "output-templates/structured-test-cases.md"
	AdvancedResource   = "prompts/advanced-test-case-design.md"
	WritingResource    = "references/case-writing-rules.md"
)

var (
	ErrTargetInvalid = errors.New("test case delivery target is invalid")
	ErrOutputInvalid = errors.New("test case output is invalid")
)

type Target struct {
	InstanceID uint   `json:"instance_id"`
	NodeID     string `json:"node_id"`
}

func (target Target) Validate() error {
	target.NodeID = strings.TrimSpace(target.NodeID)
	if target.InstanceID == 0 || target.NodeID == "" || len([]rune(target.NodeID)) > 100 {
		return ErrTargetInvalid
	}
	return nil
}

// ParseTarget reads the server-owned delivery metadata stored with an AI run.
// The target object is strict so malformed or manually altered rows fail closed.
func ParseTarget(inputJSON string) (Target, bool, error) {
	var input map[string]json.RawMessage
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return Target{}, false, fmt.Errorf("%w: decode AI input", ErrTargetInvalid)
	}
	raw, found := input["test_case_delivery"]
	if !found || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return Target{}, false, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var target Target
	if err := decoder.Decode(&target); err != nil {
		return Target{}, true, fmt.Errorf("%w: %v", ErrTargetInvalid, err)
	}
	if err := target.Validate(); err != nil {
		return Target{}, true, err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Target{}, true, fmt.Errorf("%w: %v", ErrTargetInvalid, err)
	}
	target.NodeID = strings.TrimSpace(target.NodeID)
	return target, true, nil
}

func RemoveTarget(input map[string]any) {
	delete(input, "test_case_delivery")
}

func ValidateProvenance(capability string, raw json.RawMessage) error {
	if capability != "qa.test_cases" {
		return nil
	}
	var output struct {
		SkillProvenance *struct {
			Name          string   `json:"name"`
			Repository    string   `json:"repository"`
			Path          string   `json:"path"`
			Commit        string   `json:"commit"`
			PackageSHA256 string   `json:"packageSha256"`
			Mode          string   `json:"mode"`
			Resources     []string `json:"resources"`
		} `json:"skillProvenance"`
	}
	if err := json.Unmarshal(raw, &output); err != nil || output.SkillProvenance == nil {
		return fmt.Errorf("%w: fixed QA skill provenance is missing", ErrOutputInvalid)
	}
	provenance := output.SkillProvenance
	if provenance.Name != SkillName || provenance.Repository != SkillRepository ||
		provenance.Path != SkillPath || provenance.Mode != "advanced" {
		return fmt.Errorf("%w: QA skill provenance does not match the release contract", ErrOutputInvalid)
	}
	if !shaHex(provenance.Commit, 40) || !shaHex(provenance.PackageSHA256, 64) {
		return fmt.Errorf("%w: QA skill provenance commit/package hash is invalid", ErrOutputInvalid)
	}
	expected := map[string]bool{StructuredResource: true, AdvancedResource: true, WritingResource: true}
	if len(provenance.Resources) != len(expected) {
		return fmt.Errorf("%w: QA skill resources do not match the release contract", ErrOutputInvalid)
	}
	for _, resource := range provenance.Resources {
		if !expected[resource] {
			return fmt.Errorf("%w: QA skill resources do not match the release contract", ErrOutputInvalid)
		}
		delete(expected, resource)
	}
	if len(expected) > 0 {
		return fmt.Errorf("%w: QA skill resources do not match the release contract", ErrOutputInvalid)
	}
	return nil
}

func shaHex(value string, size int) bool {
	if len(value) != size {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
