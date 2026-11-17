package httpapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/santhosh-tekuri/jsonschema/v6"
	workflow "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
)

type workflowDefinition struct {
	Nodes                  []workflowNode `json:"nodes"`
	Edges                  []workflowEdge `json:"edges"`
	DeliveryProgressConfig map[string]any `json:"delivery_progress,omitempty"`
}

type workflowNode struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Name       string         `json:"name"`
	Capability string         `json:"capability,omitempty"`
	Config     map[string]any `json:"config,omitempty"`
}

type workflowEdge struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Target    string `json:"target"`
	Kind      string `json:"kind,omitempty"`
	Condition string `json:"condition,omitempty"`
	Loop      bool   `json:"loop,omitempty"`
}

var workflowNodeTypes = map[string]bool{
	"start": true, "end": true, "human_task": true, "form": true, "approval": true,
	"condition": true, "parallel_split": true, "parallel_join": true, "timer": true,
	"ai_extract": true, "ai_check": true, "ai_classify": true, "ai_generate": true,
	"ai_plan": true, "ai_summarize": true, "ai_compare": true, "outbox_action": true,
	"create_drafts": true, "requirement_gate": true,
}

var workflowSchema = map[string]any{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type":    "object", "required": []any{"nodes", "edges"}, "additionalProperties": false,
	"properties": map[string]any{
		"nodes": map[string]any{"type": "array", "minItems": 2, "items": map[string]any{
			"type": "object", "required": []any{"id", "type", "name"},
			"properties": map[string]any{
				"id":   map[string]any{"type": "string", "minLength": 1, "maxLength": 100},
				"type": map[string]any{"type": "string"}, "name": map[string]any{"type": "string", "minLength": 1},
				"capability": map[string]any{"type": "string"}, "config": map[string]any{"type": "object"},
			}, "additionalProperties": false,
		}},
		"edges": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{
			"type": "object", "required": []any{"id", "source", "target"},
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "minLength": 1}, "source": map[string]any{"type": "string"},
				"target": map[string]any{"type": "string"}, "kind": map[string]any{"type": "string"},
				"condition": map[string]any{"type": "string"}, "loop": map[string]any{"type": "boolean"},
			}, "additionalProperties": false,
		}},
		"delivery_progress": map[string]any{"type": "object"},
	},
}

func compileWorkflowDefinition(raw string) (workflowDefinition, []string) {
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return workflowDefinition{}, []string{"定义不是有效 JSON：" + err.Error()}
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", workflowSchema); err != nil {
		return workflowDefinition{}, []string{"JSON Schema 初始化失败：" + err.Error()}
	}
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return workflowDefinition{}, []string{"JSON Schema 编译失败：" + err.Error()}
	}
	if err := schema.Validate(value); err != nil {
		return workflowDefinition{}, []string{"定义结构不合法：" + err.Error()}
	}
	var definition workflowDefinition
	if err := json.Unmarshal([]byte(raw), &definition); err != nil {
		return workflowDefinition{}, []string{err.Error()}
	}
	stripWorkflowTemplateAssignees(&definition)
	stripRetiredWorkflowNodeFlags(&definition)
	return definition, validateWorkflowGraph(definition)
}

func stripRetiredWorkflowNodeFlags(definition *workflowDefinition) {
	if definition == nil {
		return
	}
	for index := range definition.Nodes {
		if definition.Nodes[index].Config == nil {
			continue
		}
		delete(definition.Nodes[index].Config, "test_report_artifact")
	}
}

func validateWorkflowGraph(def workflowDefinition) []string {
	errorsFound := make([]string, 0)
	nodes := map[string]workflowNode{}
	edgeIDs := map[string]bool{}
	incoming, outgoing := map[string][]workflowEdge{}, map[string][]workflowEdge{}
	starts, ends := 0, 0
	for _, node := range def.Nodes {
		if nodes[node.ID].ID != "" {
			errorsFound = append(errorsFound, "节点 ID 重复："+node.ID)
		}
		nodes[node.ID] = node
		if !workflowNodeTypes[node.Type] {
			errorsFound = append(errorsFound, "不支持的节点类型："+node.Type)
		}
		if node.Type == "start" {
			starts++
		}
		if node.Type == "end" {
			ends++
		}
		if strings.HasPrefix(node.Type, "ai_") && strings.TrimSpace(node.Capability) == "" {
			errorsFound = append(errorsFound, "AI 节点必须声明 capability："+node.ID)
		}
		if node.Type == "timer" {
			if delay, exists := node.Config["delay_seconds"]; exists {
				seconds, ok := delay.(float64)
				if !ok || seconds < 0 {
					errorsFound = append(errorsFound, "计时器 delay_seconds 必须是非负数："+node.ID)
				}
			}
		}
		if critical, _ := node.Config["critical_gate"].(bool); critical && node.Type != "approval" {
			errorsFound = append(errorsFound, "关键门禁必须由人工审批节点执行："+node.ID)
		}
		if required, _ := node.Config["require_design_image"].(bool); required && node.Type != "human_task" {
			errorsFound = append(errorsFound, "设计效果图必须配置在人工任务节点："+node.ID)
		}
		if required, _ := node.Config["require_design_image_on_yes"].(bool); required {
			errorsFound = append(errorsFound, "条件节点只负责分支选择，请在后续人工任务节点配置设计效果图："+node.ID)
		}
		if value, exists := node.Config["test_case_artifact"]; exists {
			required, valid := value.(bool)
			if !valid || required && node.Type != "human_task" && node.Type != "form" {
				errorsFound = append(errorsFound, "测试用例交付物必须配置在人工任务或表单节点："+node.ID)
			}
		}
		if value, exists := node.Config["requirement_intake_lock"]; exists {
			required, valid := value.(bool)
			if !valid {
				errorsFound = append(errorsFound, "需求录入锁版确认配置必须是布尔值："+node.ID)
			} else if required && node.Type != "human_task" && node.Type != "form" {
				errorsFound = append(errorsFound, "需求录入锁版确认必须配置在人工任务或表单节点："+node.ID)
			}
		}
		if _, exists := node.Config["requirements_guard"]; exists {
			guard, _, err := workflow.ParseRequirementsGuard(node.Config)
			if err != nil {
				errorsFound = append(errorsFound, "节点 "+node.ID+" 的关联需求流程配置无效："+err.Error())
			} else {
				for _, issue := range guard.ValidateStructural() {
					errorsFound = append(errorsFound, "节点 "+node.ID+" 关联需求流程："+issue)
				}
			}
		}
		if node.Type == "requirement_gate" {
			guard, present, err := workflow.ParseRequirementsGuard(node.Config)
			if err != nil || !present {
				errorsFound = append(errorsFound, "需求等待节点必须配置关联需求流程："+node.ID)
			} else if guard.OnSatisfied != workflow.GuardOnSatisfiedAutoComplete {
				errorsFound = append(errorsFound, "需求等待节点的 on_satisfied 必须为 auto_complete："+node.ID)
			}
		}
	}
	if starts != 1 {
		errorsFound = append(errorsFound, "必须且只能有一个开始节点")
	}
	if ends != 1 {
		errorsFound = append(errorsFound, "必须且只能有一个结束节点")
	}
	celEnv, _ := cel.NewEnv(cel.Variable("subject", cel.DynType), cel.Variable("context", cel.DynType))
	for _, edge := range def.Edges {
		if edgeIDs[edge.ID] {
			errorsFound = append(errorsFound, "连线 ID 重复："+edge.ID)
		}
		edgeIDs[edge.ID] = true
		if nodes[edge.Source].ID == "" || nodes[edge.Target].ID == "" {
			errorsFound = append(errorsFound, fmt.Sprintf("连线 %s 引用了不存在的节点", edge.ID))
			continue
		}
		outgoing[edge.Source] = append(outgoing[edge.Source], edge)
		incoming[edge.Target] = append(incoming[edge.Target], edge)
		if edge.Condition != "" {
			_, issues := celEnv.Compile(edge.Condition)
			if issues != nil && issues.Err() != nil {
				errorsFound = append(errorsFound, "CEL 条件无效 "+edge.ID+"："+issues.Err().Error())
			}
		}
	}
	for _, node := range def.Nodes {
		if node.Type == "start" && len(incoming[node.ID]) > 0 {
			errorsFound = append(errorsFound, "开始节点不能有入边："+node.ID)
		}
		if node.Type != "start" && len(incoming[node.ID]) == 0 {
			errorsFound = append(errorsFound, "非开始节点必须有入边："+node.ID)
		}
		if node.Type == "end" && len(outgoing[node.ID]) > 0 {
			errorsFound = append(errorsFound, "结束节点不能有出边："+node.ID)
		}
		if node.Type != "end" && len(outgoing[node.ID]) == 0 {
			errorsFound = append(errorsFound, "非结束节点必须有出边："+node.ID)
		}
		if node.Type != "condition" && node.Type != "parallel_split" {
			forward := 0
			for _, edge := range outgoing[node.ID] {
				if edge.Kind != "failure" && edge.Kind != "reject" {
					forward++
				}
			}
			if forward > 1 {
				errorsFound = append(errorsFound, "普通节点不能用多条默认路径伪装并行，请使用并行分支节点："+node.ID)
			}
		}
		if node.Type == "parallel_split" && len(outgoing[node.ID]) < 2 {
			errorsFound = append(errorsFound, "并行分支至少需要两条出边："+node.ID)
		}
		if node.Type == "parallel_join" && len(incoming[node.ID]) < 2 {
			errorsFound = append(errorsFound, "并行汇聚至少需要两条入边："+node.ID)
		}
		if strings.HasPrefix(node.Type, "ai_") {
			hasFailure := false
			for _, edge := range outgoing[node.ID] {
				hasFailure = hasFailure || edge.Kind == "failure"
			}
			if !hasFailure {
				errorsFound = append(errorsFound, "AI 节点必须配置失败分支："+node.ID)
			}
		}
	}
	pairedSplits := map[string]bool{}
	for _, node := range def.Nodes {
		if node.Type != "parallel_join" {
			continue
		}
		split, _, paired := workflowMatchingParallelSplit(def, node.ID)
		if !paired {
			errorsFound = append(errorsFound, "并行汇聚没有匹配的并行分支："+node.ID)
			continue
		}
		pairedSplits[split.ID] = true
	}
	for _, node := range def.Nodes {
		if node.Type == "parallel_split" && !pairedSplits[node.ID] {
			errorsFound = append(errorsFound, "并行分支的所有支路必须汇聚到同一个并行汇聚节点："+node.ID)
		}
	}
	startID := ""
	for _, node := range def.Nodes {
		if node.Type == "start" {
			startID = node.ID
		}
	}
	visited := map[string]bool{}
	var visit func(string, map[string]bool)
	visit = func(id string, stack map[string]bool) {
		if stack[id] {
			return
		}
		visited[id] = true
		nextStack := make(map[string]bool, len(stack)+1)
		for key, value := range stack {
			nextStack[key] = value
		}
		nextStack[id] = true
		for _, edge := range outgoing[id] {
			if nextStack[edge.Target] && !edge.Loop {
				errorsFound = append(errorsFound, "循环连线必须显式标记 loop："+edge.ID)
			}
			if !nextStack[edge.Target] {
				visit(edge.Target, nextStack)
			}
		}
	}
	if startID != "" {
		visit(startID, map[string]bool{})
	}
	for _, node := range def.Nodes {
		if !visited[node.ID] {
			errorsFound = append(errorsFound, "存在不可达节点："+node.ID)
		}
	}
	endID := ""
	for _, node := range def.Nodes {
		if node.Type == "end" {
			endID = node.ID
		}
	}
	if endID != "" {
		for _, node := range def.Nodes {
			if reachable, _ := workflowReachable(def, node.ID, endID); !reachable {
				errorsFound = append(errorsFound, "节点无法到达结束节点："+node.ID)
			}
		}
	}
	return uniqueStrings(errorsFound)
}

func validateWorkflowCapabilities(def workflowDefinition, subjectType string) []string {
	issues := make([]string, 0)
	for _, node := range def.Nodes {
		if value, exists := node.Config["requirement_stage"]; exists {
			stage, _ := value.(string)
			_, validStage := workflow.StageIndexMap()[stage]
			if subjectType != "plan_unit" {
				issues = append(issues, "需求阶段联动只能配置在项目/版本模板："+node.ID)
			} else if stage != "none" && !validStage {
				issues = append(issues, "节点 "+node.ID+" 的需求阶段无效（可用：none/backlog/grooming/development/acceptance/done）")
			} else if stage != "none" && node.Type != "human_task" && node.Type != "form" && node.Type != "approval" {
				issues = append(issues, "需求阶段联动只能配置在人工任务、表单或审批节点："+node.ID)
			}
		}
		if _, exists := node.Config["requirement_intake_lock"]; exists && subjectType != "plan_unit" {
			issues = append(issues, "需求录入锁版确认只能配置在项目/版本模板："+node.ID)
		}
		if node.Type == "requirement_gate" && subjectType != "plan_unit" {
			issues = append(issues, "需求等待节点只能配置在项目/版本模板："+node.ID)
		}
		if guard, present, err := workflow.ParseRequirementsGuard(node.Config); present {
			if err != nil {
				issues = append(issues, "节点 "+node.ID+" 的关联需求流程配置无效："+err.Error())
				continue
			}
			guardNodeTypes := node.Type == "human_task" || node.Type == "approval" || node.Type == "requirement_gate"
			if !guardNodeTypes {
				issues = append(issues, "关联需求流程只能配置在人工任务、审批或需求等待节点："+node.ID)
			}
			if guard.OnSatisfied == workflow.GuardOnSatisfiedAutoComplete && node.Type != "requirement_gate" {
				issues = append(issues, "auto_complete 只能配置在需求等待节点："+node.ID)
			}
			if guard.OnSatisfied == workflow.GuardOnSatisfiedAction && node.Type == "requirement_gate" {
				issues = append(issues, "需求等待节点必须使用 auto_complete："+node.ID)
			}
		}
		if !strings.HasPrefix(node.Type, "ai_") {
			continue
		}
		capability, found := aiCapabilityByKey(node.Capability)
		if !found {
			issues = append(issues, "AI capability 不存在："+node.Capability)
			continue
		}
		if !containsString(capability.SubjectTypes, subjectType) {
			issues = append(issues, "AI capability 不支持模板对象："+node.Capability)
		}
		if !containsString(capability.WorkflowSubjectTypes, subjectType) {
			issues = append(issues, "AI capability 未开放为流程节点："+node.Capability)
			continue
		}
		checkNode := node.Type == "ai_check" || node.Type == "ai_classify" || node.Type == "ai_compare"
		if checkNode != (capability.Mode == "analysis") {
			issues = append(issues, "AI capability 与节点类型不匹配："+node.Capability)
		}
	}
	if def.DeliveryProgressConfig != nil {
		if subjectType != "plan_unit" {
			issues = append(issues, "需求交付进度只能配置在项目/版本模板")
		} else if config, present, err := workflow.ParseDeliveryProgress(def.DeliveryProgressConfig); err != nil {
			issues = append(issues, "需求交付进度配置无效："+err.Error())
		} else if present {
			issues = append(issues, config.ValidateStructural()...)
		}
	}
	return uniqueStrings(issues)
}

// validateWorkflowActionEdges is an authoring-time guard. It intentionally is
// not part of compileWorkflowDefinition so immutable historical versions keep
// their runtime compatibility, while newly saved/published templates cannot
// use edge kinds that their source node can never execute.
func validateWorkflowActionEdges(def workflowDefinition) []string {
	nodes := make(map[string]workflowNode, len(def.Nodes))
	outgoing := make(map[string][]workflowEdge, len(def.Nodes))
	for _, node := range def.Nodes {
		nodes[node.ID] = node
	}
	issues := make([]string, 0)
	for _, edge := range def.Edges {
		node, found := nodes[edge.Source]
		if !found {
			continue
		}
		outgoing[node.ID] = append(outgoing[node.ID], edge)
		if !workflowNodeAllowsEdgeKind(node.Type, strings.TrimSpace(edge.Kind)) {
			issues = append(issues, fmt.Sprintf("连线 %s 的 kind=%q 不能由节点 %s 执行", edge.ID, edge.Kind, node.ID))
		}
	}
	for _, node := range def.Nodes {
		if node.Type != "condition" {
			continue
		}
		hasYes, hasNo := false, false
		for _, edge := range outgoing[node.ID] {
			kind := strings.TrimSpace(edge.Kind)
			hasYes = hasYes || kind == "" || kind == "success" || kind == "yes" || kind == "condition_yes"
			hasNo = hasNo || kind == "" || kind == "no" || kind == "condition_no"
		}
		if !hasYes || !hasNo {
			issues = append(issues, "条件节点必须同时配置是与否的可执行路径："+node.ID)
		}
	}
	return uniqueStrings(issues)
}

func workflowNodeAllowsEdgeKind(nodeType, kind string) bool {
	switch {
	case nodeType == "approval":
		return kind == "" || kind == "success" || kind == "approve" || kind == "reject"
	case nodeType == "condition":
		return kind == "" || kind == "success" || kind == "yes" || kind == "no" || kind == "condition_yes" || kind == "condition_no"
	case strings.HasPrefix(nodeType, "ai_"):
		return kind == "" || kind == "success" || kind == "failure"
	case nodeType == "start":
		return kind == "" || kind == "success" || kind == "advance"
	case nodeType == "parallel_split" || nodeType == "parallel_join":
		return kind == "" || kind == "success" || kind == "complete"
	default:
		return kind == "" || kind == "success" || kind == "advance" || kind == "complete"
	}
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
