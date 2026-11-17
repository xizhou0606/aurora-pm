package designreview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
)

var normalizationFingerprint = regexp.MustCompile(`^[a-f0-9]{16,64}$`)
var normalizationElementID = regexp.MustCompile(`^el-[1-9][0-9]*$`)

type NormalizationControlState struct {
	Checked      *bool   `json:"checked"`
	AriaChecked  *string `json:"aria_checked"`
	AriaPressed  *string `json:"aria_pressed"`
	AriaSelected *string `json:"aria_selected"`
	DataState    *string `json:"data_state"`
}

type NormalizationElement struct {
	ID                    string                     `json:"id"`
	ParentID              *string                    `json:"parent_id"`
	Tag                   string                     `json:"tag"`
	Role                  *string                    `json:"role"`
	Text                  *string                    `json:"text"`
	Bounds                map[string]float64         `json:"bounds"`
	Styles                map[string]string          `json:"styles"`
	Clipped               bool                       `json:"clipped"`
	StructuralFingerprint string                     `json:"structural_fingerprint"`
	ControlState          *NormalizationControlState `json:"control_state"`
	AllowedOperations     []string                   `json:"allowed_operations"`
}

type NormalizationInventory struct {
	Version         string                 `json:"version"`
	Token           string                 `json:"token"`
	CapturedAt      string                 `json:"captured_at"`
	PageFingerprint string                 `json:"page_fingerprint"`
	Elements        []NormalizationElement `json:"elements"`
	Truncated       bool                   `json:"truncated"`
}

type NormalizationRepeatedValue struct {
	TargetID     string                     `json:"target_id"`
	Text         *string                    `json:"text"`
	ControlState *NormalizationControlState `json:"control_state"`
}

type NormalizationRepeatedItem struct {
	Values []NormalizationRepeatedValue `json:"values"`
}

type NormalizationOperation struct {
	Type                         string                      `json:"type"`
	TargetID                     string                      `json:"target_id,omitempty"`
	ExpectedFingerprint          string                      `json:"expected_fingerprint,omitempty"`
	Text                         *string                     `json:"text,omitempty"`
	State                        *NormalizationControlState  `json:"state,omitempty"`
	ContainerID                  string                      `json:"container_id,omitempty"`
	TemplateID                   string                      `json:"template_id,omitempty"`
	ExpectedContainerFingerprint string                      `json:"expected_container_fingerprint,omitempty"`
	ExpectedTemplateFingerprint  string                      `json:"expected_template_fingerprint,omitempty"`
	Items                        []NormalizationRepeatedItem `json:"items,omitempty"`
	FigmaSource                  string                      `json:"figma_source"`
	Confidence                   float64                     `json:"confidence"`
}

type NormalizationSkipped struct {
	FigmaSource string  `json:"figma_source"`
	TargetID    *string `json:"target_id"`
	Reason      string  `json:"reason"`
}

type DOMNormalizationPlan struct {
	InventoryToken   string                   `json:"inventory_token"`
	Model            string                   `json:"model"`
	PromptVersion    string                   `json:"prompt_version"`
	AlreadyAligned   bool                     `json:"already_aligned"`
	MismatchDetected bool                     `json:"mismatch_detected"`
	Operations       []NormalizationOperation `json:"operations"`
	Skipped          []NormalizationSkipped   `json:"skipped"`
	Summary          string                   `json:"summary"`
}

type DOMNormalizationRequest struct {
	WorkspaceID      uint
	Model            string
	ReasoningEffort  string
	ImageDetail      string
	FigmaContentType string
	FigmaSHA256      string
	FigmaData        []byte
	Inventory        NormalizationInventory
}

type domNormalizationConfig struct {
	Enabled bool `json:"enabled"`
}

type domNormalizationState struct {
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
	Report  struct {
		Counts struct {
			Applied int `json:"applied"`
			Skipped int `json:"skipped"`
		} `json:"counts"`
	} `json:"report"`
}

type normalizationManifest struct {
	Requested bool `json:"requested"`
	Report    struct {
		Plan struct {
			Model            string `json:"model"`
			PromptVersion    string `json:"prompt_version"`
			AlreadyAligned   bool   `json:"already_aligned"`
			MismatchDetected bool   `json:"mismatch_detected"`
			Summary          string `json:"summary"`
			Skipped          []struct {
				FigmaSource string  `json:"figma_source"`
				TargetID    *string `json:"target_id"`
				Reason      string  `json:"reason"`
			} `json:"skipped"`
		} `json:"plan"`
		Operations []struct {
			Type     string  `json:"type"`
			TargetID string  `json:"target_id"`
			Result   string  `json:"result"`
			Reason   *string `json:"reason"`
		} `json:"operations"`
		Counts struct {
			Applied int `json:"applied"`
			Skipped int `json:"skipped"`
		} `json:"counts"`
	} `json:"report"`
}

func ParseDOMNormalizationConfig(raw json.RawMessage, implementationKind string) (json.RawMessage, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{"enabled":false}`)
	}
	var config domNormalizationConfig
	if decodeStrictJSON(raw, &config) != nil || config.Enabled && implementationKind != "web" {
		return nil, ErrInvalid
	}
	if !config.Enabled {
		return json.RawMessage(`{"enabled":false}`), nil
	}
	return json.RawMessage(`{"enabled":true,"status":"pending"}`), nil
}

func DOMNormalizationEnabled(raw json.RawMessage) bool {
	var state struct {
		Enabled bool `json:"enabled"`
	}
	return json.Unmarshal(raw, &state) == nil && state.Enabled
}

func ParseDOMNormalizationManifest(raw json.RawMessage) (json.RawMessage, error) {
	var manifest normalizationManifest
	if decodeStrictJSON(raw, &manifest) != nil || !manifest.Requested ||
		strings.TrimSpace(manifest.Report.Plan.Model) == "" || len(manifest.Report.Plan.Model) > 120 ||
		strings.TrimSpace(manifest.Report.Plan.PromptVersion) == "" || len(manifest.Report.Plan.PromptVersion) > 160 ||
		strings.TrimSpace(manifest.Report.Plan.Summary) == "" || len(manifest.Report.Plan.Summary) > 2000 ||
		len(manifest.Report.Plan.Skipped) > 200 || len(manifest.Report.Operations) > 200 ||
		manifest.Report.Counts.Applied < 0 || manifest.Report.Counts.Skipped < 0 ||
		manifest.Report.Plan.AlreadyAligned && (manifest.Report.Plan.MismatchDetected || len(manifest.Report.Operations) > 0) {
		return nil, ErrInvalid
	}
	for _, skipped := range manifest.Report.Plan.Skipped {
		if strings.TrimSpace(skipped.FigmaSource) == "" || len(skipped.FigmaSource) > 500 ||
			strings.TrimSpace(skipped.Reason) == "" || len(skipped.Reason) > 1000 ||
			skipped.TargetID != nil && !normalizationElementID.MatchString(*skipped.TargetID) {
			return nil, ErrInvalid
		}
	}
	applied, skipped := 0, len(manifest.Report.Plan.Skipped)
	for _, operation := range manifest.Report.Operations {
		if !normalizationElementID.MatchString(operation.TargetID) ||
			(operation.Type != "set_text" && operation.Type != "set_control_state" && operation.Type != "replace_repeated_items") ||
			(operation.Result != "applied" && operation.Result != "skipped") ||
			operation.Result == "applied" && operation.Reason != nil ||
			operation.Result == "skipped" && (operation.Reason == nil || strings.TrimSpace(*operation.Reason) == "" || len(*operation.Reason) > 1000) {
			return nil, ErrInvalid
		}
		if operation.Result == "applied" {
			applied++
		} else {
			skipped++
		}
	}
	if applied != manifest.Report.Counts.Applied || skipped != manifest.Report.Counts.Skipped {
		return nil, ErrInvalid
	}
	status := "complete"
	if skipped > 0 {
		status = "partial"
	}
	state := domNormalizationState{Enabled: true, Status: status}
	state.Report.Counts.Applied, state.Report.Counts.Skipped = applied, skipped
	return json.Marshal(state)
}

func (service *Service) NormalizeDOM(ctx context.Context, workspaceID uint, actor ActorIdentity, figmaURL string, inventoryRaw json.RawMessage) (DOMNormalizationPlan, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsReviewsCreate); err != nil {
		return DOMNormalizationPlan{}, err
	}
	if !service.availability.VisualModel || !service.availability.Figma || service.normalizer == nil || service.figma == nil {
		return DOMNormalizationPlan{}, ErrUnavailable
	}
	var inventory NormalizationInventory
	if json.Unmarshal(inventoryRaw, &inventory) != nil || !inventory.Valid() {
		return DOMNormalizationPlan{}, ErrInvalid
	}
	fileKey, nodeID, err := ParseFigmaNodeURL(figmaURL)
	if err != nil {
		return DOMNormalizationPlan{}, err
	}
	_, figmaRaw, contentType, err := service.figma.Evidence(ctx, fileKey, nodeID)
	if err != nil {
		return DOMNormalizationPlan{}, err
	}
	sum := sha256.Sum256(figmaRaw)
	plan, err := service.normalizer.Plan(ctx, DOMNormalizationRequest{
		WorkspaceID: workspaceID, Model: service.availability.Model, ReasoningEffort: service.availability.ReasoningEffort,
		ImageDetail: service.availability.ImageDetail, FigmaContentType: contentType, FigmaSHA256: hex.EncodeToString(sum[:]),
		FigmaData: figmaRaw, Inventory: inventory,
	})
	if err != nil || !plan.Valid(inventory) {
		if err != nil {
			return DOMNormalizationPlan{}, err
		}
		return DOMNormalizationPlan{}, ErrInvalid
	}
	return plan, nil
}

func (inventory NormalizationInventory) Valid() bool {
	if inventory.Version != "1.0" || uuidValid(inventory.Token) != nil || !normalizationFingerprint.MatchString(inventory.PageFingerprint) || len(inventory.Elements) > 1500 {
		return false
	}
	seen := make(map[string]struct{}, len(inventory.Elements))
	for _, element := range inventory.Elements {
		if !normalizationElementID.MatchString(element.ID) || !normalizationFingerprint.MatchString(element.StructuralFingerprint) || len(element.Tag) == 0 || len(element.Tag) > 40 || len(element.Styles) > 50 {
			return false
		}
		if _, exists := seen[element.ID]; exists {
			return false
		}
		seen[element.ID] = struct{}{}
		for _, operation := range element.AllowedOperations {
			if operation != "set_text" && operation != "set_control_state" && operation != "repeat_template" {
				return false
			}
		}
	}
	for _, element := range inventory.Elements {
		if element.ParentID != nil {
			if _, exists := seen[*element.ParentID]; !exists {
				return false
			}
		}
	}
	return true
}

func (plan DOMNormalizationPlan) Valid(inventory NormalizationInventory) bool {
	if plan.InventoryToken != inventory.Token || strings.TrimSpace(plan.Model) == "" || strings.TrimSpace(plan.PromptVersion) == "" || strings.TrimSpace(plan.Summary) == "" || len(plan.Operations) > 200 || len(plan.Skipped) > 200 {
		return false
	}
	if plan.AlreadyAligned && (plan.MismatchDetected || len(plan.Operations) > 0) {
		return false
	}
	elements := make(map[string]NormalizationElement, len(inventory.Elements))
	for _, element := range inventory.Elements {
		elements[element.ID] = element
	}
	used := make(map[string]struct{}, len(plan.Operations))
	for _, operation := range plan.Operations {
		if operation.Confidence < 0.9 || operation.Confidence > 1 || strings.TrimSpace(operation.FigmaSource) == "" {
			return false
		}
		target := operation.TargetID
		if operation.Type == "replace_repeated_items" {
			target = operation.TemplateID
		}
		if _, duplicate := used[target]; duplicate {
			return false
		}
		used[target] = struct{}{}
		if !validNormalizationOperation(operation, elements) {
			return false
		}
	}
	return true
}

func validNormalizationOperation(operation NormalizationOperation, elements map[string]NormalizationElement) bool {
	hasAllowed := func(element NormalizationElement, allowed string) bool {
		for _, value := range element.AllowedOperations {
			if value == allowed {
				return true
			}
		}
		return false
	}
	switch operation.Type {
	case "set_text":
		element, ok := elements[operation.TargetID]
		return ok && operation.Text != nil && operation.State == nil && operation.ExpectedFingerprint == element.StructuralFingerprint && hasAllowed(element, operation.Type)
	case "set_control_state":
		element, ok := elements[operation.TargetID]
		return ok && operation.Text == nil && operation.State != nil && operation.ExpectedFingerprint == element.StructuralFingerprint && hasAllowed(element, operation.Type)
	case "replace_repeated_items":
		template, templateOK := elements[operation.TemplateID]
		container, containerOK := elements[operation.ContainerID]
		if !templateOK || !containerOK || template.ParentID == nil || *template.ParentID != container.ID || !hasAllowed(template, "repeat_template") ||
			operation.ExpectedTemplateFingerprint != template.StructuralFingerprint || operation.ExpectedContainerFingerprint != container.StructuralFingerprint || len(operation.Items) == 0 || len(operation.Items) > 100 {
			return false
		}
		for _, item := range operation.Items {
			if len(item.Values) == 0 || len(item.Values) > 30 {
				return false
			}
			itemTargets := make(map[string]struct{}, len(item.Values))
			for _, value := range item.Values {
				target, ok := elements[value.TargetID]
				_, duplicate := itemTargets[value.TargetID]
				if !ok || duplicate || !descendsFrom(value.TargetID, template.ID, elements) ||
					(value.Text == nil && value.ControlState == nil) ||
					(value.Text != nil && !hasAllowed(target, "set_text")) ||
					(value.ControlState != nil && !hasAllowed(target, "set_control_state")) {
					return false
				}
				itemTargets[value.TargetID] = struct{}{}
			}
		}
		return true
	default:
		return false
	}
}

func descendsFrom(id, root string, elements map[string]NormalizationElement) bool {
	for current := id; current != ""; {
		if current == root {
			return true
		}
		element, ok := elements[current]
		if !ok || element.ParentID == nil {
			return false
		}
		current = *element.ParentID
	}
	return false
}

var uuidValid = func(value string) error { return uuid.Validate(value) }
