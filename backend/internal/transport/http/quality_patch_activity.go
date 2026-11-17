package httpapi

import (
	"strconv"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	"gorm.io/gorm"
)

type qualityPatchChange struct {
	Field    string
	OldValue string
	NewValue string
	Summary  string
}

func recordQualityPatchActivity(
	tx *gorm.DB,
	before qualitygorm.QualityBug,
	after qualitygorm.QualityBug,
	actorID uint,
	payload qualityWorkflowPayload,
) (string, error) {
	oldStatus := normalizeQualityBugStatus(before.Status)
	newStatus := normalizeQualityBugStatus(after.Status)
	statusChanged := oldStatus != newStatus
	backfilledResolution := newStatus == "review" && oldStatus == "review" &&
		strings.TrimSpace(before.ResolutionReason) == "" && strings.TrimSpace(after.ResolutionReason) != ""
	skip := qualityPatchWorkflowSkip(oldStatus, newStatus, backfilledResolution)
	if statusChanged && payload.AssignmentTargetChange {
		skip["assignee_user_id"] = true
	}
	changes := collectQualityPatchChanges(before, after, skip)
	action, field, oldValue, newValue, summary := "", "", "", "", ""
	eventAt := time.Now().UTC()

	switch {
	case newStatus == "review" && (oldStatus != "review" || backfilledResolution):
		parts := []string{"解决"}
		if after.ResolutionReason != "" {
			parts = append(parts, "解决方案："+qualityResolutionReasonLabel(after.ResolutionReason))
		}
		if after.ReviewerUserID != nil && *after.ReviewerUserID > 0 {
			parts = append(parts, "已指定验收人")
		}
		if urls := qualityMaterialURLs(decodeQualityMaterials(after.ResolutionMaterialsJSON)); len(urls) > 0 {
			parts = append(parts, "附件："+strconv.Itoa(len(urls))+"个", "材料："+strings.Join(urls, "、"))
		}
		parts = appendQualityPatchChanges(parts, changes)
		if strings.TrimSpace(after.ResolutionNote) != "" {
			parts = append(parts, "备注："+strings.TrimSpace(after.ResolutionNote))
		}
		action, field, oldValue, newValue = "resolved", "status", oldStatus, "review"
		if backfilledResolution {
			field, oldValue, newValue = "resolution_reason", "", after.ResolutionReason
		}
		summary = strings.Join(parts, "；")
	case payload.IsRegressionReturn && (newStatus == "reopened" || newStatus == "in_progress"):
		parts := []string{"重新打开"}
		if payload.AssignmentTargetChange && after.AssigneeUserID != nil {
			parts = append(parts, "已指派负责人")
		}
		if urls := qualityMaterialURLs(payload.RegressionMaterials); len(urls) > 0 {
			parts = append(parts, "附件："+strconv.Itoa(len(urls))+"个", "材料："+strings.Join(urls, "、"))
		}
		parts = appendQualityPatchChanges(parts, changes)
		if payload.RegressionNote != "" {
			parts = append(parts, "备注："+payload.RegressionNote)
		}
		action, field, oldValue, newValue = "returned", "status", "review", newStatus
		summary = strings.Join(parts, "；")
	case statusChanged && newStatus == "reopened":
		parts := []string{"重新打开"}
		if payload.AssignmentTargetChange && after.AssigneeUserID != nil {
			parts = append(parts, "已指派负责人")
		}
		if urls := qualityMaterialURLs(payload.RegressionMaterials); len(urls) > 0 {
			parts = append(parts, "附件："+strconv.Itoa(len(urls))+"个", "材料："+strings.Join(urls, "、"))
		}
		parts = appendQualityPatchChanges(parts, changes)
		if payload.RegressionNote != "" {
			parts = append(parts, "备注："+payload.RegressionNote)
		}
		action, field, oldValue, newValue = "returned", "status", oldStatus, newStatus
		summary = strings.Join(parts, "；")
	case statusChanged && newStatus == "in_progress":
		parts := []string{"确认"}
		if payload.AssignmentTargetChange && after.AssigneeUserID != nil {
			parts = append(parts, "已指派负责人")
		}
		parts = appendQualityPatchChanges(parts, changes)
		if payload.AssignmentNote != "" {
			parts = append(parts, "备注："+payload.AssignmentNote)
		}
		action, field, oldValue, newValue = "confirmed", "status", oldStatus, newStatus
		summary = strings.Join(parts, "；")
	case statusChanged && newStatus == "done":
		parts := []string{"验证通过"}
		if after.ReviewerUserID != nil {
			parts = append(parts, "已记录验收人")
		}
		if urls := qualityMaterialURLs(payload.ResolutionMaterials); len(urls) > 0 {
			parts = append(parts, "附件："+strconv.Itoa(len(urls))+"个", "材料："+strings.Join(urls, "、"))
		}
		parts = appendQualityPatchChanges(parts, changes)
		if payload.ResolutionNote != "" {
			parts = append(parts, "备注："+payload.ResolutionNote)
		}
		action, field, oldValue, newValue = "verified", "status", oldStatus, newStatus
		summary = strings.Join(parts, "；")
	case statusChanged && newStatus == "triage":
		parts := []string{"恢复待确认", "来源：" + qualityStatusLabel(oldStatus)}
		parts = appendQualityPatchChanges(parts, changes)
		action, field, oldValue, newValue = "restored", "status", oldStatus, newStatus
		summary = strings.Join(parts, "；")
	case statusChanged && newStatus == "canceled":
		parts := appendQualityPatchChanges([]string{"取消"}, changes)
		action = "canceled"
		field, oldValue, newValue, summary = "status", oldStatus, newStatus, strings.Join(parts, "；")
	case statusChanged:
		parts := []string{"状态流转", "状态：" + qualityStatusLabel(oldStatus) + " → " + qualityStatusLabel(newStatus)}
		parts = appendQualityPatchChanges(parts, changes)
		action, field, oldValue, newValue = "status_transition", "status", oldStatus, newStatus
		summary = strings.Join(parts, "；")
	case len(changes) > 0 && !qualityPatchChangesOnlyAssignee(changes):
		parts := appendQualityPatchChanges([]string{"编辑"}, changes)
		if payload.AssignmentNote != "" {
			parts = append(parts, "备注："+payload.AssignmentNote)
		}
		action, field, summary = "updated", "multiple_fields", strings.Join(parts, "；")
	case payload.shouldRecordAssignment():
		parts := []string{"指派"}
		if after.AssigneeUserID != nil {
			parts = append(parts, "已更新负责人")
		}
		if len(payload.CCUserIDs) > 0 {
			parts = append(parts, "已更新抄送人")
		}
		if urls := qualityMaterialURLs(payload.AssignmentMaterials); len(urls) > 0 {
			parts = append(parts, "附件："+strconv.Itoa(len(urls))+"个", "材料："+strings.Join(urls, "、"))
		}
		if payload.AssignmentNote != "" {
			parts = append(parts, "备注："+payload.AssignmentNote)
		}
		action, field = "assigned", "assignee_user_id"
		oldValue, newValue = qualityUintPtrValue(before.AssigneeUserID), qualityUintPtrValue(after.AssigneeUserID)
		summary = strings.Join(parts, "；")
	}
	if action == "" {
		return "", nil
	}
	if statusChanged && payload.AssignmentTargetChange {
		if err := tx.Create(&qualitygorm.QualityBugActivity{
			WorkspaceID: before.WorkspaceID, QualityBugID: before.ID, UserID: actorID,
			Action: "assigned", FieldKey: "assignee_user_id",
			OldValue: qualityUintPtrValue(before.AssigneeUserID), NewValue: qualityUintPtrValue(after.AssigneeUserID),
			Summary: "更新负责人", CreatedAt: eventAt,
		}).Error; err != nil {
			return "", err
		}
	}
	if strings.TrimSpace(before.ResolutionReason) != strings.TrimSpace(after.ResolutionReason) &&
		field != "resolution_reason" {
		if err := tx.Create(&qualitygorm.QualityBugActivity{
			WorkspaceID: before.WorkspaceID, QualityBugID: before.ID, UserID: actorID,
			Action: "resolution_decided", FieldKey: "resolution_reason",
			OldValue: before.ResolutionReason, NewValue: after.ResolutionReason,
			Summary: "更新解决方案", CreatedAt: eventAt,
		}).Error; err != nil {
			return "", err
		}
	}
	row := qualitygorm.QualityBugActivity{
		WorkspaceID: before.WorkspaceID, QualityBugID: before.ID, UserID: actorID,
		Action: action, FieldKey: field, OldValue: oldValue, NewValue: newValue, Summary: summary, CreatedAt: eventAt,
	}
	return summary, tx.Create(&row).Error
}

func collectQualityPatchChanges(before, after qualitygorm.QualityBug, skip map[string]bool) []qualityPatchChange {
	changes := []qualityPatchChange{}
	record := func(field, oldValue, newValue string) {
		if skip[field] || oldValue == newValue {
			return
		}
		summary := qualityPatchFieldLabel(field) + "：" +
			qualityPatchDisplayValue(field, oldValue) + " → " + qualityPatchDisplayValue(field, newValue)
		if field == "assignee_user_id" || field == "reviewer_user_id" {
			summary = "已更新" + qualityPatchFieldLabel(field)
		}
		changes = append(changes, qualityPatchChange{
			Field: field, OldValue: oldValue, NewValue: newValue,
			Summary: summary,
		})
	}
	record("title", before.Title, after.Title)
	record("status", normalizeQualityBugStatus(before.Status), normalizeQualityBugStatus(after.Status))
	record("priority", before.Priority, after.Priority)
	record("bug_type", before.BugType, after.BugType)
	record("defect_domain", quality.NormalizeBugDefectDomain(before.DefectDomain), quality.NormalizeBugDefectDomain(after.DefectDomain))
	record("due_date", qualityTimePtrValue(before.DueAt), qualityTimePtrValue(after.DueAt))
	record("severity_key", before.SeverityKey, after.SeverityKey)
	record("found_environment", before.FoundEnvironment, after.FoundEnvironment)
	record("operating_system", before.OperatingSystem, after.OperatingSystem)
	record("browser", before.Browser, after.Browser)
	record("found_version", before.FoundVersion, after.FoundVersion)
	record("resolution_reason", before.ResolutionReason, after.ResolutionReason)
	record("resolution_note", before.ResolutionNote, after.ResolutionNote)
	record("assignee_user_id", qualityUintPtrValue(before.AssigneeUserID), qualityUintPtrValue(after.AssigneeUserID))
	record("reviewer_user_id", qualityUintPtrValue(before.ReviewerUserID), qualityUintPtrValue(after.ReviewerUserID))
	record("requirement_id", strconv.FormatUint(uint64(before.RequirementID), 10), strconv.FormatUint(uint64(after.RequirementID), 10))
	record("reproduction_steps", before.ReproductionSteps, after.ReproductionSteps)
	record("expected_result", before.ExpectedResult, after.ExpectedResult)
	record("actual_result", before.ActualResult, after.ActualResult)
	record("reproduction_materials", qualityMaterialsValue(before.ReproductionMaterialsJSON), qualityMaterialsValue(after.ReproductionMaterialsJSON))
	record("resolution_materials", qualityMaterialsValue(before.ResolutionMaterialsJSON), qualityMaterialsValue(after.ResolutionMaterialsJSON))
	return changes
}

func qualityTimePtrValue(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
