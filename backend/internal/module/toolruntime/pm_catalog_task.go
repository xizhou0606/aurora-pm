package toolruntime

func pmTaskOperationSpecs() []pmOperationSpec {
	const read = "pm_native_task_read"
	const write = "pm_native_task_write"
	id := pmInteger("id", "任务 ID", true)
	scope := []pmCatalogField{pmInteger("project_set_id", "项目集 ID", false), pmInteger("project_new_id", "项目/版本 ID", false), pmInteger("requirement_id", "需求 ID", false)}
	taskReference := []pmCatalogField{
		pmString("task_ref", "任务号或完整标题", false), pmInteger("id", "任务 ID", false),
		pmInteger("project_set_id", "项目集 ID，用于标题消歧", false), pmInteger("project_new_id", "项目/版本 ID，用于标题消歧", false),
		pmInteger("requirement_id", "需求 ID，用于标题消歧", false),
	}
	list := append([]pmCatalogField{
		pmArray("ids", "任务 ID", map[string]any{"type": "integer", "minimum": 1}, false),
		pmInteger("assignee_id", "负责人用户 ID", false), pmInteger("created_by_user_id", "创建人用户 ID", false),
		pmInteger("task_client_id", "所属端目录 ID", false),
		pmString("created_from", "创建时间下界 RFC3339", false), pmString("created_to", "创建时间上界 RFC3339", false),
		pmInteger("parent_id", "父任务 ID", false), pmString("q", "标题、编号或描述关键词", false),
		pmString("status", "状态列表", false), pmString("priority", "优先级列表", false), pmString("kind", "任务类型列表", false),
		pmEnum("due_bucket", "截止时间分组", []string{"overdue", "due_soon", "later", "unset"}, false),
		pmBool("unfinished", "仅未完成任务", false), pmEnum("sort", "排序", []string{"created_at_desc", "created_at_asc", "updated_at_desc", "updated_at_asc"}, false),
		pmEnum("group_by", "分组", []string{"requirement_id", "assignee_id", "priority"}, false), pmEnum("hierarchy", "层级模式", []string{"filtered_tree"}, false),
		pmInteger("limit", "最大返回数", false), pmNonNegativeInteger("offset", "偏移量", false),
	}, scope...)
	referenceTaskRef := pmString("reference_task_ref", "参考任务号或完整标题", false)
	referenceTaskRef.schema["minLength"] = 1
	taskTitle := pmStringMax("title", "任务标题", 160, true)
	taskTitle.schema["minLength"] = 1
	create := []pmCatalogField{
		referenceTaskRef, pmInteger("reference_task_id", "参考任务 ID", false),
		pmInteger("requirement_id", "需求 ID", false), taskTitle, pmString("description", "任务描述", false),
		pmNullable(pmInteger("task_client_id", "所属端目录 ID", false)),
		pmEnum("kind", "任务类型", []string{"task", "subtask"}, false), pmEnum("status", "状态", []string{"draft", "in_dev", "pending_test", "completed", "cancelled"}, false),
		pmEnum("priority", "优先级", []string{"low", "normal", "medium", "high", "urgent"}, false),
		pmNullable(pmInteger("assignee_id", "负责人用户 ID", false)), pmNullable(pmInteger("reporter_user_id", "报告人用户 ID", false)),
		pmNullable(pmInteger("parent_id", "父任务 ID", false)), pmNullable(pmNonNegativeInteger("estimated_minutes", "预计工时分钟", false)),
		pmNullable(pmDateTime("starts_at", "开始时间 RFC3339", false)), pmNullable(pmDateTime("due_at", "截止时间 RFC3339", false)),
	}
	updates := append([]pmCatalogField{id}, create...)
	for index := range updates {
		if updates[index].name == "title" {
			updates[index].required = false
		}
	}
	statusUpdate := []pmCatalogField{
		id,
		pmEnum("status", "目标状态", []string{"draft", "in_dev", "pending_test", "completed", "cancelled"}, true),
	}
	workflowScope := []pmCatalogField{pmInteger("team_id", "团队 ID", false), pmInteger("project_id", "项目/版本 ID", false)}
	batchTaskProperties := map[string]any{
		"client_id": map[string]any{"type": "string", "maxLength": 100}, "title": map[string]any{"type": "string", "minLength": 1, "maxLength": 160},
		"description":    map[string]any{"type": "string"},
		"status":         map[string]any{"type": "string", "enum": []string{"draft", "in_dev", "pending_test", "completed", "cancelled"}},
		"priority":       map[string]any{"type": "string", "enum": []string{"low", "normal", "medium", "high", "urgent"}},
		"task_client_id": map[string]any{"type": "integer", "minimum": 1},
		"assignee_id":    map[string]any{"type": "integer", "minimum": 1}, "reporter_user_id": map[string]any{"type": "integer", "minimum": 1},
		"estimated_minutes": map[string]any{"type": "integer", "minimum": 0},
		"starts_at":         map[string]any{"anyOf": []any{map[string]any{"type": "string", "format": "date-time"}, map[string]any{"type": "null"}}},
		"due_at":            map[string]any{"anyOf": []any{map[string]any{"type": "string", "format": "date-time"}, map[string]any{"type": "null"}}},
	}
	childSchema := map[string]any{
		"type": "object", "properties": batchTaskProperties,
		"required": []string{"client_id", "title"}, "additionalProperties": false,
	}
	children := pmArray("children", "待创建的子任务", childSchema, true)
	children.schema["maxItems"] = 200
	parentProperties := make(map[string]any, len(batchTaskProperties)+2)
	for key, value := range batchTaskProperties {
		parentProperties[key] = value
	}
	parentProperties["requirement_id"] = map[string]any{"type": "integer", "minimum": 1}
	parentProperties["children"] = children.schema
	parentSchema := map[string]any{
		"type": "object", "properties": parentProperties,
		"required": []string{"client_id", "requirement_id", "title"}, "additionalProperties": false,
	}
	tasks := pmArray("tasks", "待创建的父任务及子任务层级", parentSchema, true)
	tasks.schema["maxItems"] = 50
	batchIDs := pmArray("ids", "任务 ID", map[string]any{"type": "integer", "minimum": 1}, false)
	batchIDs.schema["maxItems"] = 500
	batchRefs := pmArray("task_refs", "任务号或完整标题", map[string]any{"type": "string"}, false)
	batchUpdates := pmObject("updates", "允许批量更新的字段", map[string]any{
		"title": map[string]any{"type": "string", "maxLength": 160}, "description": map[string]any{"type": "string"},
		"status":            map[string]any{"type": "string", "enum": []string{"draft", "in_dev", "pending_test", "completed", "cancelled"}},
		"priority":          map[string]any{"type": "string", "enum": []string{"low", "normal", "medium", "high", "urgent"}},
		"task_client_id":    map[string]any{"anyOf": []any{map[string]any{"type": "integer", "minimum": 1}, map[string]any{"type": "null"}}},
		"assignee_id":       map[string]any{"anyOf": []any{map[string]any{"type": "integer", "minimum": 1}, map[string]any{"type": "null"}}},
		"reporter_user_id":  map[string]any{"anyOf": []any{map[string]any{"type": "integer", "minimum": 1}, map[string]any{"type": "null"}}},
		"estimated_minutes": map[string]any{"anyOf": []any{map[string]any{"type": "integer", "minimum": 0}, map[string]any{"type": "null"}}},
		"starts_at":         map[string]any{"anyOf": []any{map[string]any{"type": "string", "format": "date-time"}, map[string]any{"type": "null"}}},
		"due_at":            map[string]any{"anyOf": []any{map[string]any{"type": "string", "format": "date-time"}, map[string]any{"type": "null"}}},
	}, true)
	batchUpdates.schema["minProperties"] = 1
	batchStatusUpdates := pmObject("updates", "批量流转目标状态", map[string]any{
		"status": map[string]any{"type": "string", "enum": []string{"draft", "in_dev", "pending_test", "completed", "cancelled"}},
	}, true)
	batchStatusUpdates.schema["required"] = []string{"status"}

	return []pmOperationSpec{
		pmActivatable(pmSpec("task.client_option.list", read, "GET", "tasks/client-options", "查询任务所属端目录", "L0")),
		pmActivatable(pmSpec("task.workflow.status.list", read, "GET", "issue-statuses", "查询任务工作流状态事实", "L0", append(workflowScope, pmEnum("kind", "工作项类型", []string{"task", "subtask"}, true))...)),
		pmActivatable(pmSpec("task.workflow.transition.list", read, "GET", "issue-workflow-transitions", "查询任务工作流迁移事实", "L0", append(workflowScope, pmEnum("kind", "工作项类型", []string{"task", "subtask"}, true))...)),
		pmActivatable(pmSpec("task.workflow.kind.list", read, "GET", "issue-kinds", "查询工作项类型事实", "L0", workflowScope...)),
		pmActivatable(pmSpec("task.list", read, "GET", "tasks", "查询任务", "L0", list...)),
		pmActivatable(pmSpec("task.get", read, "GET", "tasks/{id}", "查询任务详情", "L0", taskReference...)),
		pmActivatable(pmSpec("task.create", write, "POST", "tasks", "创建任务", "L1", create...)),
		pmActivatable(pmSpec("task.update", write, "PATCH", "tasks/{id}", "更新任务", "L1", updates...)),
		pmActivatable(pmSpec("task.status.update", write, "PATCH", "tasks/{id}/status", "流转任务状态", "L1", statusUpdate...)),
		pmActivatable(pmSpec("task.delete", write, "DELETE", "tasks/{id}", "删除任务", "L4", id, pmBool("cascade", "级联删除子任务", false))),
		pmActivatable(pmSpec("task.stats.overview", read, "GET", "tasks/stats", "查询任务统计", "L0", list...)),
		pmActivatable(pmSpec("task.stats.delivery", read, "GET", "tasks/stats/delivery", "查询任务交付统计", "L0", list...)),
		pmActivatable(pmSpec("task.stats.due_completion", read, "GET", "tasks/stats/due-completion", "查询按期完成统计", "L0", list...)),
		pmActivatable(pmSpec("task.stats.assignee_status", read, "GET", "tasks/stats/by-assignee-status", "查询负责人和状态统计", "L0", list...)),
		pmActivatable(pmSpec("task.comment.list", read, "GET", "tasks/{id}/comments", "查询任务评论", "L0", id)),
		pmActivatable(pmSpec("task.comment.create", write, "POST", "tasks/{id}/comments", "新增任务评论", "L1", id, pmStringMax("body", "评论内容", 20000, true))),
		pmActivatable(pmSpec("task.comment.delete", write, "DELETE", "tasks/{id}/comments/{cid}", "删除任务评论", "L4", id, pmInteger("cid", "评论 ID", true))),
		pmActivatable(pmSpec("task.activity.list", read, "GET", "tasks/{id}/activities", "查询任务活动", "L0", id, pmInteger("limit", "最大返回数", false), pmNonNegativeInteger("offset", "偏移量", false))),
		pmActivatable(pmSpec("task.attachment.list", read, "GET", "tasks/{id}/attachments", "查询任务附件", "L0", id)),
		pmActivatable(pmSpec("task.attachment.delete", write, "DELETE", "tasks/attachments/{aid}", "删除任务附件", "L4", pmInteger("aid", "附件 ID", true), id)),
		pmActivatable(pmSpec("task.git_link.list", read, "GET", "tasks/{id}/git-links", "查询任务 Git 关联", "L0", id)),
		pmActivatable(pmSpec("task.related.list", read, "GET", "tasks/{id}/related", "查询相关任务", "L0", id)),
		pmActivatable(pmSpec("task.due_conflict.list", read, "GET", "tasks/{id}/due-at-descendant-conflicts", "查询子任务截止日期冲突", "L0", id, pmString("due_at", "拟设置截止时间 RFC3339", true))),
		pmActivatable(pmSpec("task.watch.update", write, "PUT", "tasks/{id}/watch", "关注或取消关注任务", "L1", id, pmBool("watch", "是否关注", true))),
		pmActivatable(pmSpec("task.batch_create", write, "POST", "tasks/batch-create", "原子批量创建父任务及子任务", "L1", tasks)),
		pmActivatable(pmSpec("task.children.batch_create", write, "POST", "tasks/{id}/children/batch", "原子批量创建子任务", "L1", id, children)),
		pmActivatable(pmSpec("task.clone", write, "POST", "tasks/{id}/clone", "克隆任务", "L1", id, pmStringMax("title_suffix", "标题后缀", 160, false), pmInteger("target_requirement_id", "目标需求 ID", false))),
		pmActivatable(pmSpec("task.batch_delete", write, "POST", "tasks/batch-delete", "批量删除任务", "L4", append([]pmCatalogField{batchIDs, batchRefs, pmBool("cascade", "级联删除所选任务的子任务", false)}, scope...)...)),
		pmActivatable(pmSpec("task.batch_update", write, "POST", "tasks/batch-update", "原子批量更新任务", "L3", append([]pmCatalogField{batchIDs, batchRefs, batchUpdates}, scope...)...)),
		pmActivatable(pmSpec("task.status.batch_update", write, "POST", "tasks/batch-status-update", "原子批量流转任务状态", "L3", append([]pmCatalogField{batchIDs, batchRefs, batchStatusUpdates}, scope...)...)),
	}
}
