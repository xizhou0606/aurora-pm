package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
)

type preparedQualityBugCreate struct {
	Bug                 qualitygorm.QualityBug
	Project             projectgorm.ProjectSet
	NotificationContext workspaceContext
	ReporterID          uint
}

type qualityBugCreateError struct {
	Status       int
	Message      string
	PlatformUser bool
	Cause        error
}

func (e *qualityBugCreateError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "quality bug validation failed"
}

func (e *qualityBugCreateError) Unwrap() error {
	return e.Cause
}

func (s *Server) prepareQualityBugCreate(
	c *gin.Context,
	workspace identitygorm.Workspace,
	authActor actor,
	body qualityBugBody,
) (preparedQualityBugCreate, error) {
	if body.RequirementID == 0 || body.Title == nil || strings.TrimSpace(*body.Title) == "" {
		return preparedQualityBugCreate{}, &qualityBugCreateError{
			Status: http.StatusBadRequest, Message: "requirement_id and title required",
		}
	}
	var requirement requirementgorm.Requirement
	if err := s.db.Where(
		"id = ? AND workspace_id = ?", body.RequirementID, workspace.ID,
	).First(&requirement).Error; err != nil {
		return preparedQualityBugCreate{}, &qualityBugCreateError{
			Status: http.StatusNotFound, Message: "requirement not found", Cause: err,
		}
	}
	var project projectgorm.ProjectSet
	if err := s.db.Where(
		"id = ? AND workspace_id = ?", requirement.ProjectSetID, workspace.ID,
	).First(&project).Error; err != nil {
		return preparedQualityBugCreate{}, &qualityBugCreateError{
			Status: http.StatusNotFound, Message: "project set not found", Cause: err,
		}
	}
	if !s.userCanAccessProject(c, workspace.ID, project.ID) {
		return preparedQualityBugCreate{}, &qualityBugCreateError{
			Status: http.StatusNotFound, Message: "project set not found",
		}
	}
	if err := s.validateWorkspaceHierarchy(workspace.ID, project, requirement); err != nil {
		return preparedQualityBugCreate{}, &qualityBugCreateError{
			Status: http.StatusBadRequest, Message: err.Error(), Cause: err,
		}
	}
	if err := s.validateActivePMUserReferences(c, qualityBodyUserFactIDs(body, false)); err != nil {
		return preparedQualityBugCreate{}, &qualityBugCreateError{
			Status: http.StatusBadRequest, Message: err.Error(), PlatformUser: true, Cause: err,
		}
	}
	if body.FoundEnvironment == nil || strings.TrimSpace(*body.FoundEnvironment) == "" {
		defaultEnvironment := quality.DefaultEnvironmentName
		body.FoundEnvironment = &defaultEnvironment
	}
	updates, err := s.qualityBugUpdates(body)
	if err != nil {
		return preparedQualityBugCreate{}, &qualityBugCreateError{
			Status: http.StatusBadRequest, Message: err.Error(), Cause: err,
		}
	}
	reporterID := factUserID(authActor.User)
	bug := qualitygorm.QualityBug{
		WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
		ProjectID: requirement.PlanUnitID, RequirementID: requirement.ID,
		Title: strings.TrimSpace(*body.Title), Status: "triage", Priority: "medium",
		SeverityKey: "p2", DefectDomain: quality.BugDefectDomainFunctional, ReporterUserID: &reporterID,
		StatusChangedAt: time.Now().UTC(),
	}
	effectiveStatus := "triage"
	if value, ok := updates["status"].(string); ok {
		effectiveStatus = value
	}
	if (effectiveStatus == "review" || effectiveStatus == "done") &&
		strings.TrimSpace(stringUpdate(updates, "resolution_reason")) == "" {
		return preparedQualityBugCreate{}, &qualityBugCreateError{
			Status: http.StatusBadRequest, Message: "resolution_reason required",
		}
	}
	applyQualityStatusTimestamps(&bug, updates, reporterID)
	applyQualityBugUpdates(&bug, updates)
	applyQualityPointers(&bug, updates)
	return preparedQualityBugCreate{
		Bug: bug, Project: project, ReporterID: reporterID,
		NotificationContext: s.loadWorkspaceContext(
			workspace.ID,
			workspaceContextReferences{}.withQualityBug(bug),
		),
	}, nil
}

func (s *Server) createPreparedQualityBug(
	tx *gorm.DB,
	c *gin.Context,
	authActor actor,
	prepared *preparedQualityBugCreate,
) error {
	bug := &prepared.Bug
	lockedProject, err := lockProjectSetTx(tx, bug.WorkspaceID, prepared.Project.ID)
	if err != nil {
		return err
	}
	lockedRequirement, err := lockProjectRequirementTx(
		tx, bug.WorkspaceID, bug.RequirementID, lockedProject.ID,
	)
	if err != nil {
		return err
	}
	if !requirementMatchesProject(lockedRequirement, lockedProject) {
		return errProjectChildScopeChanged
	}
	prepared.Project = lockedProject
	bug.TeamID = lockedRequirement.TeamID
	bug.ProjectSetID = lockedRequirement.ProjectSetID
	bug.ProjectID = lockedRequirement.PlanUnitID
	bug.RequirementID = lockedRequirement.ID
	if err := tx.Create(bug).Error; err != nil {
		return err
	}
	bug.BugKey = fmt.Sprintf("BUG-%d", bug.ID)
	if err := tx.Model(&qualitygorm.QualityBug{}).
		Where("id = ? AND workspace_id = ?", bug.ID, bug.WorkspaceID).
		Update("bug_key", bug.BugKey).Error; err != nil {
		return err
	}
	if err := tx.Create(&qualitygorm.QualityBugActivity{
		WorkspaceID: bug.WorkspaceID, QualityBugID: bug.ID,
		UserID: factUserID(authActor.User), Action: "created", Summary: "提交缺陷", CreatedAt: bug.CreatedAt,
	}).Error; err != nil {
		return err
	}
	if bug.AssigneeUserID != nil && *bug.AssigneeUserID > 0 {
		if err := tx.Create(&qualitygorm.QualityBugActivity{
			WorkspaceID: bug.WorkspaceID, QualityBugID: bug.ID,
			UserID: factUserID(authActor.User), Action: "assigned", FieldKey: "assignee_user_id",
			NewValue: strconv.FormatUint(uint64(*bug.AssigneeUserID), 10),
			Summary:  "创建时指派负责人", CreatedAt: bug.CreatedAt,
		}).Error; err != nil {
			return err
		}
	}
	if err := s.recordMutation(
		tx, c, prepared.Project.ID, "quality_bug", bug.ID, "created", bug.Title, *bug,
	); err != nil {
		return err
	}
	recipients := []uint{prepared.ReporterID}
	if bug.AssigneeUserID != nil {
		recipients = append(recipients, *bug.AssigneeUserID)
	}
	return s.recordQualityBugNotification(
		tx, c, *bug, prepared.NotificationContext, notification.SceneQualityBugCreated,
		recipients, nil, "", true,
	)
}

func respondQualityBugCreateError(c *gin.Context, err error) {
	if respondQualityNoteLengthError(c, err) {
		return
	}
	var validation *qualityBugCreateError
	if !errors.As(err, &validation) {
		bad(c, err.Error())
		return
	}
	if validation.PlatformUser {
		respondPMUserRegistryValidationError(c, validation.Cause)
		return
	}
	fail(c, validation.Status, validation.Message)
}
