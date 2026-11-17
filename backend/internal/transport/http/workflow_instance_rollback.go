package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

var errWorkflowRollbackChanged = errors.New("workflow changed before rollback")

func workflowReachableFrom(definition workflowDefinition, sourceID string) map[string]bool {
	reachable := map[string]bool{}
	queue := []string{sourceID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if reachable[current] {
			continue
		}
		reachable[current] = true
		for _, edge := range definition.Edges {
			// Failure and rejection edges model recovery paths back into an
			// earlier stage. Following them while replaying a rollback can loop
			// into a completed sibling branch and incorrectly reset its status.
			if edge.Source == current && edge.Kind != "failure" && edge.Kind != "reject" && !reachable[edge.Target] {
				queue = append(queue, edge.Target)
			}
		}
	}
	return reachable
}

func workflowTimelineState(definition workflowDefinition, runs []workflowgorm.WorkflowNodeRun) (map[string]string, map[string]time.Time) {
	statuses := map[string]string{}
	completedAt := map[string]time.Time{}
	for _, run := range runs {
		// Supplemental material is immutable node history, not another execution
		// attempt. It must never change completion, progress, or rollback state.
		if run.Status == "supplemented" {
			continue
		}
		actor := decodeWorkflowRunActor(run.OutputJSON)
		if actor.RollbackTargetNodeID != "" {
			for nodeID := range workflowReachableFrom(definition, actor.RollbackTargetNodeID) {
				statuses[nodeID] = "pending"
				delete(completedAt, nodeID)
			}
			if run.NodeID != actor.RollbackTargetNodeID {
				statuses[run.NodeID] = "rolled_back"
			}
			continue
		}
		if targetNodeID := workflowCompletedAIRecoveryTarget(definition, run, actor); targetNodeID != "" {
			for nodeID := range workflowReachableFrom(definition, targetNodeID) {
				statuses[nodeID] = "pending"
				delete(completedAt, nodeID)
			}
			statuses[run.NodeID] = "rolled_back"
			continue
		}
		statuses[run.NodeID] = run.Status
		if run.Status == "completed" && run.CompletedAt != nil {
			completedAt[run.NodeID] = *run.CompletedAt
		} else {
			delete(completedAt, run.NodeID)
		}
	}
	return statuses, completedAt
}

func workflowCompletedAIRecoveryTarget(
	definition workflowDefinition,
	run workflowgorm.WorkflowNodeRun,
	actor workflowRunActor,
) string {
	if run.Status != "completed" || !strings.HasPrefix(run.NodeType, "ai_") || actor.TransitionTargetNodeID == "" {
		return ""
	}
	for _, edge := range definition.Edges {
		if edge.Source == run.NodeID && edge.Target == actor.TransitionTargetNodeID &&
			(edge.Kind == "failure" || edge.Kind == "reject") &&
			workflowRecoveryTargetIsUnambiguous(definition, edge) {
			return edge.Target
		}
	}
	return ""
}

func workflowRollbackTargets(definition workflowDefinition, runs []workflowgorm.WorkflowNodeRun, sourceNodeID string, sourceIsActive bool) []gin.H {
	targetIDs := workflowRollbackTargetIDs(definition, runs, sourceNodeID, sourceIsActive)
	targets := make([]gin.H, 0)
	for _, node := range definition.Nodes {
		if targetIDs[node.ID] {
			targets = append(targets, gin.H{"id": node.ID, "name": node.Name, "type": node.Type})
		}
	}
	return targets
}

func (s *Server) rollbackWorkflowInstance(c *gin.Context, instance workflowgorm.WorkflowInstance, definition workflowDefinition, sourceNodeID, targetNodeID, comment string) {
	if instance.Status != "running" && instance.Status != "completed" {
		fail(c, http.StatusConflict, "only a running or completed workflow can be rolled back")
		return
	}
	comment = strings.TrimSpace(comment)
	targetNodeID = strings.TrimSpace(targetNodeID)
	if targetNodeID == "" || comment == "" {
		bad(c, "target_node_id and rollback comment required")
		return
	}
	current := currentWorkflowNodes(instance.CurrentNodeIDs)
	if instance.Status == "running" {
		sourceNodeID = strings.TrimSpace(sourceNodeID)
		if !containsString(current, sourceNodeID) {
			fail(c, http.StatusConflict, "rollback source node is not active")
			return
		}
	} else {
		sourceNodeID = latestCompletedBusinessNode(definition, s.db, instance)
		if sourceNodeID == "" {
			fail(c, http.StatusConflict, "completed workflow has no business node to roll back")
			return
		}
	}
	var runs []workflowgorm.WorkflowNodeRun
	if s.db.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).Order("id").Find(&runs).Error != nil {
		dbFail(c)
		return
	}
	validTargets := workflowRollbackTargetIDs(definition, runs, sourceNodeID, instance.Status == "running")
	if !validTargets[targetNodeID] {
		fail(c, http.StatusConflict, "rollback target must be a completed predecessor on the source execution path")
		return
	}
	affectedCurrent, nextCurrent, planned := workflowRollbackCurrentPlan(definition, runs, current, sourceNodeID, targetNodeID)
	if !planned {
		fail(c, http.StatusConflict, "rollback cannot safely preserve the current parallel branches")
		return
	}
	sourceNode, sourceFound := workflowNodeByID(definition, sourceNodeID)
	targetNode, targetFound := workflowNodeByID(definition, targetNodeID)
	if !sourceFound || !targetFound {
		fail(c, http.StatusConflict, "rollback node is unavailable")
		return
	}
	if !s.requireWorkflowNodeActive(c, instance, sourceNode.ID, instance.Status == "running") {
		return
	}
	var observedTokens []workflowgorm.WorkflowToken
	if err := s.db.Where(
		"workspace_id = ? AND instance_id = ? AND node_id IN ? AND status IN ?",
		instance.WorkspaceID, instance.ID, current, []string{"active", "waiting"},
	).Order("id ASC").Find(&observedTokens).Error; err != nil {
		dbFail(c)
		return
	}
	affectedSet := make(map[string]bool, len(affectedCurrent))
	for _, nodeID := range affectedCurrent {
		affectedSet[nodeID] = true
	}
	currentTokens := make([]workflowgorm.WorkflowToken, 0, len(affectedCurrent))
	if instance.Status == "running" {
		byNode := make(map[string]int, len(current))
		for _, token := range observedTokens {
			byNode[token.NodeID]++
			if affectedSet[token.NodeID] {
				currentTokens = append(currentTokens, token)
			}
		}
		for _, nodeID := range current {
			if byNode[nodeID] != 1 {
				fail(c, http.StatusConflict, "workflow changed before rollback")
				return
			}
		}
	}
	sort.Slice(currentTokens, func(left, right int) bool {
		if currentTokens[left].NodeID == currentTokens[right].NodeID {
			return currentTokens[left].ID < currentTokens[right].ID
		}
		return currentTokens[left].NodeID < currentTokens[right].NodeID
	})
	expectedProjectID, projectErr := workflowSubjectProjectIDForGate(
		s.db, instance.WorkspaceID, instance.SubjectType, instance.SubjectID,
	)
	if projectErr != nil {
		if !respondProjectChildScopeConflict(c, projectErr) {
			dbFail(c)
		}
		return
	}
	now := time.Now().UTC()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.lockWorkflowSubjectProjectTx(tx, instance, expectedProjectID); err != nil {
			return err
		}
		// Workflow actions and node-plan updates lock claimed tokens before the
		// instance after the shared project/subject gate. Preserve that order and
		// CAS every affected token fact.
		for _, token := range currentTokens {
			query := tx.Model(&workflowgorm.WorkflowToken{}).Where(
				"id = ? AND workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
				token.ID, instance.WorkspaceID, instance.ID, token.NodeID, token.Status,
			)
			if token.AssigneeUserID == nil {
				query = query.Where("assignee_user_id IS NULL")
			} else {
				query = query.Where("assignee_user_id = ?", *token.AssigneeUserID)
			}
			changed := query.Update("status", "rolled_back")
			if changed.Error != nil {
				return changed.Error
			}
			if changed.RowsAffected != 1 {
				return errWorkflowRollbackChanged
			}
		}
		query := tx.Model(&workflowgorm.WorkflowInstance{}).Where(
			"id = ? AND workspace_id = ? AND status = ? AND current_node_ids = ? AND node_plans_json = ?",
			instance.ID, instance.WorkspaceID, instance.Status, instance.CurrentNodeIDs, instance.NodePlansJSON,
		)
		encoded, _ := json.Marshal(nextCurrent)
		updated := query.Updates(map[string]any{"status": "running", "current_node_ids": string(encoded), "completed_at": nil})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errWorkflowRollbackChanged
		}
		if err := s.cancelActiveWorkflowAIForRollback(tx, instance, affectedCurrent, now); err != nil {
			return err
		}
		if err := createWorkflowTokenTx(tx, instance, definition, targetNodeID, "active", true); err != nil {
			return err
		}
		rollbackRun := workflowgorm.WorkflowNodeRun{
			WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeID: sourceNodeID,
			NodeType: sourceNode.Type, Status: "rolled_back", OutputJSON: encodeWorkflowRunRollback(currentFactUserID(c), comment, targetNodeID),
			StartedAt: now, CompletedAt: &now,
		}
		if err := tx.Create(&rollbackRun).Error; err != nil {
			return err
		}
		instance.Status, instance.CurrentNodeIDs, instance.CompletedAt = "running", string(encoded), nil
		if err := syncWorkflowSubjectStatus(tx, instance, definition, nextCurrent); err != nil {
			return err
		}
		if err := s.queueEnteredWorkflowAINodesTx(tx, c, instance, definition, []string{targetNodeID}, now); err != nil {
			return err
		}
		if err := s.recordMutation(tx, c, expectedProjectID, "workflow_instance", instance.ID, "rolled_back", targetNode.Name, gin.H{
			"source_node_id": sourceNodeID, "target_node_id": targetNodeID, "comment": comment,
		}); err != nil {
			return err
		}
		// Evidence invalidated by the rollback: the scheduled plan unit's
		// gates and delivery progress must re-evaluate.
		if instance.SubjectType == "requirement" {
			if unitID := requirementScheduledPlanUnitTx(tx, instance.WorkspaceID, instance.SubjectID); unitID > 0 {
				if err := recordDeliveryReconcileTx(tx, instance.WorkspaceID, unitID, "requirement_rollback"); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if respondProjectChildScopeConflict(c, err) {
			return
		}
		if errors.Is(err, errWorkflowRollbackChanged) {
			fail(c, http.StatusConflict, err.Error())
			return
		}
		if workflowNodePlanDatabaseConflict(err) {
			fail(c, http.StatusConflict, "workflow changed before rollback")
			return
		}
		dbFail(c)
		return
	}
	s.writeWorkflowInstanceResponse(c, instance.ID, instance.WorkspaceID, definition)
}

func latestCompletedBusinessNode(definition workflowDefinition, tx *gorm.DB, instance workflowgorm.WorkflowInstance) string {
	var runs []workflowgorm.WorkflowNodeRun
	if tx.Where("workspace_id = ? AND instance_id = ? AND status = ?", instance.WorkspaceID, instance.ID, "completed").Order("id DESC").Find(&runs).Error != nil {
		return ""
	}
	for _, run := range runs {
		if node, found := workflowNodeByID(definition, run.NodeID); found {
			switch node.Type {
			case "start", "end", "parallel_split", "parallel_join":
				continue
			default:
				return node.ID
			}
		}
	}
	return ""
}

func (s *Server) cancelActiveWorkflowAIForRollback(tx *gorm.DB, instance workflowgorm.WorkflowInstance, current []string, now time.Time) error {
	if len(current) == 0 {
		return nil
	}
	var waiting []workflowgorm.WorkflowNodeRun
	if err := tx.Where("workspace_id = ? AND instance_id = ? AND node_id IN ? AND status = ?", instance.WorkspaceID, instance.ID, current, "waiting").Find(&waiting).Error; err != nil {
		return err
	}
	for _, run := range waiting {
		var job workflowgorm.WorkflowJob
		if err := tx.Where("workspace_id = ? AND instance_id = ? AND node_run_id = ?", instance.WorkspaceID, instance.ID, run.ID).First(&job).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if job.ID > 0 {
			var payload struct {
				AIRunID uint `json:"ai_run_id"`
			}
			_ = json.Unmarshal([]byte(job.PayloadJSON), &payload)
			if payload.AIRunID > 0 {
				if err := tx.Model(&aigorm.AIRun{}).Where("id = ? AND workspace_id = ? AND status IN ?", payload.AIRunID, instance.WorkspaceID, []string{"queued", "running"}).Updates(map[string]any{"status": "cancelled", "completed_at": now}).Error; err != nil {
					return err
				}
			}
			if err := tx.Model(&job).Updates(map[string]any{"status": "cancelled", "last_error": "workflow rolled back"}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&run).Updates(map[string]any{"status": "cancelled", "completed_at": now, "error_message": "workflow rolled back"}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) supplementWorkflowDesign(c *gin.Context, instance workflowgorm.WorkflowInstance, definition workflowDefinition, nodeID, designURL string) {
	node, found := workflowNodeByID(definition, strings.TrimSpace(nodeID))
	if instance.SubjectType != "requirement" || !found || !workflowNodeRequiresDesignAsset(definition, node) {
		fail(c, http.StatusConflict, "workflow node does not accept a design material")
		return
	}
	if !s.requireWorkflowNodeActive(c, instance, node.ID, containsString(currentWorkflowNodes(instance.CurrentNodeIDs), node.ID)) {
		return
	}
	selected, selectedErr := workflowNodeIsSelected(s.db, definition, instance, node.ID)
	if selectedErr != nil {
		fail(c, http.StatusConflict, "workflow selected path is unavailable")
		return
	}
	if !selected {
		fail(c, http.StatusConflict, "workflow node is outside the selected path")
		return
	}
	designURL = strings.TrimSpace(designURL)
	resolved, ok := s.resolveRequirementDesignURL(instance.WorkspaceID, instance.SubjectID, designURL)
	if !ok {
		bad(c, "a valid design_url is required")
		return
	}
	designURL = resolved
	now := time.Now().UTC()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := lockWorkflowInstanceTx(tx, instance.WorkspaceID, instance.ID); err != nil {
			return err
		}
		var requirement requirementgorm.Requirement
		if err := tx.Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).First(&requirement).Error; err != nil {
			return err
		}
		projectID := s.workflowSubjectProjectID(tx, instance.WorkspaceID, instance.SubjectType, instance.SubjectID)
		if requirement.DesignURL != designURL {
			if err := tx.Model(&requirement).Updates(map[string]any{"design_url": designURL, "version": gorm.Expr("version + 1")}).Error; err != nil {
				return err
			}
			if err := s.recordMutation(tx, c, projectID, "requirement", instance.SubjectID, "design_attached", node.Name, gin.H{"design_url": designURL, "workflow_instance_id": instance.ID, "node_id": node.ID}); err != nil {
				return err
			}
		}
		run := workflowgorm.WorkflowNodeRun{
			WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeID: node.ID,
			NodeType: node.Type, Status: "supplemented",
			OutputJSON: encodeWorkflowRunMaterial(currentFactUserID(c), "补交设计效果图或链接。", designURL),
			StartedAt:  now, CompletedAt: &now,
		}
		if err := tx.Create(&run).Error; err != nil {
			return err
		}
		return s.recordMutation(tx, c, projectID, "workflow_instance", instance.ID, "material_supplemented", node.Name, gin.H{"node_id": node.ID, "kind": "design_image"})
	})
	if err != nil {
		dbFail(c)
		return
	}
	s.writeWorkflowInstanceResponse(c, instance.ID, instance.WorkspaceID, definition)
}

func (s *Server) writeWorkflowInstanceResponse(c *gin.Context, instanceID, workspaceID uint, definition workflowDefinition) {
	var instance workflowgorm.WorkflowInstance
	var runs []workflowgorm.WorkflowNodeRun
	var approvals []workflowgorm.WorkflowApproval
	var template workflowgorm.WorkflowTemplate
	var version workflowgorm.WorkflowTemplateVersion
	if s.db.Where("id = ? AND workspace_id = ?", instanceID, workspaceID).First(&instance).Error != nil {
		dbFail(c)
		return
	}
	_ = s.db.Where("workspace_id = ? AND instance_id = ?", workspaceID, instanceID).Order("id").Find(&runs).Error
	_ = s.db.Where("workspace_id = ? AND instance_id = ?", workspaceID, instanceID).Order("id").Find(&approvals).Error
	_ = s.db.Where("id = ? AND workspace_id = ?", instance.TemplateID, workspaceID).First(&template).Error
	_ = s.db.Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, workspaceID).First(&version).Error
	ok(c, attachWorkflowTemplateMeta(s.workflowInstanceDTOWithDefinition(instance, runs, approvals, definition), template, version))
}
