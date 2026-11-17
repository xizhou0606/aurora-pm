package toolruntime

import "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"

func pmQualityOperationSpecs() []pmOperationSpec {
	const read = "pm_native_quality_read"
	const write = "pm_native_quality_write"
	id := pmInteger("id", "质量 Bug ID", true)
	uploadSize := pmInteger("size", "文件字节数，最大 200MB", true)
	uploadSize.schema["maximum"] = MaxToolFileBytes
	uploadDigest := pmStringMax("sha256", "文件 SHA-256 小写十六进制摘要", 64, true)
	uploadDigest.schema["pattern"] = "^[a-f0-9]{64}$"
	scope := []pmCatalogField{pmInteger("project_set_id", "项目集 ID", false), pmInteger("project_new_id", "项目/版本 ID", false), pmInteger("requirement_id", "需求 ID", false)}
	create := []pmCatalogField{
		pmQualityMaterials("reproduction_materials", "复现附件 URL 列表"), pmQualityMaterials("resolution_materials", "解决附件 URL 列表"),
		pmInteger("requirement_id", "需求 ID", true), pmStringMax("title", "Bug 标题", 160, true),
		pmEnum("status", "状态", []string{"triage", "todo", "in_progress", "reopened", "review", "done", "canceled", "duplicate"}, false),
		pmEnum("priority", "优先级", []string{"low", "medium", "high", "urgent"}, false), pmString("bug_type", "Bug 类型", false),
		pmEnum("defect_domain", "缺陷领域", []string{"functional", "ui"}, false),
		pmEnum("severity_key", "严重级别", []string{"p0", "p1", "p2", "p3"}, false), pmNullable(pmString("due_at", "截止时间 RFC3339", false)),
		pmString("found_environment", "发现环境", false), pmString("operating_system", "操作系统", false), pmString("browser", "浏览器", false),
		pmString("found_version", "发现版本", false), pmString("reproduction_steps", "复现步骤", false), pmString("expected_result", "预期结果", false),
		pmString("actual_result", "实际结果", false), pmEnum("resolution_reason", "解决原因代码或已知中文（如已修复），统一保存标准代码", quality.BugResolutionReasonInputs(), false), pmStringMax("resolution_note", "解决说明", 1000, false),
		pmNullable(pmInteger("assignee_user_id", "负责人用户 ID", false)), pmNullable(pmInteger("reviewer_user_id", "验证人用户 ID", false)),
		pmNullable(pmInteger("source_execution_id", "来源测试执行 ID", false)), pmNullable(pmInteger("source_test_id", "来源测试用例 ID", false)),
	}
	update := append([]pmCatalogField{id}, create...)
	for index := range update {
		if update[index].name == "title" {
			update[index].required = false
		}
	}
	statusUpdate := []pmCatalogField{
		id,
		pmStringMax("regression_note", "重新打开或回归退回的说明（可包含正文图片 URL）", 1000, false),
		pmStringMax("activity_note", "本次状态变更说明", 1000, false),
		pmQualityMaterials("regression_materials", "本次重新打开的图片和附件，保存到流转活动，不覆盖复现正文"),
		pmQualityMaterials("resolution_materials", "本次解决或验收的图片和附件"),
		pmEnum("status", "目标状态", []string{"triage", "todo", "in_progress", "reopened", "review", "done", "canceled"}, true),
		pmEnum("resolution_reason", "解决原因代码或已知中文（如已修复），统一保存标准代码", quality.BugResolutionReasonInputs(), false),
		pmStringMax("resolution_note", "解决说明", 1000, false),
		pmNullable(pmInteger("reviewer_user_id", "验证人用户 ID", false)),
	}
	assignment := []pmCatalogField{
		id,
		pmStringMax("assignment_note", "本次指派说明", 1000, false),
		pmQualityMaterials("assignment_materials", "本次指派的图片和附件"),
		pmInteger("assignee_user_id", "负责人用户 ID", true),
	}
	list := append(scope,
		pmString("q", "标题或编号关键词", false), pmString("status", "状态列表", false), pmString("priority", "优先级列表", false),
		pmString("severity_key", "严重级别列表", false), pmString("found_environment", "发现环境", false), pmString("found_version", "发现版本", false),
		pmString("bug_type", "Bug 类型", false), pmEnum("defect_domain", "缺陷领域", []string{"functional", "ui"}, false),
		pmInteger("current_owner_user_id", "当前处理人 ID", false),
		pmInteger("assignee_id", "负责人 ID", false), pmInteger("reporter_user_id", "报告人 ID", false),
		pmInteger("limit", "最大返回数", false), pmNonNegativeInteger("offset", "偏移量", false))
	bugProperties := map[string]any{"client_id": map[string]any{"type": "string", "minLength": 1, "maxLength": 100}}
	for _, field := range create {
		property := make(map[string]any, len(field.schema)+1)
		for key, value := range field.schema {
			property[key] = value
		}
		property["description"] = field.description
		bugProperties[field.name] = property
	}
	bugItem := map[string]any{
		"type": "object", "properties": bugProperties,
		"required": []string{"client_id", "requirement_id", "title"}, "additionalProperties": false,
	}
	bugs := pmArray("bugs", "待创建 Bug", bugItem, true)
	bugs.schema["maxItems"] = 100
	ids := pmArray("ids", "Bug ID", map[string]any{"type": "integer", "minimum": 1}, true)
	ids.schema["maxItems"] = 100
	specs := []pmOperationSpec{
		pmActivatable(pmSpec("quality.material.upload", write, "POST", "quality/bugs/materials/upload", "上传质量附件或正文图片；先提交元数据，确认时提供文件", "L1", pmInteger("requirement_id", "关联需求 ID", false), pmStringMax("name", "文件名", 255, true), uploadSize, uploadDigest, pmStringMax("content_type", "文件 MIME 类型", 255, true))),
		pmActivatable(pmSpec("quality.bug.list", read, "GET", "quality/bugs", "查询质量 Bug", "L0", list...)),
		pmActivatable(pmSpec("quality.bug.get", read, "GET", "quality/bugs/{id}", "查询质量 Bug 详情", "L0", id)),
		pmActivatable(pmSpec("quality.bug.create", write, "POST", "quality/bugs", "创建质量 Bug", "L1", create...)),
		pmActivatable(pmSpec("quality.bug.update", write, "PATCH", "quality/bugs/{id}", "修改质量 Bug 普通字段；提交解决和重新打开请使用 quality.bug.status.update", "L1", update...)),
		pmActivatable(pmSpec("quality.bug.assign", write, "PATCH", "quality/bugs/{id}/assignment", "指派质量 Bug", "L1", assignment...)),
		pmActivatable(pmSpec("quality.bug.status.update", write, "PATCH", "quality/bugs/{id}/status", "提交解决、重新打开或验证通过；说明与附件随状态原子保存为流转活动", "L1", statusUpdate...)),
		pmActivatable(pmSpec("quality.bug.delete", write, "DELETE", "quality/bugs/{id}", "删除质量 Bug", "L4", id)),
		pmActivatable(pmSpec("quality.bug.activity.list", read, "GET", "quality/bugs/{id}/activities", "查询质量 Bug 活动", "L0",
			id, pmInteger("limit", "最大返回数", false), pmNonNegativeInteger("offset", "偏移量", false))),
		pmActivatable(pmSpec("quality.bug.batch_create", write, "POST", "quality/bugs/batch-create", "原子批量创建质量 Bug", "L1", bugs)),
		pmActivatable(pmSpec("quality.bug.batch_assign", write, "POST", "quality/bugs/batch-assignment", "原子批量指派质量 Bug", "L3", ids, pmInteger("assignee_user_id", "负责人用户 ID", true))),
		pmActivatable(pmSpec("quality.bug.batch_delete", write, "POST", "quality/bugs/batch-delete", "原子批量删除质量 Bug", "L4", ids)),
	}
	for index := range specs {
		switch specs[index].name {
		case "quality.bug.create", "quality.bug.update", "quality.bug.status.update", "quality.bug.batch_create":
			specs[index].version = "1.2.0"
		}
	}
	return specs
}

func pmQualityMaterials(name, description string) pmCatalogField {
	return pmArray(name, description, map[string]any{"type": "object", "properties": map[string]any{"url": map[string]any{"type": "string", "minLength": 1}}, "required": []string{"url"}, "additionalProperties": false}, false)
}
