package httpapi

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

// Plan unit member management: owner + participants of ONE project/version
// item. Reads require version-level data access; member reads/write options
// and mutations require the member-management fact (set creator, unit owner,
// or access administrator).

// getPlanUnitMembers returns the owner and participants for a plan unit.
// GET /project-new/:id/members
func (s *Server) getPlanUnitMembers(c *gin.Context) {
	unit, _, allowed := s.planUnitByParam(c)
	if !allowed {
		return
	}

	page := pageNumber(c)
	pageSize := pageSize(c)
	query := c.Query("q")

	memberPage, err := s.memberFacts.ListPlanUnitMemberUsers(c.Request.Context(), unit.WorkspaceID, unit.ID, query, page, pageSize)
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{
		"items":     memberPage.Items,
		"page":      memberPage.Page,
		"page_size": memberPage.PageSize,
		"total":     memberPage.Total,
		"has_more":  memberPage.HasMore,
	})
}

// getPlanUnitMemberOptions searches active workspace users that can be added
// as participants. Unlike /members it is NOT limited to current unit members:
// it enumerates every active user of the workspace so a manager can add
// someone who has not joined the unit yet.
// GET /project-new/:id/member-options
func (s *Server) getPlanUnitMemberOptions(c *gin.Context) {
	unit, _, allowed := s.planUnitByParam(c)
	if !allowed {
		return
	}
	if !s.requirePlanUnitMemberManage(c, unit) {
		return
	}

	page := pageNumber(c)
	pageSize := pageSize(c)
	query := c.Query("q")

	memberPage, err := s.memberFacts.SearchWorkspaceActiveUsers(c.Request.Context(), unit.WorkspaceID, query, page, pageSize)
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{
		"items":     memberPage.Items,
		"page":      memberPage.Page,
		"page_size": memberPage.PageSize,
		"total":     memberPage.Total,
		"has_more":  memberPage.HasMore,
	})
}

// getPlanUnitMemberRemovalImpact returns outstanding responsibilities blocking
// a participant removal from one unit.
// GET /project-new/:id/members/:userID/removal-impact
func (s *Server) getPlanUnitMemberRemovalImpact(c *gin.Context) {
	unit, _, allowed := s.planUnitByParam(c)
	if !allowed {
		return
	}
	if !s.requirePlanUnitMemberManage(c, unit) {
		return
	}
	userID, okID := paramID(c, "userID")
	if !okID {
		return
	}

	inspector, isInspector := s.responsibilityInspector.(project.PlanUnitResponsibilityInspector)
	if !isInspector {
		dbFail(c)
		return
	}
	impact, err := inspector.InspectPlanUnitRemovalImpact(c.Request.Context(), unit.WorkspaceID, unit.ID, userID)
	if err != nil {
		dbFail(c)
		return
	}
	reader, available := s.memberFacts.(project.MemberTaskReader)
	if !available {
		dbFail(c)
		return
	}
	tasks, err := reader.ListPlanUnitMemberTasks(c.Request.Context(), unit.WorkspaceID, unit.ID, userID)
	if err != nil {
		dbFail(c)
		return
	}
	allowedTasks, valid := s.taskCreatorBatchAccess(c, accesscatalog.ActionWorkItemsTasksBatchUpdate, memberTaskPolicyRows(unit.WorkspaceID, tasks))
	if !valid {
		return
	}
	canResolve := true
	for _, row := range tasks {
		canResolve = canResolve && allowedTasks[row.ID]
	}
	ok(c, struct {
		project.RemovalImpact
		Tasks            []task.MemberTask `json:"tasks"`
		CanResolve       bool              `json:"can_resolve_tasks"`
		MembersUpdatedAt time.Time         `json:"members_updated_at"`
	}{impact, tasks, canResolve, unit.UpdatedAt})
}

// putPlanUnitMembers atomically replaces the owner and participants of one
// plan unit. Blocks removal of any participant with outstanding
// responsibilities inside the unit.
// PUT /project-new/:id/members
func (s *Server) putPlanUnitMembers(c *gin.Context) {
	unit, proj, allowed := s.planUnitByParam(c)
	if !allowed {
		return
	}
	if !s.requirePlanUnitMemberManage(c, unit) {
		return
	}

	var body struct {
		OwnerUserID       *uint                      `json:"owner_user_id"`
		UserIDs           []uint                     `json:"user_ids"`
		ExpectedUpdatedAt *string                    `json:"expected_updated_at"`
		Reason            string                     `json:"reason"`
		TaskResolution    *task.MemberTaskResolution `json:"task_resolution"`
	}
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	if body.OwnerUserID == nil || *body.OwnerUserID == 0 {
		bad(c, "owner_user_id is required")
		return
	}

	// Validate the new owner is active and workspace-scoped.
	if err := s.validateActivePMUserReferences(c, []uint{*body.OwnerUserID}); err != nil {
		respondPMUserRegistryValidationError(c, err)
		return
	}

	// The pre-save snapshot also drives the assignment notifications below.
	var currentState project.MemberState
	if s.memberFacts != nil && s.responsibilityInspector != nil {
		loaded, err := s.memberFacts.LoadPlanUnitMemberState(c.Request.Context(), unit.WorkspaceID, unit.ID)
		if err != nil {
			respondMemberError(c, err)
			return
		}
		currentState = loaded

	}

	// Task handover requires the existing task batch-update grant/object policy.
	if body.TaskResolution != nil && !s.requireTaskCreatorAction(c, accesscatalog.ActionWorkItemsTasksBatchUpdate, memberTaskPolicyRows(unit.WorkspaceID, body.TaskResolution.Tasks)...) {
		return
	}
	// Build the member state.
	state := project.MemberState{
		OwnerUserID:        body.OwnerUserID,
		ParticipantUserIDs: body.UserIDs,
		UpdatedAt:          unit.UpdatedAt,
	}
	meta := project.MemberMutationMeta{
		ActorUserID:    actorFactUserID(c),
		Reason:         body.Reason,
		TaskResolution: body.TaskResolution,
	}
	if body.ExpectedUpdatedAt != nil && *body.ExpectedUpdatedAt != "" {
		t, err := time.Parse(time.RFC3339, *body.ExpectedUpdatedAt)
		if err != nil {
			bad(c, "expected_updated_at must be RFC 3339")
			return
		}
		meta.ExpectedAt = &t
	}

	saved, err := s.memberFacts.SavePlanUnitMemberState(c.Request.Context(), unit.WorkspaceID, unit.ID, state, meta)
	if err != nil {
		respondMemberError(c, err)
		return
	}

	// IM notification for every newly assigned owner / added participant.
	// The member change is already committed; a notification failure must not
	// roll it back, so enqueue in its own transaction and surface failures in
	// logs only (the outbox remains the durable retry boundary).
	if notifyErr := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		return s.recordPlanUnitMemberNotifications(tx, proj, unit, currentState, saved, meta.ActorUserID)
	}); notifyErr != nil {
		log.Printf("plan unit member notification enqueue failed: unit=%d err=%v", unit.ID, notifyErr)
	}

	ok(c, project.PlanUnitMembersSnapshot{
		WorkspaceID:        unit.WorkspaceID,
		PlanUnitID:         unit.ID,
		ProjectSetID:       proj.ID,
		OwnerUserID:        saved.OwnerUserID,
		ParticipantUserIDs: saved.ParticipantUserIDs,
		UpdatedAt:          saved.UpdatedAt,
		CanManageMembers:   true,
	})
}

// respondMemberError maps domain member errors to HTTP responses.
func respondMemberError(c *gin.Context, err error) {
	var blocked *project.MemberRemovalBlocked
	switch {
	case errors.As(err, &blocked):
		failCode(c, http.StatusConflict, "PROJECT_MEMBER_RESPONSIBILITIES", blocked.Error())
	case errors.Is(err, task.ErrMemberTasksChanged), errors.Is(err, task.ErrBatchMutationConflict):
		failCode(c, http.StatusConflict, "PROJECT_MEMBER_TASKS_CHANGED", "member tasks changed; refresh before confirming")
	case errors.Is(err, task.ErrBatchMutationInvalid):
		bad(c, "invalid task handover")
	case errors.Is(err, project.ErrParticipantInvalid):
		bad(c, "project member parameters are invalid")
	case errors.Is(err, project.ErrParticipantPlanUnitNotFound):
		fail(c, http.StatusNotFound, "plan unit not found")
	case errors.Is(err, project.ErrParticipantProjectNotFound):
		fail(c, http.StatusNotFound, "project set not found")
	case errors.Is(err, project.ErrParticipantStale):
		failCode(c, http.StatusConflict, "PROJECT_MEMBERS_CHANGED", "project members changed concurrently")
	case errors.Is(err, project.ErrParticipantIsOwner):
		bad(c, "owner cannot be stored as a participant")
	case errors.Is(err, project.ErrParticipantPermissionDenied):
		fail(c, http.StatusForbidden, "project member modification is not permitted")
	default:
		dbFail(c)
	}
}

// Only IDs and workspace enter the existing task creator/action policy.
func memberTaskPolicyRows(workspaceID uint, tasks []task.MemberTask) []taskgorm.WorkTask {
	rows := make([]taskgorm.WorkTask, 0, len(tasks))
	for _, task := range tasks {
		rows = append(rows, taskgorm.WorkTask{ID: task.ID, WorkspaceID: workspaceID})
	}
	return rows
}
