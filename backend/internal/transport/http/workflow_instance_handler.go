package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

func (s *Server) startWorkflowInstance(c *gin.Context) {
	var body struct {
		TemplateID  uint   `json:"template_id"`
		SubjectType string `json:"subject_type"`
		SubjectID   uint   `json:"subject_id"`
	}
	if c.ShouldBindJSON(&body) != nil || body.TemplateID == 0 || body.SubjectID == 0 || !validWorkflowSubject(body.SubjectType) {
		bad(c, "template_id, subject_type and subject_id required")
		return
	}
	fail(c, http.StatusConflict, "workflow template must be selected when the subject is created")
}

func (s *Server) listWorkflowInstances(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	subjectType := strings.TrimSpace(c.Query("subject_type"))
	subjectID := queryUint(c, "subject_id")
	if !validWorkflowSubject(subjectType) || subjectID == 0 {
		bad(c, "subject_type and subject_id required")
		return
	}
	if !s.workflowSubjectExists(workspace.ID, subjectType, subjectID) {
		fail(c, http.StatusNotFound, "workflow subject not found")
		return
	}
	// Gate project access for the workflow subject.
	if pid := s.workflowSubjectProjectID(s.db, workspace.ID, subjectType, subjectID); pid > 0 {
		if !s.requireProjectAccess(c, workspace.ID, pid) {
			return
		}
	}
	var rows []workflowgorm.WorkflowInstance
	if s.db.Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspace.ID, subjectType, subjectID).Order("id DESC").Limit(20).Find(&rows).Error != nil {
		dbFail(c)
		return
	}
	data := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		var version workflowgorm.WorkflowTemplateVersion
		var runs []workflowgorm.WorkflowNodeRun
		if s.db.Where("id = ? AND workspace_id = ?", row.TemplateVersionID, row.WorkspaceID).First(&version).Error != nil ||
			s.db.Where("workspace_id = ? AND instance_id = ?", row.WorkspaceID, row.ID).Order("id").Find(&runs).Error != nil {
			dbFail(c)
			return
		}
		definition, issues := compileWorkflowDefinition(version.Definition)
		if len(issues) > 0 {
			fail(c, http.StatusConflict, "workflow definition unavailable")
			return
		}
		dto := workflowInstanceDTO(row, runs, nil)
		dto["subject_schedule"] = s.workflowSubjectScheduleDTO(s.db, row, definition, runs)
		data = append(data, dto)
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}

func (s *Server) getWorkflowInstance(c *gin.Context) {
	instance, okRow := s.workflowInstanceByID(c)
	if !okRow {
		return
	}
	var runs []workflowgorm.WorkflowNodeRun
	var approvals []workflowgorm.WorkflowApproval
	s.db.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).Order("id").Find(&runs)
	s.db.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).Order("id").Find(&approvals)
	var version workflowgorm.WorkflowTemplateVersion
	var template workflowgorm.WorkflowTemplate
	var definition workflowDefinition
	if s.db.Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, instance.WorkspaceID).First(&version).Error == nil {
		definition, _ = compileWorkflowDefinition(version.Definition)
	}
	_ = s.db.Where("id = ? AND workspace_id = ?", instance.TemplateID, instance.WorkspaceID).First(&template).Error
	ok(c, attachWorkflowTemplateMeta(s.workflowInstanceDTOWithDefinition(instance, runs, approvals, definition), template, version))
}

func (s *Server) actOnWorkflowInstance(c *gin.Context) {
	instance, okRow := s.workflowInstanceByID(c)
	if !okRow {
		return
	}
	if !s.requireWorkflowRequirementCreatorAction(c, instance) {
		return
	}
	var body struct {
		Action                 string                      `json:"action"`
		NodeID                 string                      `json:"node_id"`
		TargetNodeID           string                      `json:"target_node_id"`
		Comment                string                      `json:"comment"`
		DesignURL              string                      `json:"design_url"`
		SkipTaskPlan           bool                        `json:"skip_task_plan"`
		ConfirmRequirementLock bool                        `json:"confirm_requirement_lock"`
		NextNodeAssignees      []workflowNextAssigneeInput `json:"next_node_assignees"`
	}
	if c.ShouldBindJSON(&body) != nil || strings.TrimSpace(body.Action) == "" {
		bad(c, "action required")
		return
	}
	body.Action = strings.TrimSpace(body.Action)
	canonicalAction := body.Action
	if canonicalAction == "advance" {
		canonicalAction = "complete"
	}
	var version workflowgorm.WorkflowTemplateVersion
	if s.db.Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, instance.WorkspaceID).First(&version).Error != nil {
		dbFail(c)
		return
	}
	definition, issues := compileWorkflowDefinition(version.Definition)
	if len(issues) > 0 {
		fail(c, http.StatusConflict, "workflow definition unavailable")
		return
	}
	if body.Action == "resume" {
		if instance.Status != "rejected" {
			fail(c, http.StatusConflict, "only a rejected workflow can be resumed")
			return
		}
		s.resumeRejectedWorkflowInstance(c, instance, definition)
		return
	}
	if body.Action == "rollback" {
		s.rollbackWorkflowInstance(c, instance, definition, body.NodeID, body.TargetNodeID, body.Comment)
		return
	}
	if body.Action == "supplement_design" {
		s.supplementWorkflowDesign(c, instance, definition, body.NodeID, body.DesignURL)
		return
	}
	current := currentWorkflowNodes(instance.CurrentNodeIDs)
	if instance.Status != "running" {
		fail(c, http.StatusConflict, "workflow is not running")
		return
	}
	nodeID := body.NodeID
	if nodeID == "" && len(current) == 1 {
		nodeID = current[0]
	}
	if body.Action == "recheck_ai" || body.Action == "retry_ai" {
		node, found := workflowNodeByID(definition, nodeID)
		if !found || !strings.HasPrefix(node.Type, "ai_") {
			fail(c, http.StatusConflict, "only an AI node can be rechecked")
			return
		}
		if !s.cfg.AI.Enabled {
			fail(c, http.StatusServiceUnavailable, errAIRuntimeDisabled.Error())
			return
		}
		if !s.workflowAINodeCanRecheck(instance, definition, node) {
			fail(c, http.StatusConflict, "AI node must be active or have a selected execution history before recheck")
			return
		}
		if !s.requireWorkflowNodeActive(c, instance, node.ID, containsString(current, node.ID)) {
			return
		}
		nodeStatus := "completed"
		if containsString(current, node.ID) {
			nodeStatus = "active"
		}
		execution := s.workflowAIExecutions(instance)[node.ID]
		if !containsString(s.workflowNodeAllowedActions(
			instance, definition, node, nodeStatus, execution, true, false, false,
		), body.Action) {
			fail(c, http.StatusConflict, "workflow action is not allowed for the current AI result")
			return
		}
		s.recheckWorkflowAINode(c, instance, definition, node, body.Action)
		return
	}
	if !containsString(current, nodeID) {
		fail(c, http.StatusConflict, "node is not active")
		return
	}
	node, found := workflowNodeByID(definition, nodeID)
	if !found {
		fail(c, http.StatusConflict, "active node missing from definition")
		return
	}
	if !s.requireWorkflowNodeActive(c, instance, node.ID, true) {
		return
	}
	if strings.HasPrefix(node.Type, "ai_") && !s.cfg.AI.Enabled {
		fail(c, http.StatusServiceUnavailable, errAIRuntimeDisabled.Error())
		return
	}
	isAINode := strings.HasPrefix(node.Type, "ai_")
	isCondition := node.Type == "condition"
	if body.Action == "override_ai" && (!isAINode || node.Capability != "requirement.completeness") {
		fail(c, http.StatusConflict, "only requirement completeness AI results can be manually overridden")
		return
	}
	if body.SkipTaskPlan && (canonicalAction != "complete" || !isAINode || node.Capability != "requirement.task_plan") {
		fail(c, http.StatusConflict, "only a requirement task plan result can be skipped")
		return
	}
	if node.Type == "approval" && body.Action != "approve" && body.Action != "reject" {
		fail(c, http.StatusConflict, "approval node requires approve or reject")
		return
	}
	if isCondition && body.Action != "condition_yes" && body.Action != "condition_no" {
		fail(c, http.StatusConflict, "condition node requires an explicit yes or no decision")
		return
	}
	if node.Type != "approval" && !isCondition && canonicalAction != "complete" && body.Action != "override_ai" {
		fail(c, http.StatusConflict, "node requires advance or complete")
		return
	}
	if !s.guardWorkflowRequirementIntakeLock(c, instance, node, canonicalAction, body.ConfirmRequirementLock) {
		return
	}
	conditionDecision, hasConditionDecision := workflowConditionDecision(body.Action)
	if hasConditionDecision {
		body.DesignURL = ""
		if strings.TrimSpace(body.Comment) == "" {
			if conditionDecision {
				body.Comment = "条件选择：是。"
			} else {
				body.Comment = "条件选择：否。"
			}
		}
	}
	if instance.SubjectType == "requirement" && workflowNodeRequiresDesignAsset(definition, node) {
		hasDesign, err := s.materials.HasKind(instance.WorkspaceID, instance.SubjectID, requirement.MaterialKindDesign)
		if err != nil {
			dbFail(c)
			return
		}
		body.DesignURL = strings.TrimSpace(body.DesignURL)
		if resolved, ok := s.resolveRequirementDesignURL(instance.WorkspaceID, instance.SubjectID, body.DesignURL); ok {
			body.DesignURL = resolved
		}
		if !hasDesign && !validHTTPURL(body.DesignURL) {
			failCode(c, http.StatusConflict, "WORKFLOW_DESIGN_ASSET_REQUIRED", "a valid uploaded design image or design link is required before completing this node")
			return
		}
		if strings.TrimSpace(body.Comment) == "" {
			body.Comment = "已提交设计效果图或链接。"
		}
	}
	if node.Type == "parallel_join" && !s.workflowJoinReady(s.db, definition, instance, node.ID) {
		failCode(c, http.StatusConflict, "WORKFLOW_PARALLEL_JOIN_WAITING", "parallel join is waiting for all incoming branches")
		return
	}
	if node.Type == "timer" && !workflowTimerReady(s.db, instance, node) {
		failCode(c, http.StatusConflict, "WORKFLOW_TIMER_NOT_DUE", "timer node is not due")
		return
	}
	if body.Action == "reject" {
		previousNodeID, err := s.workflowPreviousBusinessNode(instance, definition, nodeID)
		if err != nil {
			dbFail(c)
			return
		}
		next := []string{}
		if previousNodeID != "" {
			next = []string{previousNodeID}
		} else {
			next, err = s.workflowNextNodesForInstance(definition, instance, nodeID, body.Action)
			if err != nil {
				fail(c, http.StatusConflict, err.Error())
				return
			}
		}
		if len(next) == 0 {
			fail(c, http.StatusConflict, "approval has no previous business node or configured reject path")
			return
		}
		s.finishWorkflowNode(c, instance, definition, node, "rejected", body.Comment, next, "", false, nil, "", nil, false)
		return
	}
	next, err := s.workflowNextNodesForInstance(definition, instance, nodeID, body.Action)
	if err != nil {
		fail(c, http.StatusConflict, err.Error())
		return
	}
	if len(next) == 0 {
		fail(c, http.StatusConflict, errWorkflowNoTransition.Error())
		return
	}
	overridePath := ""
	if body.Action == "override_ai" {
		overridePath = "success"
		if strings.TrimSpace(body.Comment) == "" {
			body.Comment = "人工确认完整性风险，允许低分需求继续进入下一流程。"
		}
	}
	var decision *bool
	if hasConditionDecision {
		decision = &conditionDecision
	}
	nextAssignees, parseErr := parseWorkflowNextNodeAssignees(body.NextNodeAssignees, definition)
	if parseErr != nil {
		bad(c, parseErr.Error())
		return
	}
	s.finishWorkflowNode(c, instance, definition, node, "completed", body.Comment, next, overridePath, body.SkipTaskPlan, decision, body.DesignURL, nextAssignees, body.ConfirmRequirementLock)
}

func (s *Server) workflowAINodeCanRecheck(instance workflowgorm.WorkflowInstance, definition workflowDefinition, node workflowNode) bool {
	if containsString(currentWorkflowNodes(instance.CurrentNodeIDs), node.ID) {
		return true
	}
	selected, err := workflowNodeIsSelected(s.db, definition, instance, node.ID)
	if err != nil || !selected {
		return false
	}
	var history int64
	return s.db.Model(&workflowgorm.WorkflowNodeRun{}).
		Where("workspace_id = ? AND instance_id = ? AND node_id = ?", instance.WorkspaceID, instance.ID, node.ID).
		Count(&history).Error == nil && history > 0
}
