package httpapi

import (
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

type workflowScheduleBoundary struct {
	PlannedStartAt *time.Time `json:"planned_start_at"`
	PlannedEndAt   *time.Time `json:"planned_end_at"`
}

type workflowScheduleConflictItem struct {
	Code          string     `json:"code"`
	SubjectType   string     `json:"subject_type,omitempty"`
	SubjectID     uint       `json:"subject_id,omitempty"`
	NodeID        string     `json:"node_id,omitempty"`
	RelatedNodeID string     `json:"related_node_id,omitempty"`
	Field         string     `json:"field,omitempty"`
	Boundary      *time.Time `json:"boundary,omitempty"`
	Value         *time.Time `json:"value,omitempty"`
}

type workflowSubjectSchedule struct {
	State                  string                         `json:"state"`
	Subject                workflowScheduleBoundary       `json:"subject"`
	Constraints            *workflowScheduleBoundary      `json:"constraints,omitempty"`
	Suggested              *workflowScheduleBoundary      `json:"suggested"`
	CanConfirm             bool                           `json:"can_confirm"`
	ActualDatesIndependent bool                           `json:"actual_dates_independent"`
	Conflicts              []workflowScheduleConflictItem `json:"conflicts"`
}

type workflowScheduleRequestError struct {
	Code     string
	Schedule workflowSubjectSchedule
}

func (err *workflowScheduleRequestError) Error() string { return err.Code }

func respondWorkflowScheduleError(c *gin.Context, err error) bool {
	var scheduleErr *workflowScheduleRequestError
	if !errors.As(err, &scheduleErr) {
		return false
	}
	message := "workflow schedule conflicts with its subject"
	if scheduleErr.Code == apiErrorWorkflowScheduleConfirmation {
		message = "workflow schedule confirmation required"
	} else if scheduleErr.Code == apiErrorScheduleSubjectStale {
		message = "schedule changed; reload and retry"
	}
	c.AbortWithStatusJSON(http.StatusConflict, gin.H{
		"code": scheduleErr.Code, "error": message, "schedule": scheduleErr.Schedule,
	})
	return true
}

func workflowScheduleForParts(
	definition workflowDefinition,
	selectedNodeIDs map[string]bool,
	selectedEdgeIDs []string,
	plans map[string]workflowNodePlan,
	scope workflowScheduleScope,
) workflowSubjectSchedule {
	businessIDs := workflowScheduleBusinessNodeIDs(definition, selectedNodeIDs)
	schedule := workflowSubjectSchedule{
		State: "unscheduled", Subject: scope.Subject,
		Constraints: workflowScheduleBoundaryPointer(scope.Constraints), ActualDatesIndependent: true,
		Conflicts: make([]workflowScheduleConflictItem, 0),
	}
	allNodesComplete, anyNodeDate := true, false
	var minStart, maxEnd *time.Time
	for _, nodeID := range businessIDs {
		plan := plans[nodeID]
		if plan.PlannedStartAt != nil || plan.PlannedEndAt != nil {
			anyNodeDate = true
		}
		if plan.PlannedStartAt == nil || plan.PlannedEndAt == nil {
			allNodesComplete = false
		}
		if plan.PlannedStartAt != nil && (minStart == nil || plan.PlannedStartAt.Before(*minStart)) {
			value := *plan.PlannedStartAt
			minStart = &value
		}
		if plan.PlannedEndAt != nil && (maxEnd == nil || plan.PlannedEndAt.After(*maxEnd)) {
			value := *plan.PlannedEndAt
			maxEnd = &value
		}
		workflowScheduleAppendNodeConflicts(&schedule, nodeID, plan, scope)
	}
	if allNodesComplete && minStart != nil && maxEnd != nil {
		schedule.Suggested = &workflowScheduleBoundary{PlannedStartAt: minStart, PlannedEndAt: maxEnd}
	}
	if scope.Subject.PlannedStartAt != nil && scope.Subject.PlannedEndAt != nil && scope.Subject.PlannedStartAt.After(*scope.Subject.PlannedEndAt) {
		schedule.Conflicts = append(schedule.Conflicts, workflowScheduleConflictItem{
			Code: scheduleConflictSubjectStartAfterEnd, Field: "planned_start_at",
			Boundary: scope.Subject.PlannedEndAt, Value: scope.Subject.PlannedStartAt,
		})
	}
	workflowScheduleAppendConstraintOrderConflict(&schedule, scope)
	schedule.Conflicts = append(schedule.Conflicts,
		workflowScheduleSerialConflicts(definition, selectedNodeIDs, selectedEdgeIDs, plans)...)
	schedule.CanConfirm = scope.HasPlanUnit && schedule.Suggested != nil &&
		(scope.Subject.PlannedStartAt == nil || scope.Subject.PlannedEndAt == nil) && len(schedule.Conflicts) == 0

	if !scope.HasPlanUnit {
		fatalConflict := len(schedule.Conflicts) > 0
		if anyNodeDate {
			schedule.Conflicts = append(schedule.Conflicts, workflowScheduleConflictItem{Code: scheduleConflictSubjectPlanUnitRequired})
		}
		if fatalConflict {
			schedule.State = "conflict"
		} else {
			schedule.State = "unscheduled"
		}
		return schedule
	}
	if len(schedule.Conflicts) > 0 {
		schedule.State = "conflict"
		return schedule
	}
	if scope.Subject.PlannedStartAt != nil && scope.Subject.PlannedEndAt != nil && allNodesComplete {
		schedule.State = "scheduled"
		return schedule
	}
	if scope.Subject.PlannedStartAt != nil || scope.Subject.PlannedEndAt != nil || anyNodeDate {
		schedule.State = "partial"
	}
	return schedule
}

func workflowScheduleBusinessNodeIDs(definition workflowDefinition, selected map[string]bool) []string {
	result := make([]string, 0)
	for _, node := range definition.Nodes {
		if selected[node.ID] && workflowNodeNeedsAssignee(node) {
			result = append(result, node.ID)
		}
	}
	return result
}

func workflowScheduleAppendNodeConflicts(
	schedule *workflowSubjectSchedule,
	nodeID string,
	plan workflowNodePlan,
	scope workflowScheduleScope,
) {
	if plan.PlannedStartAt != nil && plan.PlannedEndAt != nil && plan.PlannedStartAt.After(*plan.PlannedEndAt) {
		schedule.Conflicts = append(schedule.Conflicts, workflowScheduleConflictItem{
			Code: scheduleConflictNodeStartAfterEnd, NodeID: nodeID, Field: "planned_start_at",
			Boundary: plan.PlannedEndAt, Value: plan.PlannedStartAt,
		})
	}
	for _, value := range []struct {
		field string
		date  *time.Time
	}{{"planned_start_at", plan.PlannedStartAt}, {"planned_end_at", plan.PlannedEndAt}} {
		if value.date == nil {
			continue
		}
		if scope.Constraints.PlannedStartAt != nil && value.date.Before(*scope.Constraints.PlannedStartAt) {
			schedule.Conflicts = append(schedule.Conflicts, workflowScheduleConflictItem{
				Code: scheduleConflictNodeBeforeSubject, NodeID: nodeID, Field: value.field,
				Boundary: scope.Constraints.PlannedStartAt, Value: value.date,
				SubjectType: scope.StartConstraintType, SubjectID: scope.StartConstraintID,
			})
		}
		if scope.Constraints.PlannedEndAt != nil && value.date.After(*scope.Constraints.PlannedEndAt) {
			schedule.Conflicts = append(schedule.Conflicts, workflowScheduleConflictItem{
				Code: scheduleConflictNodeAfterSubject, NodeID: nodeID, Field: value.field,
				Boundary: scope.Constraints.PlannedEndAt, Value: value.date,
				SubjectType: scope.EndConstraintType, SubjectID: scope.EndConstraintID,
			})
		}
	}
}

func workflowScheduleSerialConflicts(
	definition workflowDefinition,
	selectedNodeIDs map[string]bool,
	selectedEdgeIDs []string,
	plans map[string]workflowNodePlan,
) []workflowScheduleConflictItem {
	business := map[string]bool{}
	for _, nodeID := range workflowScheduleBusinessNodeIDs(definition, selectedNodeIDs) {
		business[nodeID] = true
	}
	edges := map[string]bool{}
	for _, edgeID := range selectedEdgeIDs {
		edges[edgeID] = true
	}
	outgoing := map[string][]string{}
	for _, edge := range definition.Edges {
		if !edges[edge.ID] || edge.Loop {
			continue
		}
		outgoing[edge.Source] = append(outgoing[edge.Source], edge.Target)
	}
	for nodeID := range outgoing {
		sort.Strings(outgoing[nodeID])
	}
	conflicts := make([]workflowScheduleConflictItem, 0)
	for source := range business {
		for _, target := range workflowScheduleNextBusinessNodes(source, outgoing, business) {
			left, right := plans[source], plans[target]
			if left.PlannedEndAt != nil && right.PlannedStartAt != nil && left.PlannedEndAt.After(*right.PlannedStartAt) {
				conflicts = append(conflicts, workflowScheduleConflictItem{
					Code: scheduleConflictSerialNodeOrder, NodeID: target, RelatedNodeID: source,
					Field: "planned_start_at", Boundary: left.PlannedEndAt, Value: right.PlannedStartAt,
				})
			}
		}
	}
	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].NodeID == conflicts[j].NodeID {
			return conflicts[i].RelatedNodeID < conflicts[j].RelatedNodeID
		}
		return conflicts[i].NodeID < conflicts[j].NodeID
	})
	return conflicts
}

func workflowScheduleNextBusinessNodes(source string, outgoing map[string][]string, business map[string]bool) []string {
	seen, found := map[string]bool{source: true}, map[string]bool{}
	queue := append([]string(nil), outgoing[source]...)
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		if seen[nodeID] {
			continue
		}
		seen[nodeID] = true
		if business[nodeID] {
			found[nodeID] = true
			continue
		}
		queue = append(queue, outgoing[nodeID]...)
	}
	result := make([]string, 0, len(found))
	for nodeID := range found {
		result = append(result, nodeID)
	}
	sort.Strings(result)
	return result
}

func (s *Server) workflowSubjectScheduleDTO(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
	definition workflowDefinition,
	runs []workflowgorm.WorkflowNodeRun,
) workflowSubjectSchedule {
	plans, err := decodeWorkflowNodePlans(instance)
	if err != nil {
		return unavailableWorkflowSubjectSchedule()
	}
	selected, edges, err := workflowScheduleDisplayTopology(tx, definition, instance, runs)
	if err != nil {
		return unavailableWorkflowSubjectSchedule()
	}
	scope, err := loadWorkflowScheduleScope(tx, instance)
	if err != nil {
		return unavailableWorkflowSubjectSchedule()
	}
	return workflowScheduleForParts(definition, selected, edges, plans, scope)
}

func unavailableWorkflowSubjectSchedule() workflowSubjectSchedule {
	return workflowSubjectSchedule{
		State: "conflict", ActualDatesIndependent: true,
		Conflicts: []workflowScheduleConflictItem{{Code: scheduleConflictSubjectUnavailable}},
	}
}
