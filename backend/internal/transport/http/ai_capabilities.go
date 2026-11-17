package httpapi

type aiCapability struct {
	Key                  string   `json:"key"`
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	SubjectTypes         []string `json:"subject_types"`
	WorkflowSubjectTypes []string `json:"workflow_subject_types"`
	AllowedFields        []string `json:"allowed_fields"`
	Mode                 string   `json:"mode"`
}

var aiCapabilities = []aiCapability{
	{Key: "version.scope_proposal", Name: "版本范围建议", Description: "生成版本目标、范围、依赖与风险建议", SubjectTypes: []string{"plan_unit"}, Mode: "proposal"},
	{Key: "requirement.draft", Name: "需求草拟", Description: "从上下文生成结构化需求草稿", SubjectTypes: []string{"requirement"}, AllowedFields: []string{"title", "description", "acceptance_criteria"}, Mode: "proposal"},
	{Key: "requirement.triage", Name: "需求分诊", Description: "建议优先级、归属和下一步", SubjectTypes: []string{"requirement"}, AllowedFields: []string{"priority"}, Mode: "proposal"},
	{Key: "requirement.duplicate_check", Name: "重复需求检查", Description: "对需求池进行语义重复检查", SubjectTypes: []string{"requirement"}, Mode: "analysis"},
	{Key: "requirement.acceptance_criteria", Name: "验收标准", Description: "生成可验证的验收标准", SubjectTypes: []string{"requirement"}, AllowedFields: []string{"acceptance_criteria"}, Mode: "proposal"},
	{Key: "requirement.completeness", Name: "完整性检查", Description: "检查背景、边界、异常和验收条件", SubjectTypes: []string{"requirement"}, WorkflowSubjectTypes: []string{"requirement"}, Mode: "analysis"},
	{Key: "requirement.design_classify", Name: "设计介入判断", Description: "判断是否需要 UI/交互设计介入", SubjectTypes: []string{"requirement"}, Mode: "analysis"},
	{Key: "design.requirement_coverage", Name: "设计覆盖检查", Description: "检查设计稿对需求和状态的覆盖", SubjectTypes: []string{"requirement"}, Mode: "analysis"},
	{Key: "requirement.review_pack", Name: "评审材料包", Description: "汇总评审议题、风险和待决项", SubjectTypes: []string{"requirement"}, Mode: "artifact"},
	{Key: "meeting.extract_decisions", Name: "会议决策提取", Description: "提取结论、责任人和截止时间", SubjectTypes: []string{"requirement", "plan_unit"}, Mode: "artifact"},
	{Key: "workflow.generate", Name: "工作流生成", Description: "按业务描述生成受限工作流定义", SubjectTypes: []string{"requirement", "plan_unit", "workflow_template"}, Mode: "proposal"},
	{Key: "workflow.lint_explain", Name: "工作流解释", Description: "解释校验错误和修复方式", SubjectTypes: []string{"requirement", "plan_unit", "workflow_template"}, Mode: "analysis"},
	{Key: "workflow.simulate", Name: "工作流模拟", Description: "模拟典型路径和异常分支", SubjectTypes: []string{"requirement", "plan_unit", "workflow_template"}, Mode: "analysis"},
	{Key: "requirement.task_plan", Name: "任务拆解", Description: "生成任务、依赖和角色草稿", SubjectTypes: []string{"requirement"}, WorkflowSubjectTypes: []string{"requirement"}, Mode: "artifact"},
	{Key: "requirement.acceptance_cases", Name: "验收用例", Description: "生成业务验收用例草稿", SubjectTypes: []string{"requirement"}, Mode: "artifact"},
	{Key: "delivery.schedule_proposal", Name: "排期建议", Description: "基于依赖和工作量生成排期建议", SubjectTypes: []string{"plan_unit"}, Mode: "proposal"},
	{Key: "delivery.risk_monitor", Name: "交付风险", Description: "识别延期、阻塞和范围漂移风险", SubjectTypes: []string{"requirement", "plan_unit"}, WorkflowSubjectTypes: []string{"plan_unit"}, Mode: "analysis"},
	{Key: "delivery.daily_brief", Name: "交付日报", Description: "生成进度、风险和行动项摘要", SubjectTypes: []string{"plan_unit"}, Mode: "artifact"},
	{Key: "delivery.change_impact", Name: "变更影响", Description: "分析需求变更对任务和交付的影响", SubjectTypes: []string{"requirement"}, Mode: "analysis"},
	{Key: "delivery.test_handoff", Name: "提测说明", Description: "生成提测范围、环境和注意事项", SubjectTypes: []string{"requirement", "plan_unit"}, Mode: "artifact"},
	{Key: "delivery.fix_plan", Name: "修复计划", Description: "汇总缺陷并生成修复顺序建议", SubjectTypes: []string{"requirement", "plan_unit"}, Mode: "proposal"},
	{Key: "qa.test_cases", Name: "AI 测试用例", Description: "生成可下载的结构化测试用例", SubjectTypes: []string{"requirement", "plan_unit"}, Mode: "artifact"},
	{Key: "qa.result_summary", Name: "测试总结", Description: "汇总测试结果与残余风险", SubjectTypes: []string{"requirement", "plan_unit"}, Mode: "artifact"},
	{Key: "ui.design_compare", Name: "设计还原对比", Description: "对比实现截图和设计材料", SubjectTypes: []string{"requirement"}, Mode: "analysis"},
	{Key: "verification.aggregate", Name: "验收汇总", Description: "聚合 QA、UI 和产品验收证据", SubjectTypes: []string{"requirement", "plan_unit"}, Mode: "artifact"},
	{Key: "release.readiness", Name: "发布就绪检查", Description: "生成发布门禁检查结果，最终上线仍需人工批准", SubjectTypes: []string{"plan_unit"}, Mode: "analysis"},
	{Key: "release.notes", Name: "发布说明", Description: "生成面向用户和内部的发布说明", SubjectTypes: []string{"plan_unit"}, Mode: "artifact"},
	{Key: "project.retrospective", Name: "项目复盘", Description: "提炼结果、偏差和改进项", SubjectTypes: []string{"plan_unit"}, Mode: "artifact"},
	{Key: "inbox.daily_brief", Name: "个人工作简报", Description: "汇总当前用户的 PM 待办和风险", SubjectTypes: []string{"workspace"}, Mode: "artifact"},
}

func aiCapabilityByKey(key string) (aiCapability, bool) {
	for _, capability := range aiCapabilities {
		if capability.Key == key {
			return capability, true
		}
	}
	return aiCapability{}, false
}

func aiCapabilitiesForResponse() []aiCapability {
	result := make([]aiCapability, len(aiCapabilities))
	copy(result, aiCapabilities)
	for index := range result {
		if result[index].SubjectTypes == nil {
			result[index].SubjectTypes = []string{}
		}
		if result[index].WorkflowSubjectTypes == nil {
			result[index].WorkflowSubjectTypes = []string{}
		}
		if result[index].AllowedFields == nil {
			result[index].AllowedFields = []string{}
		}
	}
	return result
}
