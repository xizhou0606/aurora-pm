package toolruntime

func pmDocumentAccessWriteOperationSpecs() []pmOperationSpec {
	const writes = "pm_native_document_write"
	resource := pmStringMax("resource_id", "资源 ID", 64, true)
	return []pmOperationSpec{
		pmActivatable(pmSpec("document.resource.policy.update", writes, "PUT", "document-resources/{resourceId}/access-policy", "替换资源访问策略，确认完整授权范围", "L3", resource, pmEnum("mode", "策略模式", []string{"private", "restricted", "workspace", "link"}, true), pmEnum("workspace_level", "工作区成员权限，仅 workspace 模式使用", []string{"viewer", "editor"}, false), pmInteger("expected_policy_revision", "读取时的权限策略版本", true), pmDocumentPolicyRules())),
		pmActivatable(pmSpec("document.access_request.create", writes, "POST", "document-resources/{resourceId}/access-requests", "申请资源编辑权限", "L2", resource, pmStringMax("reason", "申请理由", 1000, true), pmBool("notify_owner", "通知所有者，默认 true", false))),
		pmActivatable(pmSpec("document.access_request.decide", writes, "PATCH", "document-resources/{resourceId}/access-requests/{requestId}", "审批或撤销访问申请授权，也可标记已读", "L3", resource, pmInteger("request_id", "申请 ID", true), pmEnum("decision", "审批决定", []string{"approved", "rejected", "revoked"}, false), pmEnum("grant_level", "批准的权限，默认 editor", []string{"viewer", "editor"}, false), pmEnum("grant_duration", "批准的有效期，默认 never", []string{"24h", "7d", "never"}, false), pmBool("read", "是否已读", false))),
		pmActivatable(pmSpec("document.share_link.revoke", writes, "DELETE", "document-resources/{resourceId}/share-links/{linkId}", "撤销分享链接及对应访问上下文", "L3", resource, pmStringMax("link_id", "分享链接 ID", 40, true))),
	}
}

func pmDocumentPolicyRules() pmCatalogField {
	field := pmObject("rules", "完整的直接授权规则；其他模式传空数组", nil, true)
	field.schema = map[string]any{"type": "array", "maxItems": 1000, "items": map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"subject_kind", "subject_id", "level"},
		"properties": map[string]any{
			"subject_kind":        map[string]any{"type": "string", "enum": []string{"user", "role", "user_group", "organization_unit", "project_set"}},
			"subject_id":          map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"level":               map[string]any{"type": "string", "enum": []string{"viewer", "editor"}},
			"include_descendants": map[string]any{"type": "boolean"},
		},
	}}
	return field
}
