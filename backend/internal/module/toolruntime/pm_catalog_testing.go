package toolruntime

func pmTestingOperationSpecs() []pmOperationSpec {
	const toolset = "pm_native_task_testing"
	target := []pmCatalogField{pmString("task_ref", "任务号或完整标题", false), pmInteger("id", "任务 ID", false), pmInteger("project_set_id", "项目集 ID", false), pmInteger("project_new_id", "项目/版本 ID", false), pmInteger("requirement_id", "需求 ID", false)}
	caseItem := map[string]any{"type": "object", "properties": map[string]any{
		"key": map[string]any{"type": "string", "maxLength": 100}, "title": map[string]any{"type": "string", "maxLength": 240},
		"steps":           map[string]any{"type": "array", "minItems": 1, "maxItems": 100, "items": map[string]any{"type": "object", "properties": map[string]any{"action": map[string]any{"type": "string", "maxLength": 20000}, "expected_result": map[string]any{"type": "string", "maxLength": 20000}}, "required": []string{"action"}, "additionalProperties": false}},
		"expected_result": map[string]any{"type": "string", "maxLength": 20000},
	}, "required": []string{"key", "title", "steps"}, "additionalProperties": false}
	cases := pmArray("cases", "测试用例", caseItem, true)
	cases.schema["maxItems"] = 200
	definition := append(target, pmInteger("version", "并发版本", false), pmStringMax("purpose", "测试目的", 20000, false), pmStringMax("preconditions", "前置条件", 20000, false), pmStringMax("environment_guidance", "环境指引", 20000, false), cases)
	caseKeys := pmArray("case_keys", "限定用例 key", map[string]any{"type": "string", "maxLength": 100}, false)
	caseKeys.schema["uniqueItems"] = true
	execution := append(target, pmInteger("definition_version", "测试定义版本", true), pmStringMax("environment", "执行环境", 160, true), pmStringMax("build_version", "构建版本", 160, false), pmNullable(pmInteger("executor_user_id", "执行人用户 ID", false)), pmStringMax("summary", "执行说明", 4000, false), caseKeys)
	executedAt := pmString("executed_at", "执行时间 RFC3339", false)
	executedAt.schema["format"] = "date-time"
	runUpdate := append(target, pmInteger("run_id", "执行结果 ID", true), pmInteger("version", "并发版本", true), pmEnum("status", "执行状态", []string{"pending", "running", "passed", "failed", "blocked", "skipped"}, false), pmStringMax("actual_result", "实际结果", 20000, false), pmStringMax("failure_reason", "失败原因", 20000, false), executedAt)
	return []pmOperationSpec{
		pmActivatable(pmSpec("task.testing.definition.get", toolset, "GET", "tasks/{id}/test-definition", "查询测试定义", "L0", target...)),
		pmActivatable(pmSpec("task.testing.definition.update", toolset, "PUT", "tasks/{id}/test-definition", "更新测试定义", "L2", definition...)),
		pmActivatable(pmSpec("task.testing.context.get", toolset, "GET", "tasks/{id}/test-context", "查询测试上下文", "L0", target...)),
		pmActivatable(pmSpec("task.testing.execution.create", toolset, "POST", "tasks/{id}/test-executions", "创建测试执行", "L2", execution...)),
		pmActivatable(pmSpec("task.testing.execution_run.list", toolset, "GET", "tasks/{id}/test-execution-runs", "查询测试执行结果", "L0", append(target, pmInteger("execution_id", "测试执行 ID", true), pmEnum("status", "执行状态", []string{"pending", "running", "passed", "failed", "blocked", "skipped"}, false), pmInteger("limit", "最大返回数", false), pmNonNegativeInteger("offset", "偏移量", false))...)),
		pmActivatable(pmSpec("task.testing.execution_run.update", toolset, "PATCH", "tasks/{id}/test-execution-runs/{run_id}", "更新测试执行结果", "L2", runUpdate...)),
	}
}
