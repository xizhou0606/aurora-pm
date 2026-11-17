package toolruntime

const (
	PMCurrentUserToolName  = "user.me"
	PMCurrentUserReadScope = "identity:read"
)

func pmAccessOperationSpecs() []pmOperationSpec {
	const access = "pm_native_access_read"
	const users = "pm_native_user_read"
	return []pmOperationSpec{
		pmActivatable(pmSpec(PMCurrentUserToolName, users, "GET", "users/me", "查询当前登录用户的 PM 权威身份", "L0")),
		pmActivatable(pmSpec("workspace.member.list", access, "GET", "workspaces/{id}/members", "查询当前 workspace 的公开成员标识事实", "L0", pmInteger("id", "必须与当前授权 Workspace 一致", true))),
		pmActivatable(pmSpec("team.member.list", access, "GET", "teams/{id}/members", "查询团队成员事实", "L0", pmInteger("id", "团队 ID", true))),
		pmActivatable(pmSpec("project.participant.list", access, "GET", "project-sets/{id}/participants", "查询项目参与者事实", "L0", pmInteger("id", "项目集 ID", true))),
		pmActivatable(pmSpec("project.access.effective", access, "GET", "project-access/effective", "查询当前用户对项目的有效访问事实", "L0", pmInteger("project_set_id", "项目集 ID", true))),
		pmActivatable(pmSpec("user.search", users, "GET", "users", "搜索 PM 权威人员事实；歧义时返回候选", "L0",
			pmString("q", "姓名或用户名关键词", false), pmString("username", "精确用户名", false),
			pmInteger("id", "平台用户 ID", false), pmBool("exact", "是否精确匹配", false),
			pmInteger("limit", "最大返回数", false), pmNonNegativeInteger("offset", "偏移量", false))),
	}
}

func pmWorkbenchOperationSpecs() []pmOperationSpec {
	const toolset = "pm_native_workbench_read"
	scope := []pmCatalogField{pmInteger("project_set_id", "项目集 ID", false), pmInteger("project_new_id", "项目/版本 ID", false), pmInteger("requirement_id", "需求 ID", false)}
	return []pmOperationSpec{
		pmActivatable(pmSpec("workbench.todo.summary", toolset, "GET", "workbench/todos/summary", "查询我的待办汇总", "L0", append(scope,
			pmString("q", "待办关键词", false), pmString("domains", "task,bug,workflow", false), pmBool("include_mentions", "是否统计提及", false))...)),
		pmActivatable(pmSpec("workbench.todo.feed", toolset, "GET", "workbench/todos/feed", "查询我的待办列表", "L0", append(scope,
			pmString("q", "待办关键词", false), pmEnum("type", "待办类型", []string{"task", "bug", "workflow", "mention"}, false),
			pmInteger("limit", "最大返回数", false), pmNonNegativeInteger("offset", "偏移量", false))...)),
		pmActivatable(pmSpec("bi.screen.get", toolset, "GET", "executive-dashboard/bi-screen", "查询 PM BI 看板", "L0", append(scope,
			pmEnum("period", "统计周期", []string{"week", "month"}, false))...)),
	}
}
