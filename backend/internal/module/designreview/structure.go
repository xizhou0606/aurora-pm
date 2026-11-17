package designreview

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"strings"
)

type structureBounds struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func (bounds structureBounds) valid(allowOutside bool) bool {
	values := []float64{bounds.X, bounds.Y, bounds.Width, bounds.Height}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return bounds.Width >= 0 && bounds.Height >= 0 && (allowOutside || bounds.X >= 0 && bounds.Y >= 0)
}

func (bounds structureBounds) intersectsViewport(width, height int) bool {
	return bounds.X < float64(width) && bounds.Y < float64(height) &&
		bounds.X+bounds.Width > 0 && bounds.Y+bounds.Height > 0
}

func (bounds structureBounds) insideViewport(width, height int) bool {
	return bounds.X >= 0 && bounds.Y >= 0 &&
		bounds.X+bounds.Width <= float64(width)+1 && bounds.Y+bounds.Height <= float64(height)+1
}

type figmaStructureLayout struct {
	Mode         *string    `json:"mode,omitempty"`
	Gap          *float64   `json:"gap,omitempty"`
	Padding      []*float64 `json:"padding,omitempty"`
	CornerRadius *float64   `json:"cornerRadius,omitempty"`
}

type figmaStructureNode struct {
	ID                string                `json:"id"`
	Name              string                `json:"name"`
	Type              string                `json:"type"`
	Visible           *bool                 `json:"visible,omitempty"`
	Bounds            *structureBounds      `json:"bounds,omitempty"`
	Fills             json.RawMessage       `json:"fills,omitempty"`
	Strokes           json.RawMessage       `json:"strokes,omitempty"`
	Effects           json.RawMessage       `json:"effects,omitempty"`
	Style             json.RawMessage       `json:"style,omitempty"`
	Layout            *figmaStructureLayout `json:"layout,omitempty"`
	Text              *string               `json:"text,omitempty"`
	Children          []figmaStructureNode  `json:"children,omitempty"`
	ChildrenTruncated bool                  `json:"childrenTruncated,omitempty"`
	Truncated         bool                  `json:"truncated,omitempty"`
}

func ValidateDesignStructure(raw []byte) error {
	var root figmaStructureNode
	if decodeStrictJSON(raw, &root) != nil || root.ID == "" || len(root.ID) > 200 || len(root.Name) > 500 || root.Type == "" || len(root.Type) > 120 || root.Bounds == nil || !root.Bounds.valid(true) || root.Bounds.Width <= 0 || root.Bounds.Height <= 0 {
		return ErrInvalid
	}
	count := 0
	var visit func(figmaStructureNode, int) bool
	visit = func(node figmaStructureNode, depth int) bool {
		count++
		if count > 2500 || depth > 13 || node.ID == "" || len(node.ID) > 200 || len(node.Name) > 500 || node.Type == "" || len(node.Type) > 120 || len(node.Children) > 200 || node.Text != nil && len([]rune(*node.Text)) > 500 || node.Bounds != nil && !node.Bounds.valid(true) {
			return false
		}
		for _, child := range node.Children {
			if !visit(child, depth+1) {
				return false
			}
		}
		return true
	}
	if !visit(root, 0) {
		return ErrInvalid
	}
	return nil
}

type implementationStructure struct {
	Source                string              `json:"source"`
	ActualURL             string              `json:"actualUrl,omitempty"`
	Target                *DesignReviewTarget `json:"target,omitempty"`
	Elements              []json.RawMessage   `json:"elements"`
	StructureTruncated    bool                `json:"structureTruncated"`
	SyntheticMobileTopbar json.RawMessage     `json:"syntheticMobileTopbar"`
	ContentViewport       structureViewport   `json:"contentViewport"`
	FinalViewport         structureViewport   `json:"finalViewport"`
	Warnings              []structureWarning  `json:"warnings,omitempty"`
}

type structureViewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type structureWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type webStructureElement struct {
	ID                    string                    `json:"id"`
	ParentID              *string                   `json:"parent_id"`
	Tag                   string                    `json:"tag"`
	Role                  *string                   `json:"role"`
	Text                  *string                   `json:"text"`
	Bounds                structureBounds           `json:"bounds"`
	Styles                map[string]string         `json:"styles"`
	Clipped               bool                      `json:"clipped"`
	StructuralFingerprint string                    `json:"structural_fingerprint"`
	ControlState          *webStructureControlState `json:"control_state"`
	AllowedOperations     []string                  `json:"allowed_operations"`
}

type webStructureControlState struct {
	Checked      *bool   `json:"checked"`
	AriaChecked  *string `json:"aria_checked"`
	AriaPressed  *string `json:"aria_pressed"`
	AriaSelected *string `json:"aria_selected"`
	DataState    *string `json:"data_state"`
}

type nativeStructureElement struct {
	ID         string          `json:"id"`
	ParentID   *string         `json:"parent_id"`
	NativeType string          `json:"native_type"`
	Role       *string         `json:"role"`
	Label      *string         `json:"label"`
	Text       *string         `json:"text"`
	Value      *string         `json:"value"`
	Identifier *string         `json:"identifier"`
	ResourceID *string         `json:"resource_id"`
	Bounds     structureBounds `json:"bounds"`
	State      struct {
		Visible  bool  `json:"visible"`
		Enabled  bool  `json:"enabled"`
		Selected *bool `json:"selected"`
		Focused  *bool `json:"focused"`
		Checked  *bool `json:"checked"`
	} `json:"state"`
	Clipped               bool   `json:"clipped"`
	StructuralFingerprint string `json:"structural_fingerprint"`
}

func ValidateImplementationStructure(raw []byte, implementationKind string, viewportWidth, viewportHeight int) (bool, json.RawMessage, error) {
	var structure implementationStructure
	if decodeStrictJSON(raw, &structure) != nil || structure.Source != implementationKind || len(structure.Elements) > 1500 ||
		structure.ContentViewport.Width <= 0 || structure.ContentViewport.Height <= 0 || structure.FinalViewport.Width != viewportWidth || structure.FinalViewport.Height != viewportHeight ||
		len(structure.Warnings) > 50 || !validImplementationStructureEnvelope(structure) {
		return false, nil, ErrInvalid
	}
	seen := make(map[string]struct{}, len(structure.Elements))
	for _, rawElement := range structure.Elements {
		var id string
		var parentID *string
		var bounds structureBounds
		var fingerprint string
		var clipped bool
		if structure.Source == "web" {
			var element webStructureElement
			if decodeStrictJSON(rawElement, &element) != nil || !validWebStructureElement(element) {
				return false, nil, ErrInvalid
			}
			for _, operation := range element.AllowedOperations {
				if operation != "set_text" && operation != "set_control_state" && operation != "repeat_template" {
					return false, nil, ErrInvalid
				}
			}
			id, parentID, bounds, fingerprint, clipped = element.ID, element.ParentID, element.Bounds, element.StructuralFingerprint, element.Clipped
		} else if structure.Source == "mobile" {
			var element nativeStructureElement
			if decodeStrictJSON(rawElement, &element) != nil || !validNativeStructureElement(element) {
				return false, nil, ErrInvalid
			}
			id, parentID, bounds, fingerprint, clipped = element.ID, element.ParentID, element.Bounds, element.StructuralFingerprint, element.Clipped
		} else {
			return false, nil, ErrInvalid
		}
		if id == "" || len(id) > 200 || len(fingerprint) < 8 || len(fingerprint) > 200 || !bounds.valid(clipped) ||
			(clipped && !bounds.intersectsViewport(viewportWidth, viewportHeight)) || (!clipped && !bounds.insideViewport(viewportWidth, viewportHeight)) {
			return false, nil, ErrInvalid
		}
		if _, duplicate := seen[id]; duplicate {
			return false, nil, ErrInvalid
		}
		if parentID != nil {
			if _, exists := seen[*parentID]; !exists {
				return false, nil, ErrInvalid
			}
		}
		seen[id] = struct{}{}
	}
	for _, warning := range structure.Warnings {
		if strings.TrimSpace(warning.Code) == "" || len(warning.Code) > 80 || strings.TrimSpace(warning.Message) == "" || len([]rune(warning.Message)) > 500 {
			return false, nil, ErrInvalid
		}
	}
	warnings, _ := json.Marshal(structure.Warnings)
	return !structure.StructureTruncated, warnings, nil
}

type IdentityAnchor struct {
	Kind     string  `json:"kind"`
	StableID string  `json:"stableId"`
	NodeType *string `json:"nodeType"`
	Region   Region  `json:"region"`
}

func DeriveIdentityAnchors(designStructure, implementationStructure []byte, designRegion, implementationRegion *Region, viewportWidth, viewportHeight int) (*IdentityAnchor, *IdentityAnchor) {
	return deriveDesignAnchor(designStructure, designRegion), deriveImplementationAnchor(implementationStructure, implementationRegion, viewportWidth, viewportHeight)
}

func deriveDesignAnchor(raw []byte, target *Region) *IdentityAnchor {
	var root figmaStructureNode
	if target == nil || decodeStrictJSON(raw, &root) != nil || root.Bounds == nil {
		return nil
	}
	var best *IdentityAnchor
	bestArea := math.Inf(1)
	var visit func(figmaStructureNode)
	visit = func(node figmaStructureNode) {
		if node.Visible != nil && !*node.Visible {
			return
		}
		if node.Bounds != nil {
			if region, ok := normalizedRegion(*node.Bounds, *root.Bounds); ok && containsCenter(region, *target) {
				if area := region.Width * region.Height; area < bestArea {
					nodeType := node.Type
					best = &IdentityAnchor{Kind: "figma", StableID: node.ID, NodeType: &nodeType, Region: region}
					bestArea = area
				}
			}
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(root)
	return best
}

func deriveImplementationAnchor(raw []byte, target *Region, viewportWidth, viewportHeight int) *IdentityAnchor {
	var structure implementationStructure
	if target == nil || viewportWidth <= 0 || viewportHeight <= 0 || decodeStrictJSON(raw, &structure) != nil {
		return nil
	}
	root := structureBounds{Width: float64(viewportWidth), Height: float64(viewportHeight)}
	var best *IdentityAnchor
	bestArea := math.Inf(1)
	for _, rawElement := range structure.Elements {
		var common struct {
			Tag                   string          `json:"tag"`
			NativeType            string          `json:"native_type"`
			Role                  *string         `json:"role"`
			Bounds                structureBounds `json:"bounds"`
			StructuralFingerprint string          `json:"structural_fingerprint"`
		}
		if json.Unmarshal(rawElement, &common) != nil || common.StructuralFingerprint == "" {
			continue
		}
		region, ok := normalizedRegion(common.Bounds, root)
		if !ok || !containsCenter(region, *target) || region.Width*region.Height >= bestArea {
			continue
		}
		parts := make([]string, 0, 2)
		if common.NativeType != "" {
			parts = append(parts, common.NativeType)
		} else if common.Tag != "" {
			parts = append(parts, common.Tag)
		}
		if common.Role != nil && *common.Role != "" {
			parts = append(parts, *common.Role)
		}
		var nodeType *string
		if value := strings.Join(parts, ":"); value != "" {
			nodeType = &value
		}
		best = &IdentityAnchor{Kind: "implementation", StableID: common.StructuralFingerprint, NodeType: nodeType, Region: region}
		bestArea = region.Width * region.Height
	}
	return best
}

func normalizedRegion(value, root structureBounds) (Region, bool) {
	if root.Width <= 0 || root.Height <= 0 || value.Width <= 0 || value.Height <= 0 {
		return Region{}, false
	}
	left, top := math.Max(root.X, value.X), math.Max(root.Y, value.Y)
	right, bottom := math.Min(root.X+root.Width, value.X+value.Width), math.Min(root.Y+root.Height, value.Y+value.Height)
	if right <= left || bottom <= top {
		return Region{}, false
	}
	return Region{X: (left - root.X) / root.Width, Y: (top - root.Y) / root.Height, Width: (right - left) / root.Width, Height: (bottom - top) / root.Height}, true
}

func containsCenter(container, target Region) bool {
	x, y := target.X+target.Width/2, target.Y+target.Height/2
	return x >= container.X && x <= container.X+container.Width && y >= container.Y && y <= container.Y+container.Height
}

func decodeStrictJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ErrInvalid
	}
	return nil
}
