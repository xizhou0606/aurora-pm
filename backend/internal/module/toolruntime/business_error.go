package toolruntime

import (
	"encoding/json"
	"errors"
)

// BusinessError is a safe, protocol-neutral failure. Only registered public
// descriptions are exposed; internal errors and user content never become copy.
type BusinessError struct {
	Code      string `json:"code"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	Hint      string `json:"hint"`
	Retryable bool   `json:"retryable"`
	Status    int    `json:"status"`
}

func (err *BusinessError) Error() string { return err.Code }

func LookupBusinessError(code string) *BusinessError {
	value, ok := businessErrors[code]
	if !ok {
		return nil
	}
	value.Code = code
	return &value
}
func PublicBusinessError(err error) *BusinessError {
	var failure *BusinessError
	if errors.As(err, &failure) && failure != nil {
		return LookupBusinessError(failure.Code)
	}
	return nil
}

var businessErrors = map[string]BusinessError{
	"TASK_BATCH_INPUT_INVALID":        {Type: "validation", Status: 400, Message: "批量任务操作参数无效", Hint: "请根据 Tool schema 检查任务选择范围和操作参数"},
	"TASK_BATCH_NOT_FOUND":            {Type: "not_found", Status: 404, Message: "所选任务不存在或不在当前工作区及指定范围内", Hint: "请重新读取任务列表，核对工作区和任务范围后准备操作"},
	"TASK_REFERENCE_AMBIGUOUS":        {Type: "conflict", Status: 409, Message: "任务引用匹配到多个任务", Hint: "请使用任务 ID 或任务编号，并指定项目集、版本或需求范围"},
	"TASK_BATCH_CONFLICT":             {Type: "conflict", Status: 409, Message: "所选任务状态或关联范围已变化，批量操作未完成", Hint: "请重新读取任务及删除影响；删除父任务时须包含全部子任务，或明确指定 cascade=true 后重新准备操作"},
	"DOCUMENT_REPLACE_RESULT_UNKNOWN": {Type: "unavailable", Status: 503, Message: "未能确认文档替换的持久化结果", Hint: "请先回读正文和版本，查询原操作状态；不要直接重复创建替换操作", Retryable: true},
	"QUALITY_INPUT_INVALID":           {Type: "validation", Status: 400, Message: "质量反馈参数不符合要求", Hint: "请检查必填字段和附件 URL；附件链接必须使用 HTTP 或 HTTPS"},
	"QUALITY_NOT_FOUND":               {Type: "authorization", Status: 404, Message: "质量反馈或关联需求不存在或不可访问", Hint: "请核对工作区、需求及项目访问权限"},
	"QUALITY_CONFLICT":                {Type: "conflict", Status: 409, Message: "质量反馈状态或执行记录已变化", Hint: "请重新读取质量反馈后准备操作"},
	"QUALITY_MATERIAL_SCOPE_DENIED":   {Type: "authorization", Status: 404, Message: "需求不存在或无权上传该需求的材料", Hint: "请检查当前工作区、需求和项目访问权限"},
	"TOOL_FILE_REQUIRED":              {Type: "validation", Status: 400, Message: "确认此操作时需要提供文件", Hint: "使用 changeset confirm <id> --file <路径> 提交与准备时摘要一致的文件"},
	"TOOL_FILE_INVALID":               {Type: "validation", Status: 400, Message: "文件大小或摘要与待确认操作不一致", Hint: "请检查文件，重新准备上传操作"},
	"QUALITY_UPLOAD_UNAVAILABLE":      {Type: "unavailable", Status: 503, Message: "质量文件存储暂不可用", Hint: "请检查对象存储配置和上传结果后重试"},

	"DOCUMENT_ACCESS_CONFLICT":           {Type: "conflict", Status: 409, Message: "资源权限或申请状态已变化，无法执行当前操作", Hint: "请重新读取权限策略或申请状态，再准备操作"},
	"DOCUMENT_IDEMPOTENCY_CONFLICT":      {Type: "conflict", Status: 409, Message: "执行标识已用于其他操作内容", Hint: "请使用新的操作标识重新准备，勿复用旧标识"},
	"TOOL_AUTHORIZATION_DENIED":          {Type: "authorization", Status: 403, Message: "当前身份或授权不允许执行此操作", Hint: "请检查用户业务权限、Tool Grant 和工作区允许范围"},
	"TOOL_UAT_BLOCKED":                   {Type: "authorization", Status: 403, Message: "当前不在工具授权的有效时间内", Hint: "请在授权有效时间内重新提交"},
	"TOOL_VERSION_UNAVAILABLE":           {Type: "conflict", Status: 409, Message: "操作所用的 Tool 版本已不可用", Hint: "请重新查询 Tool 定义并准备操作"},
	"TOOL_UNAVAILABLE":                   {Type: "unavailable", Status: 503, Message: "工具执行服务暂不可用", Hint: "请稍后查询操作状态", Retryable: true},
	"TOOL_EXECUTION_FAILED":              {Type: "internal", Status: 500, Message: "操作执行失败，具体原因暂不可用", Hint: "请查询操作记录，确认业务状态后再决定是否重新提交"},
	"TOOL_OUTPUT_INVALID":                {Type: "internal", Status: 500, Message: "操作返回结果不符合契约，不能据此确认执行结果", Hint: "请核对业务对象和操作记录，不要直接重复提交"},
	"CHANGE_SET_EXPIRED":                 {Type: "conflict", Status: 409, Message: "待确认操作已过期", Hint: "请重新读取业务状态并准备操作"},
	"DOCUMENT_EDITING_IN_PROGRESS":       {Type: "conflict", Status: 409, Message: "文档正在编辑，本次未执行替换", Hint: "请待编辑者关闭该文档后，重新读取并提交"},
	"DOCUMENT_VERSION_CONFLICT":          {Type: "conflict", Status: 409, Message: "文档已更新，本次未执行替换", Hint: "请重新读取文档及版本后提交"},
	"DOCUMENT_REPLACE_IN_PROGRESS":       {Type: "conflict", Status: 409, Message: "文档正在执行全文替换", Hint: "请稍后重新读取文档再提交"},
	"DOCUMENT_EDITING_STATE_UNAVAILABLE": {Type: "unavailable", Status: 503, Message: "暂时无法确认编辑状态，本次未执行替换", Hint: "请在协作服务恢复后重新读取并提交", Retryable: true},
	"DOCUMENT_FORBIDDEN":                 {Type: "authorization", Status: 403, Message: "没有执行此文档操作的权限", Hint: "请检查文档访问权限及对应业务操作权限"},
	"DOCUMENT_NOT_FOUND":                 {Type: "not_found", Status: 404, Message: "文档或关联对象不存在或不可访问", Hint: "请核对当前工作区及对象标识"},
	"DOCUMENT_INPUT_INVALID":             {Type: "validation", Status: 422, Message: "文档操作参数无效", Hint: "请根据 Tool schema 检查参数和引用"},
	"DOCUMENT_DIRECTORY_CONFLICT":        {Type: "conflict", Status: 409, Message: "个人文档目录已变化", Hint: "请重新读取目录及版本后提交"},
	"DOCUMENT_ARCHIVE_IMPACT_CHANGED":    {Type: "conflict", Status: 409, Message: "文档归档影响范围已变化或尚未确认", Hint: "请重新查询归档影响并确认后提交"},
	"DOCUMENT_BINDING_CONFLICT":          {Type: "conflict", Status: 409, Message: "需求绑定状态冲突或尚未确认", Hint: "请重新预览绑定影响并确认后提交"},
	"DOCUMENT_COMMENT_CONFLICT":          {Type: "conflict", Status: 409, Message: "评论线程状态已变化", Hint: "请重新读取评论线程后提交"},
	"DOCUMENT_SERVICE_UNAVAILABLE":       {Type: "unavailable", Status: 503, Message: "文档所需服务暂不可用", Hint: "请稍后查询操作状态再决定是否重新提交", Retryable: true},
	"DOCUMENT_COLLABORATION_CUTOVER":     {Type: "conflict", Status: 409, Message: "该文档已启用协作，不能通过旧正文接口写入", Hint: "请使用协作正文替换能力"},
}

func (changeSet ChangeSet) Failed() bool {
	return changeSet.Status == ChangeSetFailed || changeSet.Status == ChangeSetRejected || changeSet.Status == ChangeSetExpired
}

// Reconstruct safe details from the persisted stable code, including after a
// restart. This requires no schema migration and does not store sensitive errors.
func (changeSet ChangeSet) MarshalJSON() ([]byte, error) {
	type wire ChangeSet
	return json.Marshal(struct {
		wire
		Error *BusinessError `json:"error,omitempty"`
	}{wire: wire(changeSet), Error: LookupBusinessError(changeSet.ErrorCode)})
}
