package toolruntime

func pmDocumentOperationSpecs() []pmOperationSpec {
	const reads = "pm_native_document_read"
	return []pmOperationSpec{
		pmActivatable(pmSpec("document.space.get", reads, "GET", "personal-document-spaces/default", "读取当前用户的个人文档空间", "L0")),
		pmActivatable(pmSpec("document.node.tree", reads, "GET", "personal-document-nodes/tree", "读取当前用户的个人文档目录及版本", "L0")),
		pmActivatable(pmSpec("document.get", reads, "GET", "documents/{id}", "读取文档权威正文及版本", "L0", pmInteger("id", "文档 ID", true))),
		pmActivatable(pmSpec("document.export", reads, "GET", "documents/{id}/export", "导出文档 JSON 快照", "L0", pmInteger("id", "文档 ID", true))),
		pmActivatable(pmSpec("document.binding.list", reads, "GET", "documents/{id}/bindings", "查询文档需求绑定", "L0", pmInteger("id", "文档 ID", true))),
		pmActivatable(pmSpec("document.node.deletion_impact", reads, "GET", "personal-document-nodes/{id}/deletion-impact", "预览个人目录节点归档影响", "L0", pmInteger("id", "目录节点 ID", true))),
		pmActivatable(pmSpec("document.binding.preview", reads, "POST", "documents/{id}/bindings/preview", "预览需求绑定影响，不执行绑定", "L0", pmInteger("id", "文档 ID", true), pmInteger("requirement_id", "需求 ID", true))),
		pmActivatable(pmSpec("document.asset.list", reads, "GET", "documents/{id}/assets", "查询文档附件元数据", "L0", pmInteger("id", "文档 ID", true))),
		pmActivatable(pmSpec("document.comment_thread.list", reads, "GET", "documents/{id}/comment-threads", "查询文档评论线程", "L0", pmInteger("id", "文档 ID", true), pmEnum("status", "线程状态", []string{"open", "resolved", "all"}, false), pmInteger("limit", "返回数量", false), pmNonNegativeInteger("offset", "偏移量", false))),
		pmActivatable(pmSpec("document.access_scope.get", reads, "GET", "documents/access-scope", "检查当前用户的文档全范围管理权限", "L0")),
	}
}
