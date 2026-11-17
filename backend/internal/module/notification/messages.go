package notification

import "strings"

// PlainText is the single PM-owned copy source for notification scenes shared
// by HTTP and protocol-neutral Tool mutations.
func PlainText(scene string, vars map[string]string) string {
	line := func(label, key string) string {
		value := strings.TrimSpace(vars[key])
		if value == "" {
			value = "-"
		}
		return label + "：" + value
	}
	optional := func(label, key string) string {
		value := strings.TrimSpace(vars[key])
		if value == "" || value == "-" {
			return ""
		}
		return label + "：" + value
	}
	join := func(lines ...string) string {
		result := make([]string, 0, len(lines))
		for _, value := range lines {
			if strings.TrimSpace(value) != "" {
				result = append(result, value)
			}
		}
		return strings.Join(result, "\n")
	}

	switch strings.TrimSpace(scene) {
	case SceneDocumentAccessRequested:
		return join("你有一条资源编辑权限申请，请打开 PM 查看。", line("链接", "url"))
	case SceneDocumentAccessReviewed:
		if vars["status"] == "revoked" {
			return join("你的资源授权已撤销。", line("链接", "url"))
		}
		if vars["status"] == "approved" {
			level := "可编辑"
			if vars["grantLevel"] == "viewer" {
				level = "可查看"
			}
			duration := "永久"
			switch vars["grantDuration"] {
			case "24h":
				duration = "24 小时"
			case "7d":
				duration = "7 天"
			}
			return join("你的资源权限申请已通过。", "授权："+level+"，"+duration, line("链接", "url"))
		}
		return join("你的资源权限申请未通过。", line("链接", "url"))
	case SceneDocumentCommentMentioned:
		return join("有人在文档评论中 @ 了你", line("链接", "url"))
	case SceneQualityBugCreated:
		return join(line("缺陷已提交", "title"), line("编号", "bugKey"),
			line("严重级别", "severity"), line("当前状态", "status"),
			line("操作人", "operator"), line("链接", "url"))
	case SceneQualityBugAssigned:
		return join(line("缺陷待处理", "title"), line("编号", "bugKey"),
			line("严重级别", "severity"), line("当前状态", "status"),
			line("指派给", "assignee"), optional("抄送", "ccUsers"), optional("备注", "note"),
			line("操作人", "operator"), line("链接", "url"))
	case SceneQualityBugReviewRequested:
		return join(line("缺陷待回归", "title"), line("编号", "bugKey"),
			line("解决方案", "resolutionReason"), line("处理人", "assignee"),
			line("操作人", "operator"), line("链接", "url"))
	case SceneQualityBugReturned:
		return join(line("缺陷已重新打开", "title"), line("编号", "bugKey"),
			optional("重新打开说明", "note"), line("负责人", "assignee"),
			line("操作人", "operator"), line("链接", "url"))
	case SceneQualityBugCommented:
		return join(line("缺陷新增评论", "title"), line("编号", "bugKey"),
			line("评论人", "operator"), line("评论", "comment"), line("链接", "url"))
	case SceneWorkTaskStatusChanged:
		return join(line("任务状态已变更", "title"), line("编号", "taskKey"), line("原状态", "fromStatus"), line("当前状态", "toStatus"), line("操作人", "operator"), line("链接", "url"))
	case SceneWorkTaskAssigned:
		return join(line("任务已分配", "title"), line("编号", "taskKey"),
			line("类型", "kind"), line("优先级", "priority"), line("当前状态", "status"),
			line("负责人", "assignee"), line("操作人", "operator"), line("链接", "url"))
	case SceneWorkTaskCommented:
		return join(line("任务新增评论", "title"), line("编号", "taskKey"),
			line("评论人", "operator"), line("评论", "comment"), line("链接", "url"))
	case SceneWorkflowNodeAssigned:
		return join(line("流程节点已指派", "node"), line("需求", "title"),
			line("负责人", "assignee"), line("操作人", "operator"), line("链接", "url"))
	case SceneWorkflowPreReviewInvited:
		return join(line("邀请你参与预审", "node"), line("需求", "title"),
			line("邀请人", "operator"), line("链接", "url"))
	case ScenePlanUnitMemberAssigned:
		return join(line("你已成为版本成员", "projectSet"), line("版本", "unit"),
			line("你的角色", "role"), line("操作人", "operator"), line("链接", "url"))
	case SceneIdentityPasswordResetRequested:
		return join(line("账号需要重置密码", "user"), line("操作人", "operator"),
			line("链接有效期至", "expiresAt"), line("改密链接", "url"))
	default:
		return ""
	}
}
