package toolruntime

func pmWorkflowOperationSpecs() []pmOperationSpec {
	const read = "pm_native_workflow_read"
	const write = "pm_native_workflow_write"
	id := pmInteger("id", "工作流实例 ID", true)
	nodeID := pmString("nodeId", "工作流节点 ID", true)
	subjectType := pmEnum("subject_type", "业务对象类型", []string{"plan_unit", "requirement"}, true)
	nextAssignees := pmArray("next_node_assignees", "下一节点负责人", map[string]any{
		"type": "object", "properties": map[string]any{
			"node_id":          map[string]any{"type": "string", "minLength": 1},
			"assignee_user_id": map[string]any{"type": "integer", "minimum": 1},
		}, "required": []string{"node_id", "assignee_user_id"}, "additionalProperties": false,
	}, false)
	nodes := pmArray("nodes", "节点负责人和排期 CAS 更新", map[string]any{
		"type": "object", "properties": map[string]any{
			"node_id":                   map[string]any{"type": "string", "minLength": 1},
			"planned_start_at":          map[string]any{"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "null"}}},
			"planned_end_at":            map[string]any{"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "null"}}},
			"expected_planned_start_at": map[string]any{"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "null"}}},
			"expected_planned_end_at":   map[string]any{"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "null"}}},
			"assignee_user_id":          map[string]any{"type": "integer", "minimum": 1},
		}, "required": []string{"node_id"}, "additionalProperties": false,
	}, true)
	return []pmOperationSpec{
		pmActivatable(pmSpec("workflow.template.list", read, "GET", "pm/workflows/templates", "按业务对象类型查询工作流模板", "L0", pmEnum("subject_type", "模板业务对象类型", []string{"plan_unit", "requirement"}, false))),
		pmActivatable(pmSpec("workflow.template.get", read, "GET", "pm/workflows/templates/{id}", "查询工作流模板定义和发布版本", "L0", pmInteger("id", "工作流模板 ID", true))),
		pmActivatable(pmSpec("workflow.instance.list", read, "GET", "pm/workflows/instances", "按业务对象查询工作流实例", "L0", subjectType, pmInteger("subject_id", "业务对象 ID", true))),
		pmActivatable(pmSpec("workflow.instance.get", read, "GET", "pm/workflows/instances/{id}", "查询工作流实例、动作与活动历史", "L0", id)),
		pmActivatable(pmSpec("workflow.opinion.list", read, "GET", "pm/workflows/instances/{id}/nodes/{nodeId}/opinions", "查询预审节点意见", "L0", id, nodeID)),
		pmActivatable(pmSpec("workflow.test_case_artifact.list", read, "GET", "pm/workflows/instances/{id}/nodes/{nodeId}/test-case-artifacts", "查询测试用例材料", "L0", id, nodeID)),
		pmActivatable(pmSpec("workflow.test_report_artifact.list", read, "GET", "pm/workflows/instances/{id}/nodes/{nodeId}/test-report-artifacts", "查询测试报告材料和链接", "L0", id, nodeID)),
		pmActivatable(pmSpec("workflow.action.execute", write, "POST", "pm/workflows/instances/{id}/actions", "依据 allowed_actions 执行工作流动作", "L1", id,
			pmEnum("action", "动作", []string{"complete", "approve", "reject", "condition_yes", "condition_no", "retry_ai", "recheck_ai", "override_ai", "rollback", "resume", "supplement_design"}, true),
			pmString("node_id", "节点 ID", false), pmString("target_node_id", "回滚目标节点 ID", false), pmString("comment", "动作意见", false), pmString("design_url", "设计稿 URL", false), nextAssignees,
			pmBool("confirm_requirement_lock", "显式确认锁版：完成项目需求录入节点后禁止该项目新建需求，已有需求仍可移入或排期；不能自动填 true，须先向用户呈现锁版后果", false))),
		pmActivatable(pmSpec("workflow.node_plan.update", write, "PATCH", "pm/workflows/instances/{id}/node-plans", "以 CAS 更新节点负责人和日期", "L2", id, nodes, pmBool("confirm_subject_schedule", "确认推导业务对象排期", false))),
		pmActivatable(pmSpec("workflow.opinion.create", write, "POST", "pm/workflows/instances/{id}/nodes/{nodeId}/opinions", "创建预审节点意见或邀请人员", "L1", id, nodeID, pmString("body", "意见正文", false), pmArray("invite_user_ids", "邀请用户 ID", map[string]any{"type": "integer", "minimum": 1}, false))),
		pmActivatable(pmSpec("workflow.opinion.update", write, "PATCH", "pm/workflows/instances/{id}/nodes/{nodeId}/opinions/{opinionId}", "更新当前用户创建的预审意见", "L1", id, nodeID, pmInteger("opinionId", "节点意见 ID", true), pmString("body", "意见正文", true))),
		pmActivatable(pmSpec("workflow.test_report_link.create", write, "POST", "pm/workflows/instances/{id}/nodes/{nodeId}/test-report-artifacts/link", "关联 HTTP(S) 测试报告链接", "L1", id, nodeID, pmString("url", "测试报告 URL", true))),
	}
}
