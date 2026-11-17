package toolruntime

func pmProjectOperationSpecs() []pmOperationSpec {
	const read = "pm_native_project_read"
	const write = "pm_native_project_write"
	id := pmInteger("id", "PM 资源 ID", true)
	page := []pmCatalogField{pmInteger("limit", "最大返回数", false), pmNonNegativeInteger("offset", "偏移量", false)}
	projectBody := []pmCatalogField{
		pmInteger("team_id", "团队 ID", false), pmStringMax("name", "项目集名称", 160, false),
		pmString("code", "项目代号", false), pmString("description", "项目说明", false),
		pmNullable(pmInteger("owner_user_id", "负责人用户 ID", false)), pmNullable(pmString("planned_start_at", "计划开始时间 RFC3339", false)),
		pmNullable(pmString("planned_end_at", "计划结束时间 RFC3339", false)), pmString("status", "项目状态", false),
	}
	planBody := []pmCatalogField{
		pmInteger("project_new_id", "项目集 ID", false), pmStringMax("title", "项目/版本名称", 160, false),
		pmEnum("type", "类型", []string{"project", "version"}, false), pmString("goal", "目标", false),
		pmNullable(pmInteger("owner_user_id", "负责人用户 ID", false)), pmNullable(pmString("planned_start_at", "计划开始时间 RFC3339", false)),
		pmNullable(pmString("planned_end_at", "计划结束时间 RFC3339", false)), pmString("risk_level", "风险等级", false),
		pmInteger("workflow_template_id", "已发布工作流模板 ID", false),
	}
	requirementBody := []pmCatalogField{
		pmInteger("version_task_new_id", "项目/版本 ID", false), pmStringMax("title", "需求标题", 160, false),
		pmString("description", "需求说明", false), pmString("product_doc_url", "产品文档 URL", false),
		pmNullable(pmInteger("product_owner_user_id", "产品负责人用户 ID", false)), pmString("design_url", "设计文档 URL", false),
		pmNullable(pmInteger("design_owner_user_id", "设计负责人用户 ID", false)),
		pmArray("developer_user_ids", "开发人员用户 ID", map[string]any{"type": "integer", "minimum": 1}, false),
		pmString("acceptance_criteria", "验收标准", false), pmString("priority", "优先级", false),
		pmInteger("workflow_template_id", "已发布工作流模板 ID", false),
	}
	requirementRoleOwnersBody := []pmCatalogField{
		pmArray("product_owner_user_ids", "产品负责人用户 ID", map[string]any{"type": "integer", "minimum": 1}, false),
		pmArray("design_owner_user_ids", "设计负责人用户 ID", map[string]any{"type": "integer", "minimum": 1}, false),
		pmArray("frontend_owner_user_ids", "前端负责人用户 ID", map[string]any{"type": "integer", "minimum": 1}, false),
		pmArray("backend_owner_user_ids", "后端负责人用户 ID", map[string]any{"type": "integer", "minimum": 1}, false),
		pmArray("tester_user_ids", "测试负责人用户 ID", map[string]any{"type": "integer", "minimum": 1}, false),
		pmArray("developer_user_ids", "开发人员用户 ID", map[string]any{"type": "integer", "minimum": 1}, false),
	}
	moveItems := pmArray("items", "待移动需求及 CAS version", map[string]any{
		"type": "object", "properties": map[string]any{
			"requirement_id":   map[string]any{"type": "integer", "minimum": 1},
			"expected_version": map[string]any{"type": "integer", "minimum": 1},
		}, "required": []string{"requirement_id", "expected_version"}, "additionalProperties": false,
	}, true)
	moveItems.schema["minItems"], moveItems.schema["maxItems"] = 1, 50

	return []pmOperationSpec{
		pmActivatable(pmSpec("project.board.get", read, "GET", "project-management-board", "查询项目管理看板", "L0", pmInteger("project_new_id", "项目集 ID", true), pmString("include", "包含的资源类型", false))),
		pmActivatable(pmSpec("project.scope_options.list", read, "GET", "project-requirement-scope-options", "查询项目集、项目/版本和需求范围", "L0")),
		pmActivatable(pmSpec("project.set.list", read, "GET", "project-sets", "查询项目集", "L0", append([]pmCatalogField{pmInteger("team_id", "团队 ID", false), pmString("q", "名称或代号关键词", false)}, page...)...)),
		pmActivatable(pmSpec("project.set.get", read, "GET", "project-sets/{id}", "查询项目集详情", "L0", id)),
		pmActivatable(pmSpec("project.set.audit.list", read, "GET", "project-sets/{id}/audit-logs", "查询项目集审计记录", "L0", append([]pmCatalogField{id}, page...)...)),
		pmActivatable(pmSpec("project.set.create", write, "POST", "project-sets", "创建项目集", "L1", pmRequired(projectBody, "team_id", "name")...)),
		pmActivatable(pmSpec("project.set.update", write, "PATCH", "project-sets/{id}", "更新项目集", "L1", append([]pmCatalogField{id}, pmWithout(projectBody, "code")...)...)),
		pmActivatable(pmSpec("project.set.delete", write, "DELETE", "project-sets/{id}", "删除项目集", "L4", id)),
		pmActivatable(pmSpec("project.set.transfer_owner", write, "POST", "project-sets/{id}/transfer-owner-new", "转移项目集负责人", "L3", id, pmInteger("new_owner_user_id", "新负责人用户 ID", true), pmString("transfer_reason", "转移原因", false))),
		pmActivatable(pmSpec("project.plan_unit.list", read, "GET", "project-new", "查询项目与版本", "L0", pmInteger("project_new_id", "项目集 ID", true))),
		pmActivatable(pmSpec("project.plan_unit.get", read, "GET", "project-new/{id}", "查询项目/版本详情", "L0", id)),
		pmActivatable(pmSpec("project.plan_unit.create", write, "POST", "project-new", "创建项目/版本并绑定工作流", "L1", pmRequired(planBody, "project_new_id", "title", "workflow_template_id")...)),
		pmActivatable(pmSpec("project.plan_unit.update", write, "PATCH", "project-new/{id}", "更新项目/版本", "L1", append([]pmCatalogField{id}, pmWithout(planBody, "project_new_id", "workflow_template_id")...)...)),
		pmActivatable(pmSpec("project.plan_unit.delete", write, "DELETE", "project-new/{id}", "删除项目/版本", "L4", id)),
		pmActivatable(pmSpec("project.delivery_label.list", read, "GET", "version-tasks-new/{id}/delivery-node-labels", "查询交付标签", "L0", id)),
		pmActivatable(pmSpec("project.delivery_label.create", write, "POST", "version-tasks-new/{id}/delivery-node-labels", "创建交付标签", "L1", id, pmStringMax("name", "标签名称", 100, true), pmString("client_key", "幂等标识", false), pmString("system_key", "系统键", false), pmNonNegativeInteger("sort_order", "排序", false))),
		pmActivatable(pmSpec("project.delivery_label.update", write, "PATCH", "project-delivery-node-labels/{id}", "更新交付标签", "L1", id, pmStringMax("name", "标签名称", 100, true))),
		pmActivatable(pmSpec("project.delivery_label.delete", write, "DELETE", "project-delivery-node-labels/{id}", "软删除交付标签", "L4", id)),
		pmActivatable(pmSpec("project.requirement.list", read, "GET", "product-requirements-new", "查询产品需求", "L0", append([]pmCatalogField{pmInteger("project_new_id", "项目集 ID", false), pmInteger("version_task_new_id", "项目/版本 ID", false), pmString("q", "标题关键词", false)}, page...)...)),
		pmActivatable(pmSpec("project.requirement.get", read, "GET", "product-requirements-new/{id}", "查询产品需求详情", "L0", id)),
		pmActivatable(pmSpec("project.requirement.create", write, "POST", "product-requirements-new", "创建产品需求", "L1", pmRequired(requirementBody, "version_task_new_id", "title", "workflow_template_id")...)),
		pmActivatable(pmSpec("project.requirement.update", write, "PATCH", "product-requirements-new/{id}", "更新产品需求", "L1", append(append(append([]pmCatalogField{id}, pmWithout(requirementBody, "workflow_template_id", "product_owner_user_id", "design_owner_user_id", "developer_user_ids")...), pmInteger("expected_version", "CAS version", true)), pmBool("confirm_subject_schedule", "确认推导排期", false))...)),
		pmActivatable(pmSpec("project.requirement.role_owners.assign", write, "PATCH", "requirements/{id}/role-owners", "分配需求角色负责人", "L1", append(append([]pmCatalogField{id}, requirementRoleOwnersBody...), pmInteger("expected_version", "CAS version", true))...)),
		pmActivatable(pmSpec("project.requirement.batch_move", write, "POST", "requirements/batch-move", "原子批量移动需求", "L2", moveItems, pmInteger("version_task_new_id", "目标版本 ID", true), pmBool("confirm_subject_schedule", "确认推导排期", false))),
		pmActivatable(pmSpec("project.requirement.delete", write, "DELETE", "requirements/{id}", "级联删除产品需求", "L4", id)),
	}
}
