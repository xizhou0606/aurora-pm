package httpapi

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
)

type qualityMaterial struct {
	URL string `json:"url"`
}

type qualityBugBody struct {
	RequirementID         uint                             `json:"requirement_id"`
	Title                 *string                          `json:"title"`
	Status                *string                          `json:"status"`
	Priority              *string                          `json:"priority"`
	BugType               *string                          `json:"bug_type"`
	DefectDomain          *string                          `json:"defect_domain"`
	SeverityKey           *string                          `json:"severity_key"`
	DueAt                 optionalValue[string]            `json:"due_at"`
	DueDate               optionalValue[string]            `json:"due_date"`
	FoundEnvironment      *string                          `json:"found_environment"`
	OperatingSystem       *string                          `json:"operating_system"`
	Browser               *string                          `json:"browser"`
	FoundVersion          *string                          `json:"found_version"`
	ReproductionSteps     *string                          `json:"reproduction_steps"`
	ExpectedResult        *string                          `json:"expected_result"`
	ActualResult          *string                          `json:"actual_result"`
	ReproductionMaterials optionalValue[[]qualityMaterial] `json:"reproduction_materials"`
	ResolutionReason      *string                          `json:"resolution_reason"`
	ResolutionNote        *string                          `json:"resolution_note"`
	ResolutionMaterials   optionalValue[[]qualityMaterial] `json:"resolution_materials"`
	RegressionMaterials   optionalValue[[]qualityMaterial] `json:"regression_materials"`
	AssignmentMaterials   optionalValue[[]qualityMaterial] `json:"assignment_materials"`
	AssigneeUserID        optionalValue[uint]              `json:"assignee_user_id"`
	ReviewerUserID        optionalValue[uint]              `json:"reviewer_user_id"`
	DefectOwnerUserID     optionalValue[uint]              `json:"defect_owner_user_id"`
	VerifiedByUserID      optionalValue[uint]              `json:"verified_by_user_id"`
	SourceExecutionID     optionalValue[uint]              `json:"source_execution_id"`
	SourceTestID          optionalValue[uint]              `json:"source_test_id"`
	ActivityNote          *string                          `json:"activity_note"`
	AssignmentNote        *string                          `json:"assignment_note"`
	RegressionNote        *string                          `json:"regression_note"`
	ResolvedNote          *string                          `json:"resolved_note"`
	ReviewNote            *string                          `json:"review_note"`
	RejectNote            *string                          `json:"reject_note"`
	ReturnNote            *string                          `json:"return_note"`
	Remark                *string                          `json:"remark"`
	Note                  *string                          `json:"note"`
	CCUserIDs             optionalValue[[]uint]            `json:"cc_user_ids"`
	CopyUserIDs           optionalValue[[]uint]            `json:"copy_user_ids"`
}

func qualityBodyUserFactIDs(body qualityBugBody, includeWorkflow bool) []uint {
	assignee := body.AssigneeUserID
	if !assignee.Set {
		assignee = body.DefectOwnerUserID
	}
	reviewer := body.ReviewerUserID
	if !reviewer.Set {
		reviewer = body.VerifiedByUserID
	}
	ids := make([]uint, 0, 4)
	for _, value := range []optionalValue[uint]{assignee, reviewer} {
		if value.Set && value.Value != nil && *value.Value > 0 {
			ids = append(ids, *value.Value)
		}
	}
	if includeWorkflow {
		cc := body.CCUserIDs
		if !cc.Set {
			cc = body.CopyUserIDs
		}
		if cc.Set && cc.Value != nil {
			ids = append(ids, *cc.Value...)
		}
	}
	return mergeSortedUserIDs(ids)
}

const (
	qualityBugTitleMaxRunes = 160
	qualityBugNoteMaxRunes  = 5000
)

func validQualityStatus(value string) bool {
	switch normalizeQualityBugStatus(value) {
	case "triage", "todo", "in_progress", "reopened", "review", "done":
		return true
	default:
		return false
	}
}

func validQualityPriority(value string) bool {
	switch normalizeStatus(value) {
	case "low", "medium", "high", "urgent":
		return true
	default:
		return false
	}
}

func validQualitySeverity(value string) bool {
	switch normalizeStatus(value) {
	case "p0", "p1", "p2", "p3":
		return true
	default:
		return false
	}
}

func validQualityDefectDomain(value string) bool {
	return quality.ValidBugDefectDomain(value)
}

func validQualityResolutionReason(value string) bool {
	return quality.ValidBugResolutionReason(value)
}

func encodeQualityMaterials(value []qualityMaterial) (string, error) {
	clean := make([]qualityMaterial, 0, len(value))
	for _, item := range value {
		item.URL = strings.TrimSpace(item.URL)
		if item.URL == "" {
			return "", fmt.Errorf("material url required")
		}
		parsed, err := url.Parse(item.URL)
		if err != nil || parsed == nil || parsed.Host == "" ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") {
			return "", fmt.Errorf("material url must use http or https")
		}
		clean = append(clean, item)
	}
	raw, err := json.Marshal(clean)
	return string(raw), err
}

func normalizeQualityMaterials(value []qualityMaterial) ([]qualityMaterial, error) {
	raw, err := encodeQualityMaterials(value)
	if err != nil {
		return nil, err
	}
	return decodeQualityMaterials(raw), nil
}

func decodeQualityMaterials(raw string) []qualityMaterial {
	items := []qualityMaterial{}
	if strings.TrimSpace(raw) == "" {
		return items
	}
	if json.Unmarshal([]byte(raw), &items) != nil {
		return []qualityMaterial{}
	}
	return items
}

func (s *Server) qualityBugUpdates(
	body qualityBugBody,
) (map[string]any, error) {
	updates := map[string]any{}
	for key, value := range map[string]*string{
		"title": body.Title, "bug_type": body.BugType,
		"found_environment": body.FoundEnvironment, "operating_system": body.OperatingSystem,
		"browser": body.Browser, "found_version": body.FoundVersion,
		"reproduction_steps": body.ReproductionSteps,
		"expected_result":    body.ExpectedResult, "actual_result": body.ActualResult,
		"resolution_reason": body.ResolutionReason, "resolution_note": body.ResolutionNote,
	} {
		if value != nil {
			updates[key] = strings.TrimSpace(*value)
		}
	}
	if body.DefectDomain != nil {
		if !validQualityDefectDomain(*body.DefectDomain) {
			return nil, fmt.Errorf("defect_domain is invalid")
		}
		updates["defect_domain"] = strings.ToLower(strings.TrimSpace(*body.DefectDomain))
	}
	if title, ok := updates["title"].(string); ok && quality.TextRuneCount(title) > qualityBugTitleMaxRunes {
		return nil, fmt.Errorf("title too long")
	}
	if note, ok := updates["resolution_note"].(string); ok {
		if err := validateQualityNoteLength("resolution_note", note); err != nil {
			return nil, err
		}
	}
	if body.Status != nil {
		if !validQualityStatus(*body.Status) {
			return nil, fmt.Errorf("status is invalid")
		}
		updates["status"] = normalizeQualityBugStatus(*body.Status)
	}
	if body.Priority != nil {
		if !validQualityPriority(*body.Priority) {
			return nil, fmt.Errorf("priority is invalid")
		}
		updates["priority"] = normalizeStatus(*body.Priority)
	}
	if body.SeverityKey != nil {
		if !validQualitySeverity(*body.SeverityKey) {
			return nil, fmt.Errorf("severity_key is invalid")
		}
		updates["severity_key"] = normalizeStatus(*body.SeverityKey)
	}
	if body.ResolutionReason != nil {
		if !validQualityResolutionReason(*body.ResolutionReason) {
			return nil, fmt.Errorf("resolution_reason is invalid")
		}
		updates["resolution_reason"] = normalizeStatus(*body.ResolutionReason)
	}
	due := body.DueAt
	if body.DueDate.Set {
		if due.Set {
			return nil, fmt.Errorf("due_at and due_date cannot both be provided")
		}
		due = body.DueDate
	}
	if due.Set {
		parsed, err := parseOptionalTime(due.Value)
		if err != nil {
			return nil, err
		}
		updates["due_at"] = parsed
	}
	assignee := body.AssigneeUserID
	if !assignee.Set {
		assignee = body.DefectOwnerUserID
	}
	reviewer := body.ReviewerUserID
	if !reviewer.Set {
		reviewer = body.VerifiedByUserID
	}
	for key, value := range map[string]optionalValue[uint]{
		"assignee_user_id": assignee, "reviewer_user_id": reviewer,
	} {
		if !value.Set {
			continue
		}
		if value.Value != nil && *value.Value == 0 {
			return nil, fmt.Errorf("%s must be positive", key)
		}
		updates[key] = value.Value
	}
	for key, value := range map[string]optionalValue[uint]{
		"source_execution_id": body.SourceExecutionID, "source_test_id": body.SourceTestID,
	} {
		if !value.Set {
			continue
		}
		if value.Value != nil && *value.Value == 0 {
			return nil, fmt.Errorf("%s must be positive", key)
		}
		updates[key] = value.Value
	}
	for key, value := range map[string]optionalValue[[]qualityMaterial]{
		"reproduction_materials_json": body.ReproductionMaterials,
		"resolution_materials_json":   body.ResolutionMaterials,
	} {
		if !value.Set {
			continue
		}
		materials := []qualityMaterial{}
		if value.Value != nil {
			materials = *value.Value
		}
		raw, err := encodeQualityMaterials(materials)
		if err != nil {
			return nil, err
		}
		updates[key] = raw
	}
	return updates, nil
}

func applyQualityBugUpdates(bug *qualitygorm.QualityBug, updates map[string]any) {
	// GORM applies the complete map. This helper only keeps the in-memory value
	// current for audit and response generation after a create.
	if value, ok := updates["title"].(string); ok {
		bug.Title = value
	}
	if value, ok := updates["status"].(string); ok {
		bug.Status = value
	}
	if value, ok := updates["priority"].(string); ok {
		bug.Priority = value
	}
	if value, ok := updates["severity_key"].(string); ok {
		bug.SeverityKey = value
	}
	if value, ok := updates["bug_type"].(string); ok {
		bug.BugType = value
	}
	if value, ok := updates["defect_domain"].(string); ok {
		bug.DefectDomain = value
	}
	if value, ok := updates["found_environment"].(string); ok {
		bug.FoundEnvironment = value
	}
	if value, ok := updates["operating_system"].(string); ok {
		bug.OperatingSystem = value
	}
	if value, ok := updates["browser"].(string); ok {
		bug.Browser = value
	}
	if value, ok := updates["found_version"].(string); ok {
		bug.FoundVersion = value
	}
	if value, ok := updates["reproduction_steps"].(string); ok {
		bug.ReproductionSteps = value
	}
	if value, ok := updates["expected_result"].(string); ok {
		bug.ExpectedResult = value
	}
	if value, ok := updates["actual_result"].(string); ok {
		bug.ActualResult = value
	}
	if value, ok := updates["resolution_reason"].(string); ok {
		bug.ResolutionReason = value
	}
	if value, ok := updates["resolution_note"].(string); ok {
		bug.ResolutionNote = value
	}
	if value, ok := updates["reproduction_materials_json"].(string); ok {
		bug.ReproductionMaterialsJSON = value
	}
	if value, ok := updates["resolution_materials_json"].(string); ok {
		bug.ResolutionMaterialsJSON = value
	}
}
