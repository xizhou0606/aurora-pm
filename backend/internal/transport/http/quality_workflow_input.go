package httpapi

import (
	"fmt"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	"strings"
)

type qualityWorkflowPayload struct {
	AssignmentNote         string
	ResolutionNote         string
	RegressionNote         string
	CCUserIDs              []uint
	AssignmentMaterials    []qualityMaterial
	ResolutionMaterials    []qualityMaterial
	RegressionMaterials    []qualityMaterial
	IsResolutionPatch      bool
	IsRegressionReturn     bool
	AssignmentTargetChange bool
}

func (payload qualityWorkflowPayload) hasAuxiliaryActivity() bool {
	return payload.AssignmentNote != "" || payload.RegressionNote != "" ||
		len(payload.CCUserIDs) > 0 || len(payload.AssignmentMaterials) > 0 ||
		len(payload.RegressionMaterials) > 0
}

func (payload qualityWorkflowPayload) shouldRecordAssignment() bool {
	return payload.AssignmentTargetChange || payload.AssignmentNote != "" ||
		len(payload.CCUserIDs) > 0 || len(payload.AssignmentMaterials) > 0
}

func (s *Server) normalizeQualityWorkflowBody(
	existing qualitygorm.QualityBug,
	body *qualityBugBody,
) (qualityWorkflowPayload, error) {
	payload := qualityWorkflowPayload{}
	nextStatus := ""
	if body.Status != nil {
		nextStatus = normalizeQualityBugStatus(*body.Status)
	}
	payload.IsResolutionPatch = nextStatus == "review" || body.ResolutionReason != nil || body.ResolutionMaterials.Set
	payload.IsRegressionReturn = normalizeQualityBugStatus(existing.Status) == "review" &&
		(nextStatus == "reopened" || nextStatus == "in_progress")

	payload.AssignmentNote = firstQualityNote(body.AssignmentNote)
	if payload.AssignmentNote == "" && !payload.IsResolutionPatch && !payload.IsRegressionReturn {
		payload.AssignmentNote = firstQualityNote(body.Remark, body.Note, body.ActivityNote)
	}
	payload.ResolutionNote = firstQualityNote(body.ResolutionNote, body.ResolvedNote)
	if payload.ResolutionNote == "" && payload.IsResolutionPatch {
		payload.ResolutionNote = firstQualityNote(body.Remark, body.Note, body.ActivityNote)
	}
	payload.RegressionNote = firstQualityNote(
		body.RegressionNote, body.ReviewNote, body.RejectNote, body.ReturnNote,
	)
	if payload.RegressionNote == "" && payload.IsRegressionReturn {
		payload.RegressionNote = firstQualityNote(body.Remark, body.Note, body.ActivityNote)
	}
	for key, value := range map[string]string{
		"assignment_note": payload.AssignmentNote,
		"resolution_note": payload.ResolutionNote,
		"regression_note": payload.RegressionNote,
	} {
		if err := validateQualityNoteLength(key, value); err != nil {
			return payload, err
		}
	}
	if payload.IsResolutionPatch && payload.ResolutionNote != "" {
		body.ResolutionNote = &payload.ResolutionNote
	}

	cc := body.CCUserIDs
	if !cc.Set {
		cc = body.CopyUserIDs
	}
	if cc.Set {
		ids := []uint{}
		if cc.Value != nil {
			ids = *cc.Value
		}
		seen := map[uint]struct{}{}
		for _, id := range ids {
			if id == 0 {
				return payload, fmt.Errorf("cc_user_ids must contain positive platform user ids")
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			payload.CCUserIDs = append(payload.CCUserIDs, id)
		}
	}
	if body.RegressionMaterials.Set {
		materials := []qualityMaterial{}
		if body.RegressionMaterials.Value != nil {
			materials = *body.RegressionMaterials.Value
		}
		clean, err := normalizeQualityMaterials(materials)
		if err != nil {
			return payload, err
		}
		payload.RegressionMaterials = clean
	}
	if body.ResolutionMaterials.Set {
		materials := []qualityMaterial{}
		if body.ResolutionMaterials.Value != nil {
			materials = *body.ResolutionMaterials.Value
		}
		clean, err := normalizeQualityMaterials(materials)
		if err != nil {
			return payload, err
		}
		payload.ResolutionMaterials = clean
	}
	if body.AssignmentMaterials.Set {
		materials := []qualityMaterial{}
		if body.AssignmentMaterials.Value != nil {
			materials = *body.AssignmentMaterials.Value
		}
		clean, err := normalizeQualityMaterials(materials)
		if err != nil {
			return payload, err
		}
		payload.AssignmentMaterials = clean
	}

	requestedAssignee := body.AssigneeUserID
	if !requestedAssignee.Set {
		requestedAssignee = body.DefectOwnerUserID
	}
	if requestedAssignee.Set {
		existingID := uint(0)
		if existing.AssigneeUserID != nil {
			existingID = *existing.AssigneeUserID
		}
		requestedID := uint(0)
		if requestedAssignee.Value != nil {
			requestedID = *requestedAssignee.Value
		}
		payload.AssignmentTargetChange = existingID != requestedID
	}
	return payload, nil
}

func firstQualityNote(values ...*string) string {
	for _, value := range values {
		if value == nil {
			continue
		}
		if normalized := strings.TrimSpace(*value); normalized != "" {
			return normalized
		}
	}
	return ""
}
