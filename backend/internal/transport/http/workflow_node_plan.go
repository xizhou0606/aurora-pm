package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errWorkflowNodePlansChanged = errors.New("workflow node plans changed")
var errWorkflowNodePlanTokenChanged = errors.New("workflow node assignee changed")

type workflowNodePlan struct {
	PlannedStartAt *time.Time `json:"planned_start_at,omitempty"`
	PlannedEndAt   *time.Time `json:"planned_end_at,omitempty"`
	AssigneeUserID *uint      `json:"assignee_user_id,omitempty"`
}

type workflowNodePlanUpdate struct {
	nodeID                 string
	plannedStartAt         *time.Time
	plannedEndAt           *time.Time
	expectedPlannedStartAt *time.Time
	expectedPlannedEndAt   *time.Time
	startSet               bool
	endSet                 bool
	assigneeSet            bool
	assigneeUserID         uint
}

func workflowNodePlans(instance workflowgorm.WorkflowInstance) map[string]workflowNodePlan {
	plans, err := decodeWorkflowNodePlans(instance)
	if err != nil {
		return map[string]workflowNodePlan{}
	}
	return plans
}

func decodeWorkflowNodePlans(instance workflowgorm.WorkflowInstance) (map[string]workflowNodePlan, error) {
	plans := map[string]workflowNodePlan{}
	if strings.TrimSpace(instance.NodePlansJSON) == "" {
		return nil, errors.New("workflow node plans are unavailable")
	}
	if err := json.Unmarshal([]byte(instance.NodePlansJSON), &plans); err != nil {
		return nil, fmt.Errorf("decode workflow node plans: %w", err)
	}
	if plans == nil {
		return nil, errors.New("workflow node plans are invalid")
	}
	for nodeID, plan := range plans {
		if strings.TrimSpace(nodeID) == "" || (plan.AssigneeUserID != nil && *plan.AssigneeUserID == 0) {
			return nil, errors.New("workflow node plans are invalid")
		}
	}
	return plans, nil
}

func workflowNodePlanAssignee(instance workflowgorm.WorkflowInstance, nodeID string) (*uint, bool, error) {
	plans, err := decodeWorkflowNodePlans(instance)
	if err != nil {
		return nil, false, err
	}
	plan, exists := plans[nodeID]
	if !exists || plan.AssigneeUserID == nil || *plan.AssigneeUserID == 0 {
		return nil, false, nil
	}
	value := *plan.AssigneeUserID
	return &value, true, nil
}

func (s *Server) patchWorkflowNodePlans(c *gin.Context) {
	instance, okRow := s.workflowInstanceByID(c)
	if !okRow {
		return
	}
	if !validWorkflowSubject(instance.SubjectType) {
		fail(c, http.StatusConflict, "workflow subject type is invalid")
		return
	}
	var body struct {
		ConfirmSubjectSchedule bool `json:"confirm_subject_schedule"`
		Nodes                  []struct {
			NodeID                 string                `json:"node_id"`
			PlannedStartAt         optionalValue[string] `json:"planned_start_at"`
			PlannedEndAt           optionalValue[string] `json:"planned_end_at"`
			ExpectedPlannedStartAt optionalValue[string] `json:"expected_planned_start_at"`
			ExpectedPlannedEndAt   optionalValue[string] `json:"expected_planned_end_at"`
			AssigneeUserID         optionalValue[uint]   `json:"assignee_user_id"`
		} `json:"nodes"`
	}
	if c.ShouldBindJSON(&body) != nil || body.Nodes == nil || len(body.Nodes) == 0 && !body.ConfirmSubjectSchedule {
		bad(c, "nodes required")
		return
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
	var runs []workflowgorm.WorkflowNodeRun
	if s.db.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).Order("id").Find(&runs).Error != nil {
		dbFail(c)
		return
	}
	selectedNodeIDs, _, err := workflowScheduleDisplayTopology(s.db, definition, instance, runs)
	if err != nil {
		fail(c, http.StatusConflict, "workflow selected path is unavailable")
		return
	}
	plans, err := decodeWorkflowNodePlans(instance)
	if err != nil {
		fail(c, http.StatusConflict, "workflow node plans are unavailable")
		return
	}
	prepared := make([]workflowNodePlanUpdate, 0, len(body.Nodes))
	seen, assigneeIDs := map[string]bool{}, make([]uint, 0, len(body.Nodes))
	for _, input := range body.Nodes {
		nodeID := strings.TrimSpace(input.NodeID)
		node, found := workflowNodeByID(definition, nodeID)
		if nodeID == "" || !found || !workflowNodeNeedsAssignee(node) || seen[nodeID] {
			bad(c, "workflow node plan is invalid")
			return
		}
		if !input.PlannedStartAt.Set && !input.PlannedEndAt.Set && !input.AssigneeUserID.Set {
			bad(c, "workflow node plan update requires at least one field")
			return
		}
		expectations, expectationErr := prepareScheduleDateExpectations(
			scheduleDateCASInput{Field: "planned_start_at", ActualSet: input.PlannedStartAt.Set, ExpectedRaw: input.ExpectedPlannedStartAt},
			scheduleDateCASInput{Field: "planned_end_at", ActualSet: input.PlannedEndAt.Set, ExpectedRaw: input.ExpectedPlannedEndAt},
		)
		if expectationErr != nil {
			bad(c, expectationErr.Error())
			return
		}
		if !selectedNodeIDs[nodeID] {
			fail(c, http.StatusConflict, "workflow node is outside the selected path")
			return
		}
		update := workflowNodePlanUpdate{nodeID: nodeID, startSet: input.PlannedStartAt.Set, endSet: input.PlannedEndAt.Set}
		for _, expectation := range expectations {
			switch expectation.Field {
			case "planned_start_at":
				update.expectedPlannedStartAt = expectation.Expected
			case "planned_end_at":
				update.expectedPlannedEndAt = expectation.Expected
			}
		}
		if input.PlannedStartAt.Set {
			update.plannedStartAt, err = parseOptionalTime(input.PlannedStartAt.Value)
			if err != nil {
				bad(c, err.Error())
				return
			}
		}
		if input.PlannedEndAt.Set {
			update.plannedEndAt, err = parseOptionalTime(input.PlannedEndAt.Value)
			if err != nil {
				bad(c, err.Error())
				return
			}
		}
		if input.AssigneeUserID.Set {
			if !workflowNodeTakesAssignee(node) {
				bad(c, fmt.Sprintf("workflow node %s cannot take an assignee", nodeID))
				return
			}
			if input.AssigneeUserID.Value == nil || *input.AssigneeUserID.Value == 0 {
				bad(c, "assignee_user_id must be a positive user ID")
				return
			}
			update.assigneeSet, update.assigneeUserID = true, *input.AssigneeUserID.Value
			assigneeIDs = append(assigneeIDs, update.assigneeUserID)
		}
		seen[nodeID] = true
		prepared = append(prepared, update)
	}
	if err := s.validateActivePMUserReferences(c, assigneeIDs); err != nil {
		respondPMUserRegistryValidationError(c, err)
		return
	}
	scheduleTarget, err := resolveWorkflowScheduleSubjectTarget(s.db, instance)
	if err != nil {
		fail(c, http.StatusConflict, "workflow schedule subject changed; reload and retry")
		return
	}

	preflightPlans := plans
	plans, plannedChanges, assigneeChanges := applyWorkflowNodePlanUpdates(preflightPlans, prepared)
	currentIDs := currentWorkflowNodes(instance.CurrentNodeIDs)
	encoded, err := json.Marshal(plans)
	if err != nil {
		dbFail(c)
		return
	}
	expectedPlansJSON, expectedCurrent, expectedStatus := instance.NodePlansJSON, instance.CurrentNodeIDs, instance.Status
	dateRequested := workflowNodePlanDateRequested(prepared)
	if string(encoded) == expectedPlansJSON && !body.ConfirmSubjectSchedule && !dateRequested {
		s.respondWorkflowNodePlans(c, instance, definition, version)
		return
	}
	activeTokens := make(map[string]workflowgorm.WorkflowToken, len(assigneeChanges))
	for nodeID := range assigneeChanges {
		if !containsString(currentIDs, nodeID) {
			continue
		}
		var token workflowgorm.WorkflowToken
		lookup := s.db.Where(
			"workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
			instance.WorkspaceID, instance.ID, nodeID, "active",
		).Order("id DESC").First(&token)
		if errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			fail(c, http.StatusConflict, "workflow node is no longer active")
			return
		}
		if lookup.Error != nil {
			dbFail(c)
			return
		}
		activeTokens[nodeID] = token
	}
	err = workflowScheduleTransaction(s.db, func(tx *gorm.DB) error {
		// Workflow actions claim token before updating the instance. Keep the
		// same token/instance order. The parent project is locked first so a
		// concurrent project-boundary change cannot leave MySQL on an old RR
		// snapshot while this request infers a PlanUnit boundary.
		scheduleProject, err := lockWorkflowScheduleProjectTx(tx, instance, scheduleTarget)
		if err != nil {
			return err
		}
		for _, nodeID := range sortedWorkflowNodePlanKeys(activeTokens) {
			token := activeTokens[nodeID]
			newAssignee := assigneeChanges[nodeID]
			query := tx.Model(&workflowgorm.WorkflowToken{}).Where(
				"id = ? AND workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
				token.ID, instance.WorkspaceID, instance.ID, nodeID, "active",
			)
			if token.AssigneeUserID == nil {
				query = query.Where("assignee_user_id IS NULL")
			} else {
				query = query.Where("assignee_user_id = ?", *token.AssigneeUserID)
			}
			changed := query.Update("assignee_user_id", newAssignee)
			if changed.Error != nil {
				return changed.Error
			}
			if changed.RowsAffected != 1 {
				return errWorkflowNodePlanTokenChanged
			}
		}
		var locked workflowgorm.WorkflowInstance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND workspace_id = ?", instance.ID, instance.WorkspaceID).
			First(&locked).Error; err != nil {
			return err
		}
		if locked.CurrentNodeIDs != expectedCurrent || locked.Status != expectedStatus {
			return errWorkflowNodePlansChanged
		}
		lockedPlans, err := decodeWorkflowNodePlans(locked)
		if err != nil {
			return errWorkflowNodePlansChanged
		}
		if conflicts := workflowNodePlanDateStaleConflicts(prepared, lockedPlans, instance); len(conflicts) > 0 {
			return workflowScheduleStaleError(workflowScheduleScopeForBoundary(workflowScheduleBoundary{}, scheduleTarget.HasPlanUnit), conflicts)
		}
		if workflowNodePlanAssigneeFieldsChanged(preflightPlans, lockedPlans, prepared) {
			return errWorkflowNodePlansChanged
		}
		plans, plannedChanges, assigneeChanges = applyWorkflowNodePlanUpdates(lockedPlans, prepared)
		encoded, err = json.Marshal(plans)
		if err != nil {
			return err
		}
		if string(encoded) != locked.NodePlansJSON {
			if err := tx.Model(&workflowgorm.WorkflowInstance{}).
				Where("id = ? AND workspace_id = ?", locked.ID, locked.WorkspaceID).
				Update("node_plans_json", string(encoded)).Error; err != nil {
				return fmt.Errorf("update workflow node plans: %w", err)
			}
		}
		instance = locked
		instance.NodePlansJSON = string(encoded)
		if err := s.applyWorkflowNodePlanScheduleTx(
			tx, c, instance, definition, plans, seen, len(plannedChanges) > 0,
			body.ConfirmSubjectSchedule, scheduleTarget, scheduleProject,
		); err != nil {
			return err
		}
		allNodeIDs := sortedWorkflowNodePlanKeys(seen)
		projectID := s.workflowSubjectProjectID(tx, instance.WorkspaceID, instance.SubjectType, instance.SubjectID)
		for nodeID, userID := range assigneeChanges {
			if err := s.recordWorkflowNodeAssignedNotification(tx, c, instance, nodeID, userID); err != nil {
				return err
			}
		}
		return s.recordMutation(tx, c, projectID, "workflow_instance", instance.ID, "node_plans_updated", "", gin.H{
			"node_ids": allNodeIDs, "planned_node_ids": sortedWorkflowNodePlanKeys(plannedChanges),
			"assignee_node_ids": sortedWorkflowNodePlanKeys(assigneeChanges),
		})
	})
	if err != nil {
		if respondWorkflowScheduleError(c, err) {
			return
		}
		if errors.Is(err, errWorkflowNodePlansChanged) || errors.Is(err, errWorkflowNodePlanTokenChanged) || workflowNodePlanDatabaseConflict(err) {
			fail(c, http.StatusConflict, "workflow node plans changed; reload and retry")
			return
		}
		dbFail(c)
		return
	}
	instance.NodePlansJSON = string(encoded)
	s.respondWorkflowNodePlans(c, instance, definition, version)
}

func sameWorkflowNodePlanTime(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func applyWorkflowNodePlanUpdates(
	current map[string]workflowNodePlan,
	updates []workflowNodePlanUpdate,
) (map[string]workflowNodePlan, map[string]bool, map[string]uint) {
	plans := make(map[string]workflowNodePlan, len(current))
	for nodeID, plan := range current {
		plans[nodeID] = plan
	}
	plannedChanges := map[string]bool{}
	assigneeChanges := map[string]uint{}
	for _, update := range updates {
		plan := plans[update.nodeID]
		if update.startSet {
			if !sameWorkflowNodePlanTime(plan.PlannedStartAt, update.plannedStartAt) {
				plannedChanges[update.nodeID] = true
			}
			plan.PlannedStartAt = update.plannedStartAt
		}
		if update.endSet {
			if !sameWorkflowNodePlanTime(plan.PlannedEndAt, update.plannedEndAt) {
				plannedChanges[update.nodeID] = true
			}
			plan.PlannedEndAt = update.plannedEndAt
		}
		if update.assigneeSet && (plan.AssigneeUserID == nil || *plan.AssigneeUserID != update.assigneeUserID) {
			value := update.assigneeUserID
			plan.AssigneeUserID = &value
			assigneeChanges[update.nodeID] = value
		}
		if plan.PlannedStartAt == nil && plan.PlannedEndAt == nil && plan.AssigneeUserID == nil {
			delete(plans, update.nodeID)
		} else {
			plans[update.nodeID] = plan
		}
	}
	return plans, plannedChanges, assigneeChanges
}

func workflowNodePlanDateRequested(updates []workflowNodePlanUpdate) bool {
	for _, update := range updates {
		if update.startSet || update.endSet {
			return true
		}
	}
	return false
}

func workflowNodePlanDateStaleConflicts(
	updates []workflowNodePlanUpdate,
	plans map[string]workflowNodePlan,
	instance workflowgorm.WorkflowInstance,
) []workflowScheduleConflictItem {
	conflicts := make([]workflowScheduleConflictItem, 0)
	for _, update := range updates {
		plan := plans[update.nodeID]
		expectations := make([]scheduleDateExpectation, 0, 2)
		if update.startSet {
			expectations = append(expectations, scheduleDateExpectation{Field: "planned_start_at", Expected: update.expectedPlannedStartAt})
		}
		if update.endSet {
			expectations = append(expectations, scheduleDateExpectation{Field: "planned_end_at", Expected: update.expectedPlannedEndAt})
		}
		conflicts = append(conflicts, scheduleDateStaleConflicts(expectations, map[string]*time.Time{
			"planned_start_at": plan.PlannedStartAt,
			"planned_end_at":   plan.PlannedEndAt,
		}, instance.SubjectType, instance.SubjectID, update.nodeID)...)
	}
	return conflicts
}

func workflowNodePlanAssigneeFieldsChanged(
	preflight map[string]workflowNodePlan,
	locked map[string]workflowNodePlan,
	updates []workflowNodePlanUpdate,
) bool {
	for _, update := range updates {
		if !update.assigneeSet {
			continue
		}
		left, right := preflight[update.nodeID].AssigneeUserID, locked[update.nodeID].AssigneeUserID
		if left == nil || right == nil {
			if left != nil || right != nil {
				return true
			}
			continue
		}
		if *left != *right {
			return true
		}
	}
	return false
}
