package designreview

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestParseDOMNormalizationConfig(t *testing.T) {
	state, err := ParseDOMNormalizationConfig(json.RawMessage(`{"enabled":true}`), "web")
	if err != nil || string(state) != `{"enabled":true,"status":"pending"}` {
		t.Fatalf("state=%s err=%v", state, err)
	}
	if _, err := ParseDOMNormalizationConfig(json.RawMessage(`{"enabled":true}`), "mobile"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("mobile normalization err=%v", err)
	}
	if _, err := ParseDOMNormalizationConfig(json.RawMessage(`{"enabled":false,"inventory":{}}`), "web"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown config field err=%v", err)
	}
}

func TestParseDOMNormalizationManifestDerivesCompleteness(t *testing.T) {
	complete := []byte(`{
		"requested":true,
		"report":{
			"plan":{"model":"vision","prompt_version":"v1","already_aligned":false,"mismatch_detected":true,"summary":"aligned","skipped":[]},
			"operations":[{"type":"set_text","target_id":"el-1","result":"applied","reason":null}],
			"counts":{"applied":1,"skipped":0}
		}
	}`)
	state, err := ParseDOMNormalizationManifest(complete)
	if err != nil {
		t.Fatal(err)
	}
	var decoded domNormalizationState
	if json.Unmarshal(state, &decoded) != nil || decoded.Status != "complete" || decoded.Report.Counts.Applied != 1 {
		t.Fatalf("state=%s", state)
	}

	partial := []byte(`{
		"requested":true,
		"report":{
			"plan":{"model":"vision","prompt_version":"v1","already_aligned":false,"mismatch_detected":true,"summary":"partial","skipped":[{"figma_source":"table","target_id":null,"reason":"no target"}]},
			"operations":[],"counts":{"applied":0,"skipped":1}
		}
	}`)
	state, err = ParseDOMNormalizationManifest(partial)
	if err != nil || json.Unmarshal(state, &decoded) != nil || decoded.Status != "partial" || decoded.Report.Counts.Skipped != 1 {
		t.Fatalf("state=%s err=%v", state, err)
	}

	invalidCounts := []byte(`{
		"requested":true,
		"report":{
			"plan":{"model":"vision","prompt_version":"v1","already_aligned":false,"mismatch_detected":true,"summary":"bad","skipped":[]},
			"operations":[],"counts":{"applied":1,"skipped":0}
		}
	}`)
	if _, err := ParseDOMNormalizationManifest(invalidCounts); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid counts err=%v", err)
	}
}
