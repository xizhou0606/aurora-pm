package httpapi

import (
	"strconv"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
)

func qualityPatchWorkflowSkip(oldStatus, newStatus string, backfilledResolution bool) map[string]bool {
	skip := map[string]bool{}
	if oldStatus != newStatus {
		skip["status"] = true
	}
	if newStatus == "review" || backfilledResolution {
		for _, field := range []string{
			"resolution_reason", "resolution_note", "reviewer_user_id", "resolution_materials",
		} {
			skip[field] = true
		}
	}
	if oldStatus == "review" && (newStatus == "reopened" || newStatus == "in_progress") {
		skip["assignee_user_id"] = true
	}
	if newStatus == "done" {
		for _, field := range []string{
			"reviewer_user_id", "resolution_note", "resolution_materials",
		} {
			skip[field] = true
		}
	}
	if newStatus == "triage" {
		for _, field := range []string{
			"resolution_reason", "resolution_note", "reviewer_user_id", "resolution_materials",
		} {
			skip[field] = true
		}
	}
	return skip
}

func appendQualityPatchChanges(parts []string, changes []qualityPatchChange) []string {
	for _, change := range changes {
		if change.Summary != "" {
			parts = append(parts, change.Summary)
		}
	}
	return parts
}

func qualityPatchChangesOnlyAssignee(changes []qualityPatchChange) bool {
	if len(changes) == 0 {
		return true
	}
	for _, change := range changes {
		if change.Field != "assignee_user_id" {
			return false
		}
	}
	return true
}

func qualityPatchFieldLabel(field string) string {
	labels := map[string]string{
		"title": "标题", "status": "状态", "priority": "优先级", "bug_type": "Bug类型", "defect_domain": "缺陷领域",
		"due_date": "截止日期", "severity_key": "严重级别", "found_environment": "发现环境",
		"operating_system": "操作系统", "browser": "浏览器", "found_version": "发现版本",
		"requirement_id": "关联需求", "resolution_reason": "解决方案", "resolution_note": "备注",
		"resolution_materials": "附件", "reproduction_steps": "复现步骤", "expected_result": "期望结果",
		"actual_result": "实际结果", "reproduction_materials": "复现材料",
		"assignee_user_id": "开发负责人", "reviewer_user_id": "回归验证人",
	}
	if label := labels[field]; label != "" {
		return label
	}
	return field
}

func qualityPatchDisplayValue(field, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" {
		return "（空）"
	}
	switch field {
	case "status":
		return qualityStatusLabel(raw)
	case "priority":
		return qualityPriorityLabel(raw)
	case "severity_key":
		return qualitySeverityLabel(raw)
	case "bug_type":
		return qualityTypeLabel(raw)
	case "resolution_reason":
		return qualityResolutionReasonLabel(raw)
	default:
		return raw
	}
}

func qualityStatusLabel(value string) string {
	return quality.StatusLabel(value)
}

func qualityPriorityLabel(value string) string {
	labels := map[string]string{"low": "低", "medium": "中", "high": "高", "urgent": "紧急"}
	if label := labels[normalizeStatus(value)]; label != "" {
		return label
	}
	return value
}

func qualitySeverityLabel(value string) string {
	return quality.SeverityLabel(value)
}

func qualityTypeLabel(value string) string {
	labels := map[string]string{
		"code_error": "代码错误", "config": "配置相关", "deployment": "安装部署",
		"security": "安全相关", "performance": "性能问题", "standard": "标准规范",
		"test_script": "测试脚本", "design_defect": "设计缺陷", "other": "其他",
	}
	if label := labels[normalizeStatus(value)]; label != "" {
		return label
	}
	return value
}

func qualityMaterialURLs(items []qualityMaterial) []string {
	urls := make([]string, 0, len(items))
	for _, item := range items {
		if url := strings.TrimSpace(item.URL); url != "" {
			urls = append(urls, url)
		}
	}
	return urls
}

func qualityMaterialsValue(raw string) string {
	return strings.Join(qualityMaterialURLs(decodeQualityMaterials(raw)), "、")
}

func qualityUintPtrValue(value *uint) string {
	if value == nil || *value == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(*value), 10)
}

func qualityUintListValue(values []uint) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value > 0 {
			parts = append(parts, strconv.FormatUint(uint64(value), 10))
		}
	}
	return strings.Join(parts, ",")
}

func qualityPatchActivityRow(
	bug qualitygorm.QualityBug,
	actorID uint,
	action, field, oldValue, newValue, summary string,
) qualitygorm.QualityBugActivity {
	return qualitygorm.QualityBugActivity{
		WorkspaceID: bug.WorkspaceID, QualityBugID: bug.ID, UserID: actorID,
		Action: action, FieldKey: field, OldValue: oldValue, NewValue: newValue, Summary: summary,
	}
}
