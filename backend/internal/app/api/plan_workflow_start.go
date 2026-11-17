package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"gorm.io/gorm"
)

type planWorkflowStarter struct{ workflows *workflow.SubjectStartService }

func (starter planWorkflowStarter) StartPlanUnitWorkflow(
	ctx context.Context,
	command project.PlanWorkflowStartCommand,
) (project.PlanWorkflowStartResult, error) {
	if starter.workflows == nil {
		return project.PlanWorkflowStartResult{}, project.ErrMutationRetryable
	}
	snapshot, err := json.Marshal(command.Subject)
	if err != nil {
		return project.PlanWorkflowStartResult{}, project.ErrMutationInvalid
	}
	result, err := starter.workflows.StartSubject(ctx, workflow.SubjectStartCommand{
		WorkspaceID: command.WorkspaceID, PrincipalID: command.ActorUserID,
		ApplyKey: command.ApplyKey, InputDigest: command.InputDigest, TemplateID: command.WorkflowTemplateID,
		SubjectType: "plan_unit", SubjectID: command.PlanUnitID, SubjectVersion: 1,
		ProjectSetID: command.ProjectSetID, TeamID: command.TeamID, SubjectTitle: command.Subject.Title,
		SubjectSnapshot: snapshot,
	})
	if err != nil {
		switch {
		case errorsIsWorkflowConflict(err):
			return project.PlanWorkflowStartResult{}, project.ErrMutationConflict
		default:
			return project.PlanWorkflowStartResult{}, project.ErrMutationRetryable
		}
	}
	return project.PlanWorkflowStartResult{InstanceID: result.InstanceID, SubjectStatus: result.SubjectStatus}, nil
}

func errorsIsWorkflowConflict(err error) bool {
	return errors.Is(err, workflow.ErrSubjectStartInvalid) || errors.Is(err, workflow.ErrMutationInvalid) ||
		errors.Is(err, workflow.ErrMutationNotFound) || errors.Is(err, workflow.ErrMutationConflict) ||
		errors.Is(err, workflow.ErrMutationUnavailable)
}

type planWorkflowAIStarter struct{ service *ai.WorkflowStartService }

func (starter planWorkflowAIStarter) StartSubjectAI(ctx context.Context, command workflow.SubjectAIStart) (uint, error) {
	if starter.service == nil {
		return 0, workflow.ErrMutationUnavailable
	}
	result, err := starter.service.StartWorkflowNode(ctx, ai.WorkflowStartCommand{
		WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID,
		Capability: command.Capability, SubjectType: command.SubjectType,
		SubjectID: command.SubjectID, SubjectVersion: command.SubjectVersion, SubjectSnapshot: command.SubjectSnapshot,
	})
	if err != nil {
		return 0, workflow.ErrMutationUnavailable
	}
	return result.RunID, nil
}

type planWorkflowAssignments struct {
	tx     *gorm.DB
	config config.Notification
}

func (writer planWorkflowAssignments) RecordSubjectAssignment(ctx context.Context, assignment workflow.SubjectAssignment) error {
	if writer.tx == nil {
		return workflow.ErrMutationUnavailable
	}
	if assignment.WorkspaceID == 0 || assignment.PrincipalID == 0 || assignment.ProjectSetID == 0 ||
		assignment.TeamID == 0 || assignment.InstanceID == 0 || assignment.AssigneeUserID == 0 {
		return workflow.ErrMutationConflict
	}
	users, err := toolNotificationUsers(ctx, writer.tx, assignment.WorkspaceID, assignment.PrincipalID, assignment.AssigneeUserID)
	if err != nil {
		return err
	}
	url := notification.WorkflowListURL(writer.config.PublicBaseURL)
	if assignment.SubjectType == "requirement" && assignment.SubjectID > 0 {
		url = notification.RequirementURL(writer.config.PublicBaseURL, assignment.SubjectID)
	}
	vars := map[string]string{
		"operator": toolNotificationUserLabel(users, assignment.PrincipalID),
		"node":     strings.TrimSpace(assignment.NodeName), "nodeId": assignment.NodeID,
		"assignee": toolNotificationUserLabel(users, assignment.AssigneeUserID),
		"title":    strings.TrimSpace(assignment.SubjectTitle), "url": url,
	}
	if vars["node"] == "" {
		vars["node"] = assignment.NodeID
	}
	if vars["title"] == "" {
		vars["title"] = "流程"
	}
	return notification.Enqueue(writer.tx.WithContext(ctx), "workflow_node_notification", assignment.InstanceID, notification.Intent{
		SkipExternal: !writer.config.Enabled, ResourceType: assignment.SubjectType, ResourceID: notification.WorkflowResourceID(assignment.SubjectType, assignment.ProjectSetID, assignment.SubjectID),
		WorkspaceID: assignment.WorkspaceID, Scene: notification.SceneWorkflowNodeAssigned,
		TeamID: assignment.TeamID, ProjectID: assignment.ProjectSetID, ActorUserID: assignment.PrincipalID,
		RecipientUserIDs: []uint{assignment.AssigneeUserID}, Vars: vars,
		FallbackPlainText: notification.PlainText(notification.SceneWorkflowNodeAssigned, vars),
	})
}

type workflowToolInvitations struct {
	tx     *gorm.DB
	config config.Notification
}

func (writer workflowToolInvitations) RecordPreReviewInvitation(
	ctx context.Context,
	invitation workflow.PreReviewInvitation,
) error {
	if writer.tx == nil || invitation.WorkspaceID == 0 || invitation.PrincipalID == 0 ||
		invitation.ProjectSetID == 0 || invitation.TeamID == 0 || invitation.InstanceID == 0 {
		return workflow.ErrMutationConflict
	}
	recipients := toolNotificationRecipients(invitation.PrincipalID, false, invitation.InviteUserIDs)
	if len(recipients) == 0 {
		return nil
	}
	users, err := toolNotificationUsers(ctx, writer.tx, invitation.WorkspaceID, invitation.PrincipalID)
	if err != nil {
		return err
	}
	url := notification.WorkflowListURL(writer.config.PublicBaseURL)
	if invitation.SubjectType == "requirement" && invitation.SubjectID > 0 {
		url = notification.RequirementURL(writer.config.PublicBaseURL, invitation.SubjectID)
	}
	vars := map[string]string{
		"operator": toolNotificationUserLabel(users, invitation.PrincipalID),
		"node":     strings.TrimSpace(invitation.NodeName), "nodeId": invitation.NodeID,
		"title": strings.TrimSpace(invitation.SubjectTitle), "url": url,
	}
	if vars["node"] == "" {
		vars["node"] = invitation.NodeID
	}
	if vars["title"] == "" {
		vars["title"] = "流程"
	}
	return notification.Enqueue(writer.tx.WithContext(ctx), "workflow_node_notification", invitation.InstanceID, notification.Intent{
		SkipExternal: !writer.config.Enabled, ResourceType: invitation.SubjectType, ResourceID: notification.WorkflowResourceID(invitation.SubjectType, invitation.ProjectSetID, invitation.SubjectID),
		WorkspaceID: invitation.WorkspaceID, Scene: notification.SceneWorkflowPreReviewInvited,
		TeamID: invitation.TeamID, ProjectID: invitation.ProjectSetID, ActorUserID: invitation.PrincipalID,
		RecipientUserIDs: recipients, Vars: vars,
		FallbackPlainText: notification.PlainText(notification.SceneWorkflowPreReviewInvited, vars),
	})
}

var _ project.PlanWorkflowStarter = planWorkflowStarter{}
var _ workflow.SubjectAIStarter = planWorkflowAIStarter{}
var _ workflow.SubjectAssignmentWriter = planWorkflowAssignments{}
var _ workflow.PreReviewInvitationWriter = workflowToolInvitations{}
