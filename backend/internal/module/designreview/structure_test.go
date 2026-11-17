package designreview

import (
	"encoding/json"
	"testing"
)

func TestStructureValidationAndIdentityAnchors(t *testing.T) {
	design := []byte(`{"id":"1:1","name":"Page","type":"FRAME","bounds":{"x":100,"y":200,"width":1000,"height":800},"children":[{"id":"2:1","name":"Card","type":"FRAME","bounds":{"x":200,"y":300,"width":400,"height":300},"children":[{"id":"3:1","name":"Button","type":"INSTANCE","bounds":{"x":300,"y":400,"width":100,"height":50}}]}]}`)
	implementation := []byte(`{"source":"web","actualUrl":"https://example.test/page","elements":[{"id":"web-1","parent_id":null,"tag":"main","role":null,"text":null,"bounds":{"x":0,"y":0,"width":1000,"height":800},"styles":{},"clipped":false,"structural_fingerprint":"main-stable","control_state":null,"allowed_operations":[]},{"id":"web-2","parent_id":"web-1","tag":"button","role":"button","text":"Save","bounds":{"x":200,"y":200,"width":100,"height":50},"styles":{},"clipped":false,"structural_fingerprint":"button-stable","control_state":null,"allowed_operations":["set_text"]}],"structureTruncated":false,"syntheticMobileTopbar":null,"contentViewport":{"width":1000,"height":800},"finalViewport":{"width":1000,"height":800},"warnings":[]}`)
	if err := ValidateDesignStructure(design); err != nil {
		t.Fatalf("valid design structure rejected: %v", err)
	}
	complete, warnings, err := ValidateImplementationStructure(implementation, "web", 1000, 800)
	if err != nil || !complete || string(warnings) != "[]" {
		t.Fatalf("valid implementation structure rejected: complete=%v warnings=%s err=%v", complete, warnings, err)
	}
	designRegion := &Region{X: 0.2, Y: 0.25, Width: 0.1, Height: 0.1}
	implementationRegion := &Region{X: 0.2, Y: 0.25, Width: 0.1, Height: 0.1}
	designAnchor, implementationAnchor := DeriveIdentityAnchors(design, implementation, designRegion, implementationRegion, 1000, 800)
	if designAnchor == nil || designAnchor.StableID != "3:1" {
		t.Fatalf("design anchor = %+v, want smallest containing node", designAnchor)
	}
	if implementationAnchor == nil || implementationAnchor.StableID != "button-stable" {
		t.Fatalf("implementation anchor = %+v, want smallest containing element", implementationAnchor)
	}
}

func TestImplementationStructureRejectsUnknownFieldsAndOrphanParents(t *testing.T) {
	base := map[string]any{
		"source": "web", "elements": []any{}, "structureTruncated": true, "syntheticMobileTopbar": nil,
		"contentViewport": map[string]any{"width": 390, "height": 844}, "finalViewport": map[string]any{"width": 390, "height": 844},
	}
	base["unexpected"] = true
	raw, _ := json.Marshal(base)
	if _, _, err := ValidateImplementationStructure(raw, "web", 390, 844); err == nil {
		t.Fatal("unknown root field was accepted")
	}
	delete(base, "unexpected")
	base["elements"] = []any{map[string]any{
		"id": "web-1", "parent_id": "missing", "tag": "button", "role": "button", "text": "Save",
		"bounds": map[string]any{"x": 0, "y": 0, "width": 100, "height": 40}, "styles": map[string]string{},
		"clipped": false, "structural_fingerprint": "button-stable", "control_state": nil, "allowed_operations": []string{},
	}}
	raw, _ = json.Marshal(base)
	if _, _, err := ValidateImplementationStructure(raw, "web", 390, 844); err == nil {
		t.Fatal("orphan parent was accepted")
	}
}

func TestImplementationStructureAcceptsIntersectingClippedBounds(t *testing.T) {
	raw := []byte(`{"source":"web","actualUrl":"https://example.test/page","elements":[{"id":"web-1","parent_id":null,"tag":"body","role":null,"text":null,"bounds":{"x":0,"y":0,"width":390,"height":1802},"styles":{},"clipped":true,"structural_fingerprint":"body-stable","control_state":null,"allowed_operations":[]},{"id":"web-2","parent_id":"web-1","tag":"svg","role":null,"text":null,"bounds":{"x":-120,"y":40,"width":300,"height":300},"styles":{},"clipped":true,"structural_fingerprint":"svg-stable","control_state":null,"allowed_operations":[]}],"structureTruncated":false,"syntheticMobileTopbar":null,"contentViewport":{"width":390,"height":1060},"finalViewport":{"width":390,"height":1060},"warnings":[]}`)
	complete, _, err := ValidateImplementationStructure(raw, "web", 390, 1060)
	if err != nil || !complete {
		t.Fatalf("intersecting clipped elements rejected: complete=%v err=%v", complete, err)
	}
}

func TestImplementationStructureRejectsInvalidClippingClaims(t *testing.T) {
	tests := []struct {
		name    string
		x       int
		width   int
		clipped bool
	}{
		{name: "unmarked overflow", x: -10, width: 100, clipped: false},
		{name: "fully outside viewport", x: -200, width: 100, clipped: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			envelope := map[string]any{
				"source": "web", "actualUrl": "https://example.test/page", "structureTruncated": false,
				"syntheticMobileTopbar": nil, "contentViewport": map[string]any{"width": 390, "height": 1060},
				"finalViewport": map[string]any{"width": 390, "height": 1060}, "warnings": []any{},
				"elements": []any{map[string]any{
					"id": "web-1", "parent_id": nil, "tag": "div", "role": nil, "text": nil,
					"bounds": map[string]any{"x": test.x, "y": 10, "width": test.width, "height": 40},
					"styles": map[string]string{}, "clipped": test.clipped, "structural_fingerprint": "element-stable",
					"control_state": nil, "allowed_operations": []string{},
				}},
			}
			raw, _ := json.Marshal(envelope)
			if _, _, err := ValidateImplementationStructure(raw, "web", 390, 1060); err == nil {
				t.Fatal("invalid clipping claim was accepted")
			}
		})
	}
}
