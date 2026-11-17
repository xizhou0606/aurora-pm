package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/cel-go/cel"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflow "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func (s *Server) workflowInstanceByID(c *gin.Context) (workflowgorm.WorkflowInstance, bool) {
	id, okID := paramID(c, "id")
	if !okID {
		return workflowgorm.WorkflowInstance{}, false
	}
	workspace, _ := currentWorkspace(c)
	var row workflowgorm.WorkflowInstance
	if s.db.Where("id = ? AND workspace_id = ?", id, workspace.ID).First(&row).Error != nil {
		fail(c, http.StatusNotFound, "workflow instance not found")
		return workflowgorm.WorkflowInstance{}, false
	}
	if !s.workflowSubjectExists(row.WorkspaceID, row.SubjectType, row.SubjectID) {
		fail(c, http.StatusConflict, "workflow subject no longer exists")
		return workflowgorm.WorkflowInstance{}, false
	}
	projectID := s.workflowSubjectProjectID(s.db, workspace.ID, row.SubjectType, row.SubjectID)
	if projectID == 0 {
		fail(c, http.StatusNotFound, "workflow instance subject project not found")
		return workflowgorm.WorkflowInstance{}, false
	}
	if !s.requireProjectAccess(c, workspace.ID, projectID) {
		return workflowgorm.WorkflowInstance{}, false
	}
	return row, true
}

func (s *Server) workflowSubjectHasHistory(workspaceID uint, subjectType string, subjectID uint) (bool, error) {
	return workflowSubjectHasHistoryTx(s.db, workspaceID, subjectType, subjectID)
}

func workflowSubjectHasHistoryTx(tx *gorm.DB, workspaceID uint, subjectType string, subjectID uint) (bool, error) {
	var count int64
	err := tx.Model(&workflowgorm.WorkflowInstance{}).
		Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspaceID, subjectType, subjectID).
		Count(&count).Error
	return count > 0, err
}

func (s *Server) workflowSubjectExists(workspaceID uint, subjectType string, subjectID uint) bool {
	var count int64
	query := s.db.Where("id = ? AND workspace_id = ?", subjectID, workspaceID)
	if subjectType == "requirement" {
		query.Model(&requirementgorm.Requirement{}).Count(&count)
	} else {
		query.Model(&projectgorm.PlanUnit{}).Count(&count)
	}
	return count == 1
}

func (s *Server) workflowSubjectProjectID(tx *gorm.DB, workspaceID uint, subjectType string, subjectID uint) uint {
	if subjectType == "requirement" {
		var row requirementgorm.Requirement
		if tx.Select("project_new_id").Where("id = ? AND workspace_id = ?", subjectID, workspaceID).First(&row).Error == nil {
			return row.ProjectSetID
		}
		return 0
	}
	var row projectgorm.PlanUnit
	if tx.Select("project_new_id").Where("id = ? AND workspace_id = ?", subjectID, workspaceID).First(&row).Error == nil {
		return row.ProjectSetID
	}
	return 0
}

func workflowInstanceDTO(row workflowgorm.WorkflowInstance, runs []workflowgorm.WorkflowNodeRun, approvals []workflowgorm.WorkflowApproval) gin.H {
	return gin.H{"id": row.ID, "workspace_id": row.WorkspaceID, "template_id": row.TemplateID, "template_version_id": row.TemplateVersionID, "subject_type": row.SubjectType, "subject_id": row.SubjectID, "status": row.Status, "current_node_ids": currentWorkflowNodes(row.CurrentNodeIDs), "started_by_user_id": row.StartedByUserID, "started_at": row.StartedAt, "completed_at": row.CompletedAt, "node_runs": runs, "approvals": approvals}
}

func (s *Server) workflowInstanceDTOWithDefinition(row workflowgorm.WorkflowInstance, runs []workflowgorm.WorkflowNodeRun, approvals []workflowgorm.WorkflowApproval, definition workflowDefinition) gin.H {
	dto := workflowInstanceDTO(row, runs, approvals)
	aiExecutions := s.workflowAIExecutions(row)
	nodePlans := workflowNodePlans(row)
	currentNodeIDs := currentWorkflowNodes(row.CurrentNodeIDs)
	currentIDs := map[string]bool{}
	for _, nodeID := range currentNodeIDs {
		currentIDs[nodeID] = true
	}
	latestStatus, actualCompletedAt := workflowTimelineState(definition, runs)
	selectedNodeIDs, conditionSelectedEdgeIDs, selectedErr := workflowSelectedTopologyForInstance(s.db, definition, row, runs)
	if selectedErr != nil {
		selectedNodeIDs = workflowAllNodeIDs(definition)
		conditionSelectedEdgeIDs = workflowAllEdgeIDs(definition)
	}
	selectedDisplayNodeIDs, selectedDisplayEdgeIDs := workflowDisplayTopology(definition, runs, conditionSelectedEdgeIDs, currentNodeIDs)
	selectedDisplayNodes := make(map[string]bool, len(selectedDisplayNodeIDs))
	for _, nodeID := range selectedDisplayNodeIDs {
		selectedDisplayNodes[nodeID] = true
	}
	sourceNodeID := ""
	if len(currentNodeIDs) == 1 {
		sourceNodeID = currentNodeIDs[0]
	} else if row.Status == "completed" {
		sourceNodeID = latestCompletedBusinessNode(definition, s.db, row)
	}
	rollbackTargets := workflowRollbackTargets(definition, runs, sourceNodeID, row.Status == "running" && sourceNodeID != "")
	latestTokens := workflowLatestTokensByNode(s.db, row)
	refs := workspaceContextReferences{}.withUserID(row.StartedByUserID)
	for _, token := range latestTokens {
		refs.UserIDs = appendOptionalID(refs.UserIDs, token.AssigneeUserID)
	}
	for _, plan := range nodePlans {
		refs.UserIDs = appendOptionalID(refs.UserIDs, plan.AssigneeUserID)
	}
	switch row.SubjectType {
	case "plan_unit":
		refs.ProjectIDs = appendPositiveID(refs.ProjectIDs, row.SubjectID)
	case "requirement":
		refs.RequirementIDs = appendPositiveID(refs.RequirementIDs, row.SubjectID)
	}
	workspace := s.loadWorkspaceContext(row.WorkspaceID, refs)
	// Subject title backs user-facing copy (lock confirmation text); the
	// intake-lock fact drives the two-step confirmation flow.
	planUnitIntakeLocked := false
	if row.SubjectType == "plan_unit" {
		var unit projectgorm.PlanUnit
		if s.db.Select("title", "requirements_locked_at").
			Where("id = ? AND workspace_id = ?", row.SubjectID, row.WorkspaceID).
			First(&unit).Error == nil {
			dto["subject_title"] = unit.Title
			planUnitIntakeLocked = unit.RequirementsLockedAt != nil
		}
	} else if row.SubjectType == "requirement" {
		var requirementRow requirementgorm.Requirement
		if s.db.Select("title").
			Where("id = ? AND workspace_id = ?", row.SubjectID, row.WorkspaceID).
			First(&requirementRow).Error == nil {
			dto["subject_title"] = requirementRow.Title
		}
	}
	// Guarded nodes share one scope-evidence load per instance DTO build.
	var guardInputs []workflow.GuardRequirementInput
	guardedNodes := false
	if row.SubjectType == "plan_unit" {
		for _, node := range definition.Nodes {
			_, hasGuard := workflowNodeGuard(node)
			_, stageDriven, _ := workflow.NodeStageGuard(definitionNodeFromWorkflow(node))
			if hasGuard || stageDriven {
				guardedNodes = true
				break
			}
		}
	}
	var guardInputsErr error
	if guardedNodes {
		guardInputs, guardInputsErr = workflowGuardInputsForInstance(s.db, row)
	}
	timeline := make([]gin.H, 0, len(definition.Nodes))
	timelineByNodeID := make(map[string]gin.H, len(definition.Nodes))
	totalSteps, completedCount := 0, 0
	for _, node := range definition.Nodes {
		status := "pending"
		if latestStatus[node.ID] == "completed" {
			status = "completed"
		}
		if value := latestStatus[node.ID]; value == "failed" || value == "rejected" || value == "rolled_back" {
			status = value
		}
		if currentIDs[node.ID] {
			status = "active"
		}
		if !selectedNodeIDs[node.ID] {
			status = "skipped"
		}
		parallelCompleted, parallelTotal := 0, 0
		if node.Type == "parallel_join" {
			parallelCompleted, parallelTotal = s.workflowParallelJoinProgress(s.db, definition, row, node.ID)
			if status != "completed" && status != "skipped" && parallelTotal > 0 && parallelCompleted < parallelTotal {
				status = "waiting"
			}
		}
		if status != "skipped" && selectedDisplayNodes[node.ID] && node.Type != "start" && node.Type != "end" && node.Type != "parallel_split" && node.Type != "parallel_join" {
			totalSteps++
			if status == "completed" {
				completedCount++
			}
		}
		execution := aiExecutions[node.ID]
		item := gin.H{"id": node.ID, "type": node.Type, "name": node.Name, "capability": node.Capability, "config": node.Config, "status": status}
		for key, value := range s.workflowNodeActionFields(
			row, definition, node, latestTokens[node.ID], status, execution,
			selectedNodeIDs[node.ID], node.ID == sourceNodeID, len(rollbackTargets) > 0,
		) {
			item[key] = value
		}
		if assignee, _ := item["assignee_user_id"].(*uint); assignee != nil && *assignee > 0 {
			name := userDisplayName(workspace, assignee)
			item["assignee_name"] = name
		}
		if parallelTotal > 0 {
			item["parallel_completed"] = parallelCompleted
			item["parallel_total"] = parallelTotal
		}
		if plan, ok := nodePlans[node.ID]; ok {
			item["planned_start_at"] = plan.PlannedStartAt
			item["planned_end_at"] = plan.PlannedEndAt
		}
		// Requirement-intake facts for the frontend confirmation flow: the
		// backend owns the decision, the client only renders it.
		if workflowNodeRequiresRequirementIntakeLock(row, node) {
			item["requires_requirement_lock_confirmation"] = !planUnitIntakeLocked
			item["requirement_intake_locked"] = planUnitIntakeLocked
		}
		// Linked requirement flow (关联需求流程): expose the evaluation and
		// withdraw success actions while the aggregate condition is unmet.
		// Stage-driven nodes (需求阶段驱动) additionally render as waiting
		// until every scoped requirement reaches the stage.
		guard, guardPresent := workflowNodeGuard(node)
		stageAuto := false
		if !guardPresent && row.SubjectType == "plan_unit" {
			if stageGuard, ok, auto := workflow.NodeStageGuard(definitionNodeFromWorkflow(node)); ok {
				guard, guardPresent, stageAuto = stageGuard, true, auto
			}
		}
		if guardPresent && guardInputsErr == nil {
			evaluation := guard.Evaluate(guardInputs)
			item["requirements_guard"] = evaluation
			if status == "active" && !evaluation.Satisfied() {
				if node.Type == "requirement_gate" || stageAuto {
					item["status"] = "waiting"
				}
				if node.Type != "requirement_gate" && !stageAuto {
					actions, _ := item["allowed_actions"].([]string)
					filtered := make([]string, 0, len(actions))
					for _, action := range actions {
						if action != "complete" && action != "approve" {
							filtered = append(filtered, action)
						}
					}
					item["allowed_actions"] = filtered
					item["can_act"] = len(filtered) > 0
				}
			}
		}
		if completedAt, ok := actualCompletedAt[node.ID]; ok && status == "completed" {
			item["actual_completed_at"] = completedAt
		}
		if execution != nil {
			item["ai_execution"] = execution
		}
		timeline = append(timeline, item)
		timelineByNodeID[node.ID] = item
	}
	progress := 0
	if totalSteps > 0 {
		progress = completedCount * 100 / totalSteps
	} else if row.Status == "completed" {
		progress = 100
	}
	if row.Status == "completed" {
		progress = 100
	}
	current := make([]gin.H, 0, len(currentNodeIDs))
	instanceActions := make([]string, 0)
	instanceCanAct := false
	for _, nodeID := range currentNodeIDs {
		if item := timelineByNodeID[nodeID]; item != nil {
			current = append(current, item)
			if canAct, _ := item["can_act"].(bool); canAct {
				instanceCanAct = true
			}
			if actions, ok := item["allowed_actions"].([]string); ok {
				instanceActions = append(instanceActions, actions...)
			}
		}
	}
	if row.Status == "completed" && sourceNodeID != "" {
		if item := timelineByNodeID[sourceNodeID]; item != nil {
			if canAct, _ := item["can_act"].(bool); canAct {
				instanceCanAct = true
			}
			if actions, ok := item["allowed_actions"].([]string); ok {
				instanceActions = append(instanceActions, actions...)
			}
		}
	}
	if row.Status == "rejected" {
		for index := len(runs) - 1; index >= 0; index-- {
			if runs[index].Status != "rejected" {
				continue
			}
			if item := timelineByNodeID[runs[index].NodeID]; item != nil {
				instanceActions = append(instanceActions, "resume")
				instanceCanAct = true
			}
			break
		}
	}
	instanceActions = uniqueStrings(instanceActions)
	dto["current_nodes"] = current
	dto["timeline_nodes"] = timeline
	dto["node_plans"] = nodePlans
	dto["subject_schedule"] = s.workflowSubjectScheduleDTO(s.db, row, definition, runs)
	dto["definition_edges"] = definition.Edges
	dto["selected_node_ids"] = selectedDisplayNodeIDs
	dto["selected_edge_ids"] = selectedDisplayEdgeIDs
	dto["completed_steps"] = completedCount
	dto["total_steps"] = totalSteps
	dto["progress"] = progress
	dto["transition_history"] = s.workflowTransitionHistory(row, runs, approvals, definition)
	dto["rollback_targets"] = rollbackTargets
	dto["can_act"] = instanceCanAct
	dto["can_manage_node_plans"] = true
	dto["allowed_actions"] = instanceActions
	if len(current) == 1 {
		dto["assignee_user_id"] = current[0]["assignee_user_id"]
		dto["assignee_name"] = current[0]["assignee_name"]
	}
	return dto
}

func attachWorkflowTemplateMeta(dto gin.H, template workflowgorm.WorkflowTemplate, version workflowgorm.WorkflowTemplateVersion) gin.H {
	dto["template_name"] = template.Name
	dto["template_version"] = version.Version
	return dto
}

func definitionNodeFromWorkflow(node workflowNode) workflow.DefinitionNode {
	return workflow.DefinitionNode{
		ID: node.ID, Type: node.Type, Name: node.Name, Capability: node.Capability, Config: node.Config,
	}
}

func syncWorkflowSubjectStatus(tx *gorm.DB, instance workflowgorm.WorkflowInstance, definition workflowDefinition, current []string) error {
	status := ""
	if instance.Status == "completed" {
		status = workflow.SubjectStageStatus(instance.SubjectType, workflow.DefinitionNode{}, instance.Status)
	} else if len(current) > 0 {
		nodes := make([]workflow.DefinitionNode, 0, len(current))
		for _, nodeID := range current {
			node, found := workflowNodeByID(definition, nodeID)
			if !found {
				continue
			}
			nodes = append(nodes, definitionNodeFromWorkflow(node))
		}
		status = workflow.SubjectStatusForCurrent(instance.SubjectType, instance.Status, nodes)
	}
	if status == "" {
		return nil
	}
	if instance.SubjectType == "requirement" {
		return tx.Model(&requirementgorm.Requirement{}).Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).
			Updates(map[string]any{"status": status, "version": gorm.Expr("version + 1")}).Error
	}
	return tx.Model(&projectgorm.PlanUnit{}).Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).
		Update("status", status).Error
}

func completeTerminalNodes(tx *gorm.DB, instance *workflowgorm.WorkflowInstance, definition workflowDefinition, current []string, now time.Time) ([]string, error) {
	remaining := make([]string, 0, len(current))
	completedTerminal := false
	for _, nodeID := range current {
		node, found := workflowNodeByID(definition, nodeID)
		if !found || node.Type != "end" {
			remaining = append(remaining, nodeID)
			continue
		}
		completedTerminal = true
		run := workflowgorm.WorkflowNodeRun{WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeID: node.ID, NodeType: node.Type, Status: "completed", StartedAt: now, CompletedAt: &now}
		if err := tx.Create(&run).Error; err != nil {
			return nil, err
		}
	}
	if len(remaining) == 0 {
		if !completedTerminal {
			return nil, errWorkflowNoTransition
		}
		instance.Status = "completed"
		instance.CompletedAt = &now
	}
	return remaining, nil
}

func removeString(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func (s *Server) workflowNextNodesForInstance(definition workflowDefinition, instance workflowgorm.WorkflowInstance, nodeID, action string) ([]string, error) {
	return workflowNextNodesForInstanceTx(s.db, definition, instance, nodeID, action)
}

func workflowNextNodesForInstanceTx(tx *gorm.DB, definition workflowDefinition, instance workflowgorm.WorkflowInstance, nodeID, action string) ([]string, error) {
	edges, err := workflowNextEdgesForInstanceTx(tx, definition, instance, nodeID, action)
	if err != nil {
		return nil, err
	}
	targets := make([]string, 0, len(edges))
	for _, edge := range edges {
		targets = append(targets, edge.Target)
	}
	return uniqueStrings(targets), nil
}

func workflowNextEdgesForInstanceTx(tx *gorm.DB, definition workflowDefinition, instance workflowgorm.WorkflowInstance, nodeID, action string) ([]workflowEdge, error) {
	subject, err := workflowSubjectValue(tx, instance)
	if err != nil {
		return nil, err
	}
	environment, err := cel.NewEnv(cel.Variable("subject", cel.DynType), cel.Variable("context", cel.DynType))
	if err != nil {
		return nil, workflowTopologySelectionFailure(err)
	}
	decision, hasDecision := workflowConditionDecision(action)
	if hasDecision {
		// `needs_ui` keeps already-published requirement templates compatible;
		// new templates should prefer the generic `context.decision` value.
		subject["needs_ui"] = decision
	}
	result, fallback := make([]workflowEdge, 0), make([]workflowEdge, 0)
	for _, edge := range definition.Edges {
		if edge.Source != nodeID || !workflowEdgeMatchesAction(edge.Kind, action) {
			continue
		}
		if strings.TrimSpace(edge.Condition) == "" {
			fallback = append(fallback, edge)
			continue
		}
		ast, issues := environment.Compile(edge.Condition)
		if issues != nil && issues.Err() != nil {
			return nil, workflowTopologySelectionFailure(issues.Err())
		}
		program, err := environment.Program(ast)
		if err != nil {
			return nil, workflowTopologySelectionFailure(err)
		}
		value, _, err := program.Eval(map[string]any{"subject": subject, "context": map[string]any{"action": action, "decision": decision, "workspace_id": instance.WorkspaceID, "instance_id": instance.ID}})
		if err != nil {
			return nil, workflowTopologySelectionFailure(err)
		}
		matched, ok := value.Value().(bool)
		if !ok {
			return nil, workflowTopologySelectionFailure(errors.New("workflow condition did not return boolean"))
		}
		if matched {
			result = append(result, edge)
		}
	}
	if len(result) > 0 {
		return result, nil
	}
	return fallback, nil
}

func workflowConditionDecision(action string) (bool, bool) {
	switch action {
	case "condition_yes":
		return true, true
	case "condition_no":
		return false, true
	default:
		return false, false
	}
}

func workflowNodeRequiresDesignAsset(definition workflowDefinition, node workflowNode) bool {
	if required, _ := node.Config["require_design_image"].(bool); required {
		return true
	}
	// Compatibility for immutable definitions published before design materials
	// belonged to the branch target rather than the condition node itself.
	for _, edge := range definition.Edges {
		if edge.Target == node.ID && strings.Contains(edge.Condition, "needs_ui") && strings.Contains(edge.Condition, "true") {
			return true
		}
	}
	return false
}

func workflowSubjectValue(tx *gorm.DB, instance workflowgorm.WorkflowInstance) (map[string]any, error) {
	var value any
	if instance.SubjectType == "requirement" {
		var row requirementgorm.Requirement
		if err := tx.Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).First(&row).Error; err != nil {
			return nil, err
		}
		value = row
	} else {
		var row projectgorm.PlanUnit
		if err := tx.Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).First(&row).Error; err != nil {
			return nil, err
		}
		value = row
	}
	raw, _ := json.Marshal(value)
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func workflowTimerReady(tx *gorm.DB, instance workflowgorm.WorkflowInstance, node workflowNode) bool {
	delay, ok := node.Config["delay_seconds"].(float64)
	if !ok || delay <= 0 {
		return true
	}
	var token workflowgorm.WorkflowToken
	if tx.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", instance.WorkspaceID, instance.ID, node.ID, "active").Order("id DESC").First(&token).Error != nil {
		return false
	}
	return !time.Now().UTC().Before(token.CreatedAt.Add(time.Duration(delay) * time.Second))
}

var errWorkflowAIWaiting = errors.New("workflow AI run is still processing")
var errWorkflowAIOverrideUnavailable = errors.New("workflow AI result cannot be overridden")
var errWorkflowNoTransition = errors.New("workflow action has no valid transition")

func currentWorkflowNodes(raw string) []string {
	var values []string
	_ = json.Unmarshal([]byte(raw), &values)
	return values
}
func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func workflowNodeByID(def workflowDefinition, id string) (workflowNode, bool) {
	for _, node := range def.Nodes {
		if node.ID == id {
			return node, true
		}
	}
	return workflowNode{}, false
}
func workflowNextNodes(def workflowDefinition, id, action string) []string {
	result := []string{}
	for _, edge := range def.Edges {
		if edge.Source == id && workflowEdgeMatchesAction(edge.Kind, action) {
			result = append(result, edge.Target)
		}
	}
	return uniqueStrings(result)
}

func workflowEdgeMatchesAction(kind, action string) bool {
	if action == "failure" || action == "reject" {
		return kind == action
	}
	if action == "condition_yes" {
		return kind == "" || kind == "success" || kind == "yes" || kind == action
	}
	if action == "condition_no" {
		return kind == "no" || kind == action || kind == ""
	}
	return kind == "" || kind == "success" || kind == action
}
