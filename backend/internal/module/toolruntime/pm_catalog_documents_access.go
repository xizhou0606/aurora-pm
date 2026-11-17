package toolruntime

func pmDocumentAccessOperationSpecs() []pmOperationSpec {
	const reads = "pm_native_document_read"
	return []pmOperationSpec{
		pmActivatable(pmSpec("document.resource.policy.get", reads, "GET", "document-resources/{resourceId}/access-policy", "查看资源访问策略", "L0", pmStringMax("resource_id", "资源 ID", 128, true))),
		pmActivatable(pmSpec("document.access_request.list", reads, "GET", "document-resources/{resourceId}/access-requests", "查看资源访问申请上下文和可见申请", "L0", pmStringMax("resource_id", "资源 ID", 128, true), pmEnum("resource_type", "校验资源类型", []string{"rich_text", "bitable"}, false))),
		pmActivatable(pmSpec("document.access_request.context", reads, "GET", "documents/{id}/access-request-context", "查看文档访问申请上下文", "L0", pmInteger("id", "文档 ID", true))),
		pmActivatable(pmSpec("document.bitable.access_request.context", reads, "GET", "bitables/{resourceRef}/access-request-context", "查看多维表格访问申请上下文", "L0", pmStringMax("resource_ref", "多维表格引用", 128, true))),
		pmActivatable(pmSpec("document.share_subject.search", reads, "GET", "document-resource-subjects", "搜索可用于资源授权的主体", "L0", pmEnum("kind", "主体类型", []string{"user", "role", "user_group", "organization_unit", "project_set"}, true), pmStringMax("q", "搜索词", 255, false), pmInteger("limit", "每页数量", false), pmNonNegativeInteger("offset", "偏移量，须为每页数量的倍数", false))),
		pmActivatable(pmSpec("document.share_link.list", reads, "GET", "document-resources/{resourceId}/share-links", "查询分享链接元数据，不返回秘密", "L0", pmStringMax("resource_id", "资源 ID", 128, true))),
	}
}
