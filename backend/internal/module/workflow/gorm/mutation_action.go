package workflowgorm

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func applyWorkflowAction(
	ctx context.Context,
	tx *gorm.DB,
	dependencies MutationDependencies,
	command workflow.Mutation,
	state workflowMutationState,
) (workflow.MutationResult, error) {
	switch command.Action {
	case "retry_ai", "recheck_ai":
		return applyWorkflowAIRecheck(ctx, tx, dependencies, command, state)
	case "rollback":
		return applyWorkflowRollback(ctx, tx, dependencies, command, state)
	case "resume":
		return applyWorkflowResume(ctx, tx, dependencies, command, state)
	case "supplement_design":
		return applyWorkflowSupplementDesign(ctx, tx, dependencies, command, state)
	}
	if len(command.NextAssignees) > 0 {
		assigneeIDs := make([]uint, 0, len(command.NextAssignees))
		for nodeID, userID := range command.NextAssignees {
			node, found := state.definition.Node(strings.TrimSpace(nodeID))
			if !found || !workflow.NeedsAssignee(node) || userID == 0 {
				return workflow.MutationResult{}, workflow.ErrMutationInvalid
			}
			assigneeIDs = append(assigneeIDs, userID)
		}
		if err := dependencies.Users.VerifyActiveWorkspaceUsers(ctx, command.WorkspaceID, uniqueWorkflowUserIDs(assigneeIDs)); err != nil {
			return workflow.MutationResult{}, workflow.ErrMutationConflict
		}
	}
	current := currentMutationNodeIDs(state.instance.CurrentNodeIDs)
	if state.instance.Status != "running" || len(current) == 0 {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	nodeID := strings.TrimSpace(command.NodeID)
	if nodeID == "" && len(current) == 1 {
		nodeID = current[0]
	}
	if !containsMutationNode(current, nodeID) {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	node, found := state.definition.Node(nodeID)
	if !found || !mutationNodeActionAllowed(node, command.Action) || !workflow.NeedsAssignee(node) {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	// Project requirement-intake lock: the confirmation flag expresses user
	// intent, never permission. Unrelated nodes or actions carrying the flag
	// are a parameter error, and a pending lock confirmation must not be
	// silently downgraded to a plain completion.
	intakeLockPending := false
	if command.ConfirmRequirementLock && !workflow.MutationNodeRequiresRequirementIntakeLock(state.instance.SubjectType, node, command.Action) {
		return workflow.MutationResult{}, workflow.ErrMutationInvalid
	}
	if workflow.MutationNodeRequiresRequirementIntakeLock(state.instance.SubjectType, node, command.Action) {
		if dependencies.IntakeLocks == nil || dependencies.IntakeEffects == nil {
			return workflow.MutationResult{}, workflow.ErrMutationUnavailable
		}
		alreadyLocked, lockErr := dependencies.IntakeLocks.RequirementIntakeLocked(ctx, command.WorkspaceID, state.instance.SubjectID)
		if lockErr != nil {
			return workflow.MutationResult{}, lockErr
		}
		if !alreadyLocked && !command.ConfirmRequirementLock {
			return workflow.MutationResult{}, workflow.ErrRequirementLockConfirmationRequired
		}
		intakeLockPending = !alreadyLocked
	}
	// requirements_guard: success actions on guarded plan-unit nodes wait for
	// the linked requirement evidence, on the same rules as the HTTP path.
	if guard, present, parseErr := workflow.ParseRequirementsGuard(node.Config); present {
		if parseErr != nil {
			return workflow.MutationResult{}, workflow.ErrMutationConflict
		}
		if command.Action == "complete" || command.Action == "approve" {
			if dependencies.PlanRequirements == nil {
				return workflow.MutationResult{}, workflow.ErrMutationUnavailable
			}
			refs, refsErr := dependencies.PlanRequirements.PlanUnitRequirementRefs(ctx, command.WorkspaceID, state.instance.SubjectID)
			if refsErr != nil {
				return workflow.MutationResult{}, refsErr
			}
			inputs, inputsErr := LoadGuardRequirementInputs(tx, command.WorkspaceID, refs)
			if inputsErr != nil {
				return workflow.MutationResult{}, inputsErr
			}
			if evaluation := guard.Evaluate(inputs); !evaluation.Satisfied() {
				return workflow.MutationResult{}, &workflow.GuardBlockedError{Evaluation: evaluation}
			}
		}
	}
	token, err := lockActiveMutationToken(tx, state.instance, nodeID, command.PrincipalID)
	if err != nil {
		return workflow.MutationResult{}, err
	}
	if node.Type == "timer" && !mutationTimerReady(token, node) {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	transitionAction := command.Action
	if strings.HasPrefix(node.Type, "ai_") {
		reference, err := latestWorkflowAIReference(tx, state.instance, node.ID)
		if err != nil {
			return workflow.MutationResult{}, err
		}
		prepared, err := dependencies.AI.PrepareWorkflowAIAction(ctx, workflow.AIActionRequest{
			WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, RunID: reference.RunID,
			Capability: node.Capability, Action: command.Action,
		})
		if err != nil || prepared.Path == "" {
			return workflow.MutationResult{}, workflow.ErrMutationConflict
		}
		transitionAction = prepared.Path
	}
	next := state.definition.Next(nodeID, transitionAction)
	if len(next) == 0 || node.Type == "condition" && len(next) != 1 {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	status := "completed"
	if command.Action == "reject" {
		status = "rejected"
	}
	now := time.Now().UTC()
	target := ""
	if len(next) == 1 {
		target = next[0]
	}
	outputValue := map[string]any{
		"actor_user_id": command.PrincipalID, "comment": command.Comment, "target_node_id": target,
	}
	if node.Type == "condition" {
		outputValue["decision"] = command.Action == "condition_yes"
	}
	output, _ := json.Marshal(outputValue)
	run := WorkflowNodeRun{
		WorkspaceID: state.instance.WorkspaceID, InstanceID: state.instance.ID, NodeID: node.ID,
		NodeType: node.Type, Status: status, OutputJSON: string(output), StartedAt: now, CompletedAt: &now,
	}
	if err := tx.Create(&run).Error; err != nil {
		return workflow.MutationResult{}, err
	}
	if node.Type == "approval" {
		actorID := command.PrincipalID
		approval := WorkflowApproval{
			WorkspaceID: state.instance.WorkspaceID, InstanceID: state.instance.ID, NodeRunID: run.ID,
			Status: status, ApproverUserID: &actorID, Comment: command.Comment, DecidedAt: &now, CreatedAt: now,
		}
		if err := tx.Create(&approval).Error; err != nil {
			return workflow.MutationResult{}, err
		}
	}
	changed := tx.Model(&WorkflowToken{}).Where(
		"id = ? AND workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
		token.ID, token.WorkspaceID, token.InstanceID, token.NodeID, "active",
	).Update("status", status)
	if changed.Error != nil {
		return workflow.MutationResult{}, changed.Error
	}
	if changed.RowsAffected != 1 {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	current = uniqueMutationNodes(append(removeMutationNode(current, nodeID), next...))
	current, err = advanceAutomaticMutationNodes(ctx, tx, &dependencies, state.instance, state.definition, current, command.NextAssignees)
	if err != nil {
		return workflow.MutationResult{}, err
	}
	for nodeID := range command.NextAssignees {
		if !containsMutationNode(current, nodeID) {
			return workflow.MutationResult{}, workflow.ErrMutationConflict
		}
	}
	nextPlansJSON := state.instance.NodePlansJSON
	if len(command.NextAssignees) > 0 {
		plans, decodeErr := decodeMutationNodePlans(state.instance.NodePlansJSON)
		if decodeErr != nil {
			return workflow.MutationResult{}, decodeErr
		}
		for nodeID, userID := range command.NextAssignees {
			plan := plans[nodeID]
			value := userID
			plan.AssigneeUserID = &value
			plans[nodeID] = plan
		}
		encodedPlans, encodeErr := json.Marshal(plans)
		if encodeErr != nil {
			return workflow.MutationResult{}, encodeErr
		}
		nextPlansJSON = string(encodedPlans)
	}
	encoded, _ := json.Marshal(current)
	instanceStatus := state.instance.Status
	var completedAt *time.Time
	if len(current) == 0 {
		instanceStatus, completedAt = "completed", &now
	}
	updated := tx.Model(&WorkflowInstance{}).Where(
		"id = ? AND workspace_id = ? AND status = ? AND current_node_ids = ? AND node_plans_json = ?",
		state.instance.ID, state.instance.WorkspaceID, state.instance.Status,
		state.instance.CurrentNodeIDs, state.instance.NodePlansJSON,
	).Updates(map[string]any{
		"status": instanceStatus, "current_node_ids": string(encoded), "node_plans_json": nextPlansJSON, "completed_at": completedAt,
	})
	if updated.Error != nil {
		return workflow.MutationResult{}, updated.Error
	}
	if updated.RowsAffected != 1 {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	if intakeLockPending {
		// The plan-unit row is the shared serialization point with requirement
		// creation: this FOR UPDATE write is what makes a lock that commits
		// first reject every later create, and vice versa.
		locked, lockErr := dependencies.IntakeLocks.MarkRequirementIntakeLocked(
			ctx, command.WorkspaceID, state.instance.SubjectID, command.PrincipalID, now,
		)
		if lockErr != nil {
			return workflow.MutationResult{}, lockErr
		}
		if locked {
			if effectErr := dependencies.IntakeEffects.RecordRequirementIntakeLocked(ctx, workflow.RequirementIntakeEffect{
				WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID,
				ProjectSetID: state.subject.ProjectSetID, PlanUnitID: state.instance.SubjectID,
				InstanceID: state.instance.ID, NodeID: node.ID, NodeName: node.Name, LockedAt: now,
			}); effectErr != nil {
				return workflow.MutationResult{}, effectErr
			}
		}
	}
	assignedNodeIDs := make([]string, 0, len(command.NextAssignees))
	for assignedNodeID := range command.NextAssignees {
		assignedNodeIDs = append(assignedNodeIDs, assignedNodeID)
	}
	sort.Strings(assignedNodeIDs)
	for _, assignedNodeID := range assignedNodeIDs {
		assignedNode, found := state.definition.Node(assignedNodeID)
		if !found {
			return workflow.MutationResult{}, workflow.ErrMutationConflict
		}
		if err := dependencies.Assignments.RecordSubjectAssignment(ctx, workflow.SubjectAssignment{
			WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID,
			ProjectSetID: state.subject.ProjectSetID, TeamID: state.subject.TeamID,
			InstanceID: state.instance.ID, SubjectID: state.subject.SubjectID,
			SubjectType: state.subject.SubjectType, SubjectTitle: state.subject.SubjectTitle,
			NodeID: assignedNode.ID, NodeName: assignedNode.Name,
			AssigneeUserID: command.NextAssignees[assignedNodeID],
		}); err != nil {
			return workflow.MutationResult{}, err
		}
	}
	if err := dependencies.Effects.RecordWorkflowAction(ctx, workflow.ActionEffect{
		WorkspaceID: state.instance.WorkspaceID, PrincipalID: command.PrincipalID,
		ProjectSetID: state.subject.ProjectSetID, InstanceID: state.instance.ID,
		NodeID: node.ID, Action: command.Action, Status: instanceStatus, ApplyKey: command.ApplyKey,
	}); err != nil {
		return workflow.MutationResult{}, err
	}
	return workflow.MutationResult{
		ID: state.instance.ID, WorkspaceID: state.instance.WorkspaceID, InstanceID: state.instance.ID,
		NodeID: node.ID, Status: instanceStatus, Action: command.Action, CurrentNodeIDs: current,
	}, nil
}

type workflowAIReference struct {
	RunID uint
	Job   WorkflowJob
}

func latestWorkflowAIReference(tx *gorm.DB, instance WorkflowInstance, nodeID string) (workflowAIReference, error) {
	var runs []WorkflowNodeRun
	if err := tx.Where("workspace_id = ? AND instance_id = ? AND node_id = ?", instance.WorkspaceID, instance.ID, nodeID).
		Order("id DESC").Find(&runs).Error; err != nil {
		return workflowAIReference{}, err
	}
	for _, run := range runs {
		var job WorkflowJob
		lookup := tx.Where(
			"workspace_id = ? AND instance_id = ? AND node_run_id = ? AND kind = ?",
			instance.WorkspaceID, instance.ID, run.ID, "ai_run",
		).Limit(1).Find(&job)
		if lookup.Error != nil {
			return workflowAIReference{}, lookup.Error
		}
		if lookup.RowsAffected == 0 {
			continue
		}
		var payload struct {
			RunID  uint   `json:"ai_run_id"`
			NodeID string `json:"node_id"`
		}
		if json.Unmarshal([]byte(job.PayloadJSON), &payload) != nil || payload.RunID == 0 || payload.NodeID != nodeID {
			return workflowAIReference{}, workflow.ErrMutationConflict
		}
		return workflowAIReference{RunID: payload.RunID, Job: job}, nil
	}
	return workflowAIReference{}, workflow.ErrMutationConflict
}

func applyWorkflowAIRecheck(
	ctx context.Context,
	tx *gorm.DB,
	dependencies MutationDependencies,
	command workflow.Mutation,
	state workflowMutationState,
) (workflow.MutationResult, error) {
	current := currentMutationNodeIDs(state.instance.CurrentNodeIDs)
	nodeID := strings.TrimSpace(command.NodeID)
	if nodeID == "" && len(current) == 1 {
		nodeID = current[0]
	}
	node, found := state.definition.Node(nodeID)
	if !found || !strings.HasPrefix(node.Type, "ai_") || !containsMutationNode(current, nodeID) || state.instance.Status != "running" {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	if _, err := lockActiveMutationToken(tx, state.instance, nodeID, command.PrincipalID); err != nil {
		return workflow.MutationResult{}, err
	}
	reference, err := latestWorkflowAIReference(tx, state.instance, nodeID)
	if err != nil {
		return workflow.MutationResult{}, err
	}
	prepared, err := dependencies.AI.PrepareWorkflowAIAction(ctx, workflow.AIActionRequest{
		WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, RunID: reference.RunID,
		Capability: node.Capability, Action: command.Action,
	})
	if err != nil || prepared.RunID == 0 || prepared.RunID == reference.RunID {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	now := time.Now().UTC()
	run := WorkflowNodeRun{
		WorkspaceID: state.instance.WorkspaceID, InstanceID: state.instance.ID, NodeID: node.ID,
		NodeType: node.Type, Status: "waiting", StartedAt: now,
	}
	if err := tx.Create(&run).Error; err != nil {
		return workflow.MutationResult{}, err
	}
	payload, _ := json.Marshal(map[string]any{"ai_run_id": prepared.RunID, "node_id": node.ID})
	job := WorkflowJob{
		WorkspaceID: state.instance.WorkspaceID, InstanceID: state.instance.ID, NodeRunID: run.ID,
		Kind: "ai_run", Status: "pending", PayloadJSON: string(payload),
		IdempotencyKey: "workflow-tool:" + command.ApplyKey, AvailableAt: now,
	}
	if err := tx.Create(&job).Error; err != nil {
		return workflow.MutationResult{}, err
	}
	if err := dependencies.Effects.RecordWorkflowAction(ctx, workflow.ActionEffect{
		WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: state.subject.ProjectSetID,
		InstanceID: state.instance.ID, NodeID: node.ID, Action: command.Action, Status: state.instance.Status, ApplyKey: command.ApplyKey,
	}); err != nil {
		return workflow.MutationResult{}, err
	}
	return workflow.MutationResult{
		ID: state.instance.ID, WorkspaceID: command.WorkspaceID, InstanceID: state.instance.ID,
		NodeID: node.ID, Status: state.instance.Status, Action: command.Action, CurrentNodeIDs: current,
	}, nil
}

func applyWorkflowSupplementDesign(
	ctx context.Context,
	tx *gorm.DB,
	dependencies MutationDependencies,
	command workflow.Mutation,
	state workflowMutationState,
) (workflow.MutationResult, error) {
	current := currentMutationNodeIDs(state.instance.CurrentNodeIDs)
	if state.instance.SubjectType != "requirement" || state.instance.Status != "running" || command.NodeID == "" ||
		command.DesignURL == "" || !containsMutationNode(current, command.NodeID) {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	node, found := state.definition.Node(command.NodeID)
	if !found || !workflowNodeRequiresDesign(node) {
		return workflow.MutationResult{}, workflow.ErrMutationConflict
	}
	if _, err := lockActiveMutationToken(tx, state.instance, node.ID, command.PrincipalID); err != nil {
		return workflow.MutationResult{}, err
	}
	if err := dependencies.Designs.AttachWorkflowDesign(ctx, command.WorkspaceID, state.instance.SubjectID, command.DesignURL); err != nil {
		return workflow.MutationResult{}, err
	}
	if err := dependencies.Effects.RecordWorkflowAction(ctx, workflow.ActionEffect{
		WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: state.subject.ProjectSetID,
		InstanceID: state.instance.ID, NodeID: node.ID, Action: command.Action, Status: state.instance.Status, ApplyKey: command.ApplyKey,
	}); err != nil {
		return workflow.MutationResult{}, err
	}
	return workflow.MutationResult{
		ID: state.instance.ID, WorkspaceID: command.WorkspaceID, InstanceID: state.instance.ID, NodeID: node.ID,
		Status: state.instance.Status, Action: command.Action, CurrentNodeIDs: current,
	}, nil
}

func workflowNodeRequiresDesign(node workflow.DefinitionNode) bool {
	required, _ := node.Config["require_design_image"].(bool)
	return required && node.Type == "human_task"
}

func lockActiveMutationToken(tx *gorm.DB, instance WorkflowInstance, nodeID string, principalID uint) (WorkflowToken, error) {
	var token WorkflowToken
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
		instance.WorkspaceID, instance.ID, nodeID, "active",
	).Order("id DESC").First(&token).Error; err != nil {
		return WorkflowToken{}, normalizeWorkflowMutationError(err)
	}
	if token.AssigneeUserID != nil && *token.AssigneeUserID != principalID {
		return WorkflowToken{}, workflow.ErrMutationConflict
	}
	return token, nil
}

func advanceAutomaticMutationNodes(
	ctx context.Context,
	tx *gorm.DB,
	dependencies *MutationDependencies,
	instance WorkflowInstance,
	definition workflow.Definition,
	nodeIDs []string,
	nextAssignees map[string]uint,
) ([]string, error) {
	now := time.Now().UTC()
	queue, result := append([]string(nil), nodeIDs...), make([]string, 0, len(nodeIDs))
	attempts := 0
	for len(queue) > 0 {
		attempts++
		if attempts > len(definition.Nodes)*3 {
			return nil, workflow.ErrMutationConflict
		}
		nodeID := queue[0]
		queue = queue[1:]
		node, found := definition.Node(nodeID)
		if !found {
			return nil, workflow.ErrMutationConflict
		}
		switch node.Type {
		case "end":
			if err := recordAutomaticMutationNode(tx, instance, node, now); err != nil {
				return nil, err
			}
			continue
		}
		// Stage-driven plan-unit nodes (需求阶段驱动 + 全自动推进) complete on
		// their own once every scoped requirement reaches the stage.
		if autoGuard, isAuto := mutationAutoAdvanceGuard(instance, node); isAuto {
			ready, err := mutationGuardReady(ctx, tx, dependencies, instance, autoGuard)
			if err != nil {
				return nil, err
			}
			if !ready {
				if err := ensureMutationToken(tx, instance, node.ID, "waiting", nil); err != nil {
					return nil, err
				}
				result = append(result, node.ID)
				continue
			}
			if err := recordRequirementGateMutationNode(tx, instance, node, now); err != nil {
				return nil, err
			}
			next := definition.Next(node.ID, "complete")
			if len(next) == 0 {
				return nil, workflow.ErrMutationConflict
			}
			queue = append(queue, next...)
			continue
		}
		switch node.Type {
		case "requirement_gate":
			ready, err := mutationRequirementGateReady(ctx, tx, dependencies, instance, node)
			if err != nil {
				return nil, err
			}
			if !ready {
				if err := ensureMutationToken(tx, instance, node.ID, "waiting", nil); err != nil {
					return nil, err
				}
				result = append(result, node.ID)
				continue
			}
			if err := recordRequirementGateMutationNode(tx, instance, node, now); err != nil {
				return nil, err
			}
			next := definition.Next(node.ID, "complete")
			if len(next) == 0 {
				return nil, workflow.ErrMutationConflict
			}
			queue = append(queue, next...)
			continue
		case "start", "parallel_split":
			if err := recordAutomaticMutationNode(tx, instance, node, now); err != nil {
				return nil, err
			}
			next := definition.Next(node.ID, "complete")
			if len(next) == 0 {
				return nil, workflow.ErrMutationConflict
			}
			queue = append(queue, next...)
			continue
		case "parallel_join":
			ready, err := mutationParallelJoinReady(tx, definition, instance, node.ID)
			if err != nil {
				return nil, err
			}
			if !ready {
				if err := ensureMutationToken(tx, instance, node.ID, "waiting", nil); err != nil {
					return nil, err
				}
				result = append(result, node.ID)
				continue
			}
			if err := recordAutomaticMutationNode(tx, instance, node, now); err != nil {
				return nil, err
			}
			next := definition.Next(node.ID, "complete")
			if len(next) == 0 {
				return nil, workflow.ErrMutationConflict
			}
			queue = append(queue, next...)
			continue
		}
		assigneeID := nextAssignees[node.ID]
		if assigneeID == 0 {
			plans, err := decodeMutationNodePlans(instance.NodePlansJSON)
			if err != nil {
				return nil, err
			}
			if plan := plans[node.ID]; plan.AssigneeUserID != nil {
				assigneeID = *plan.AssigneeUserID
			}
		}
		if workflow.NeedsAssignee(node) && assigneeID == 0 {
			return nil, workflow.ErrMutationConflict
		}
		var assignee *uint
		if assigneeID > 0 {
			value := assigneeID
			assignee = &value
		}
		if err := ensureMutationToken(tx, instance, node.ID, "active", assignee); err != nil {
			return nil, err
		}
		result = append(result, node.ID)
	}
	return uniqueMutationNodes(result), nil
}

func mutationTimerReady(token WorkflowToken, node workflow.DefinitionNode) bool {
	delay, ok := node.Config["delay_seconds"].(float64)
	return !ok || delay <= 0 || !time.Now().UTC().Before(token.CreatedAt.Add(time.Duration(delay)*time.Second))
}

func ensureMutationToken(tx *gorm.DB, instance WorkflowInstance, nodeID, status string, assignee *uint) error {
	var existing WorkflowToken
	lookup := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ? AND status IN ?",
		instance.WorkspaceID, instance.ID, nodeID, []string{"active", "waiting"},
	).Order("id DESC").Limit(1).Find(&existing)
	if lookup.Error != nil {
		return lookup.Error
	}
	if lookup.RowsAffected == 1 {
		updates := map[string]any{}
		if existing.Status != status {
			updates["status"] = status
		}
		if assignee != nil && (existing.AssigneeUserID == nil || *existing.AssigneeUserID != *assignee) {
			updates["assignee_user_id"] = *assignee
		}
		if len(updates) == 0 {
			return nil
		}
		changed := tx.Model(&WorkflowToken{}).Where("id = ? AND status = ?", existing.ID, existing.Status).Updates(updates)
		if changed.Error != nil {
			return changed.Error
		}
		if changed.RowsAffected != 1 {
			return workflow.ErrMutationConflict
		}
		return nil
	}
	return tx.Create(&WorkflowToken{WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeID: nodeID, Status: status, AssigneeUserID: assignee}).Error
}

func recordAutomaticMutationNode(tx *gorm.DB, instance WorkflowInstance, node workflow.DefinitionNode, now time.Time) error {
	if err := tx.Create(&WorkflowNodeRun{
		WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeID: node.ID,
		NodeType: node.Type, Status: "completed", StartedAt: now, CompletedAt: &now,
	}).Error; err != nil {
		return err
	}
	return tx.Model(&WorkflowToken{}).Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ? AND status IN ?",
		instance.WorkspaceID, instance.ID, node.ID, []string{"active", "waiting"},
	).Update("status", "completed").Error
}

// mutationRequirementGateReady evaluates the gate guard inside the mutation
// transaction with the same rules as the HTTP runtime path. A missing
// PlanRequirements dependency degrades to "not ready": the gate keeps waiting
// instead of failing the whole mutation.
func mutationRequirementGateReady(
	ctx context.Context,
	tx *gorm.DB,
	dependencies *MutationDependencies,
	instance WorkflowInstance,
	node workflow.DefinitionNode,
) (bool, error) {
	guard, isAuto := mutationAutoAdvanceGuard(instance, node)
	if !isAuto {
		return false, workflow.ErrMutationConflict
	}
	return mutationGuardReady(ctx, tx, dependencies, instance, guard)
}

// mutationAutoAdvanceGuard returns the guard that drives a node's automatic
// completion on the Tool channel: requirement_gate nodes use their configured
// guard; plan-unit human task/form nodes with a requirement stage (explicit
// config or node-name inference) use the stage-driven guard 需求阶段驱动.
func mutationAutoAdvanceGuard(
	instance WorkflowInstance,
	node workflow.DefinitionNode,
) (*workflow.RequirementsGuard, bool) {
	if node.Type == "requirement_gate" {
		guard, present, err := workflow.ParseRequirementsGuard(node.Config)
		if err != nil || !present {
			return nil, false
		}
		return guard, true
	}
	if instance.SubjectType == "plan_unit" {
		if guard, ok, _ := workflow.NodeStageGuard(node); ok {
			return guard, true
		}
	}
	return nil, false
}

// mutationGuardReady evaluates the auto-advance guard inside the mutation
// transaction. A missing PlanRequirements dependency degrades to "not ready":
// stage-driven nodes keep waiting instead of failing the mutation.
func mutationGuardReady(
	ctx context.Context,
	tx *gorm.DB,
	dependencies *MutationDependencies,
	instance WorkflowInstance,
	guard *workflow.RequirementsGuard,
) (bool, error) {
	if dependencies == nil || dependencies.PlanRequirements == nil {
		return false, nil
	}
	refs, err := dependencies.PlanRequirements.PlanUnitRequirementRefs(ctx, instance.WorkspaceID, instance.SubjectID)
	if err != nil {
		return false, err
	}
	inputs, err := LoadGuardRequirementInputs(tx, instance.WorkspaceID, refs)
	if err != nil {
		return false, err
	}
	return guard.Evaluate(inputs).Satisfied(), nil
}

// recordRequirementGateMutationNode records the system-actor gate pass with an
// explicit marker so the transition history distinguishes automatic evidence
// decisions from human completions.
func recordRequirementGateMutationNode(tx *gorm.DB, instance WorkflowInstance, node workflow.DefinitionNode, now time.Time) error {
	if err := tx.Create(&WorkflowNodeRun{
		WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeID: node.ID,
		NodeType: node.Type, Status: "completed", OutputJSON: `{"system_actor":true}`,
		StartedAt: now, CompletedAt: &now,
	}).Error; err != nil {
		return err
	}
	return tx.Model(&WorkflowToken{}).Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ? AND status IN ?",
		instance.WorkspaceID, instance.ID, node.ID, []string{"active", "waiting"},
	).Update("status", "completed").Error
}

func mutationParallelJoinReady(tx *gorm.DB, definition workflow.Definition, instance WorkflowInstance, joinID string) (bool, error) {
	incoming := make([]workflow.DefinitionEdge, 0)
	for _, edge := range definition.Edges {
		if edge.Target == joinID {
			incoming = append(incoming, edge)
		}
	}
	if len(incoming) < 2 {
		return false, workflow.ErrMutationConflict
	}
	for _, edge := range incoming {
		var run WorkflowNodeRun
		lookup := tx.Where(
			"workspace_id = ? AND instance_id = ? AND node_id = ? AND status IN ?",
			instance.WorkspaceID, instance.ID, edge.Source, []string{"completed", "rejected"},
		).Order("id DESC").Limit(1).Find(&run)
		if lookup.Error != nil {
			return false, lookup.Error
		}
		if lookup.RowsAffected == 0 {
			return false, nil
		}
		var output struct {
			TargetNodeID string `json:"target_node_id"`
		}
		if json.Unmarshal([]byte(run.OutputJSON), &output) == nil && output.TargetNodeID != "" && output.TargetNodeID != joinID {
			return false, nil
		}
	}
	return true, nil
}
