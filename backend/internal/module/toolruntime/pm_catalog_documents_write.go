package toolruntime

func pmDocumentWriteOperationSpecs() []pmOperationSpec {
	const writes = "pm_native_document_write"
	return []pmOperationSpec{
		pmActivatable(pmSpec("document.content.replace", writes, "PATCH", "documents/{id}", "写入或替换文档完整正文；先读取版本，有人在编辑时拒绝执行", "L2", pmInteger("id", "文档 ID（不是节点 ID）", true), pmInteger("expected_revision", "读取文档得到的最新 revision", true), pmInteger("schema_version", "读取文档得到的 schema_version", true), pmDocumentContentField())),
		pmActivatable(pmSpec("document.node.create", writes, "POST", "personal-document-nodes", "创建个人目录节点", "L2", pmNullable(pmInteger("parent_id", "目标父目录，null 表示根目录", false)), pmEnum("type", "节点类型", []string{"folder", "document"}, true), pmStringMax("title", "标题", 255, true), pmBool("collaborative", "是否启用协作正文", false))),
		pmActivatable(pmSpec("document.node.copy", writes, "POST", "personal-document-nodes/{id}/copies", "复制文档节点", "L2", pmInteger("id", "对象 ID", true), pmNullable(pmInteger("parent_id", "目标父目录，null 表示根目录", false)))),
		pmActivatable(pmSpec("document.node.rename", writes, "PATCH", "personal-document-nodes/{id}", "重命名个人目录节点", "L2", pmInteger("id", "对象 ID", true), pmStringMax("title", "标题", 255, true))),
		pmActivatable(pmSpec("document.node.move", writes, "POST", "personal-document-nodes/{id}/move", "移动个人目录节点", "L2", pmInteger("id", "对象 ID", true), pmNullable(pmInteger("parent_id", "目标父目录，null 表示根目录", false)), pmInteger("before_node_id", "排在此节点之前", false), pmInteger("after_node_id", "排在此节点之后", false), pmInteger("expected_directory_revision", "读取时的目录版本", true))),
		pmActivatable(pmSpec("document.node.archive", writes, "DELETE", "personal-document-nodes/{id}", "归档个人目录节点", "L3", pmInteger("id", "对象 ID", true), pmBool("confirm_non_empty", "确认归档非空目录", false), pmNonNegativeInteger("expected_descendant_count", "预览的后代数量", false), pmNonNegativeInteger("expected_document_count", "预览的文档数量", false))),
		pmActivatable(pmSpec("document.node.shortcut.create", writes, "POST", "personal-document-nodes/{id}/shortcuts", "创建文档快捷方式", "L2", pmInteger("id", "对象 ID", true), pmNullable(pmInteger("parent_id", "目标父目录，null 表示根目录", false)))),
		pmActivatable(pmSpec("document.node.preference.update", writes, "PUT", "personal-document-nodes/{id}/preferences", "设置个人置顶和收藏偏好", "L2", pmInteger("id", "对象 ID", true), pmBool("pinned", "是否置顶", false), pmBool("favorited", "是否收藏", false))),
		pmActivatable(pmSpec("document.owner.transfer", writes, "PUT", "documents/{id}/owner", "转移文档所有者", "L3", pmInteger("id", "对象 ID", true), pmInteger("owner_user_id", "新所有者的 PM 用户 ID", true))),
		pmActivatable(pmSpec("document.binding.create", writes, "POST", "documents/{id}/bindings", "绑定需求", "L2", pmInteger("id", "对象 ID", true), pmInteger("requirement_id", "需求 ID", true), pmBool("confirmed", "是否已确认绑定影响", true))),
		pmActivatable(pmSpec("document.binding.delete", writes, "DELETE", "documents/{id}/bindings/{bindingId}", "解除需求绑定", "L2", pmInteger("id", "对象 ID", true), pmInteger("binding_id", "绑定 ID", true))),
		pmActivatable(pmSpec("document.asset.delete", writes, "DELETE", "documents/{id}/assets/{assetId}", "删除文档附件", "L2", pmInteger("id", "对象 ID", true), pmInteger("asset_id", "附件 ID", true))),
		pmActivatable(pmSpec("document.comment_thread.create", writes, "POST", "documents/{id}/comment-threads", "创建文档评论线程", "L2", pmInteger("id", "对象 ID", true), pmStringMax("body", "评论内容", 100000, true), pmDocumentAnchorField())),
		pmActivatable(pmSpec("document.comment.reply", writes, "POST", "documents/{id}/comment-threads/{threadId}/comments", "回复评论线程", "L2", pmInteger("id", "对象 ID", true), pmInteger("thread_id", "线程 ID", true), pmInteger("parent_comment_id", "父评论 ID", false), pmStringMax("body", "评论内容", 100000, true))),
		pmActivatable(pmSpec("document.comment.edit_own", writes, "PATCH", "documents/{id}/comments/{commentId}", "编辑自己的评论", "L2", pmInteger("id", "对象 ID", true), pmInteger("comment_id", "评论 ID", true), pmStringMax("body", "评论内容", 100000, true))),
		pmActivatable(pmSpec("document.comment.delete_own", writes, "DELETE", "documents/{id}/comments/{commentId}", "删除自己的评论", "L2", pmInteger("id", "对象 ID", true), pmInteger("comment_id", "评论 ID", true))),
		pmActivatable(pmSpec("document.comment_thread.resolve", writes, "POST", "documents/{id}/comment-threads/{threadId}/resolve", "解决评论线程", "L2", pmInteger("id", "对象 ID", true), pmInteger("thread_id", "线程 ID", true))),
		pmActivatable(pmSpec("document.comment_thread.reopen", writes, "POST", "documents/{id}/comment-threads/{threadId}/reopen", "重新打开评论线程", "L2", pmInteger("id", "对象 ID", true), pmInteger("thread_id", "线程 ID", true))),
	}
}
func pmDocumentAnchorField() pmCatalogField {
	selection := map[string]any{"type": "object", "properties": map[string]any{
		"document_revision": pmPositiveOutputSchema(), "from": pmCountOutputSchema(), "to": pmCountOutputSchema(),
		"quote": map[string]any{"type": "string"}, "prefix": map[string]any{"type": "string"}, "suffix": map[string]any{"type": "string"},
	}, "required": []string{"document_revision", "from", "to", "quote"}, "additionalProperties": false}
	field := pmObject("anchor", "评论锚点", map[string]any{"kind": map[string]any{"type": "string", "enum": []string{"document", "selection_snapshot_v1"}}, "selection": selection}, true)
	field.schema["required"] = []string{"kind"}
	return field
}

func pmDocumentContentField() pmCatalogField {
	f := pmObject("content", "完整 Tiptap JSON 正文对象；替换全部正文", map[string]any{"type": map[string]any{"type": "string", "enum": []string{"doc"}}, "content": map[string]any{"type": "array", "items": map[string]any{"type": "object"}}}, true)
	f.schema["required"] = []string{"type", "content"}
	return f
}
