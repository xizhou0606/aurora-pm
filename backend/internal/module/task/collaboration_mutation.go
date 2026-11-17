package task

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	ErrCollaborationMutationInvalid   = errors.New("task collaboration mutation is invalid")
	ErrCollaborationMutationNotFound  = errors.New("task collaboration mutation target not found")
	ErrCollaborationMutationConflict  = errors.New("task collaboration mutation conflicts with current state")
	ErrCollaborationMutationForbidden = errors.New("task collaboration mutation is forbidden")
)

type CollaborationMutationOperation string

const (
	CommentCreate    CollaborationMutationOperation = "comment.create"
	CommentDelete    CollaborationMutationOperation = "comment.delete"
	AttachmentDelete CollaborationMutationOperation = "attachment.delete"
	WatchUpdate      CollaborationMutationOperation = "watch.update"
)

type CollaborationMutation struct {
	Operation    CollaborationMutationOperation
	WorkspaceID  uint
	PrincipalID  uint
	ApplyKey     string
	ChangeSetID  string
	InputDigest  string
	TaskID       uint
	TaskScope    MutationTaskScope
	CommentID    uint
	AttachmentID uint
	Body         string
	Watch        bool
}

type CollaborationMutationResult struct {
	Comment          *CommentSnapshot `json:"comment,omitempty"`
	DeletedID        uint             `json:"deleted_id,omitempty"`
	Deleted          bool             `json:"deleted,omitempty"`
	WorkTaskID       uint             `json:"work_task_id,omitempty"`
	Watched          *bool            `json:"watched,omitempty"`
	PendingObjectKey string           `json:"-"`
}

type CommentNotification struct {
	CommentID                                                  uint
	WorkspaceID, PrincipalID, TeamID, ProjectSetID, PlanUnitID uint
	TaskID, AssigneeUserID, ReporterUserID                     uint
	TaskKey, Title, Body                                       string
}

type CollaborationNotificationWriter interface {
	RecordTaskComment(context.Context, CommentNotification) error
}

type CollaborationMutationStore interface {
	ApplyCollaborationMutation(context.Context, CollaborationMutation) (CollaborationMutationResult, error)
	CompleteAttachmentDeletion(context.Context, CollaborationMutation) (CollaborationMutationResult, error)
}

// AttachmentObjectRemover must make deletion idempotent. S3-compatible delete
// semantics satisfy this contract, allowing a pending durable saga to resume
// safely after the object was removed but before the database finalized.
type AttachmentObjectRemover interface {
	RemoveObject(context.Context, string) error
}

type CollaborationMutationService struct {
	store   CollaborationMutationStore
	remover AttachmentObjectRemover
}

func NewCollaborationMutationService(store CollaborationMutationStore, remover AttachmentObjectRemover) (*CollaborationMutationService, error) {
	if store == nil || remover == nil {
		return nil, ErrCollaborationMutationInvalid
	}
	return &CollaborationMutationService{store: store, remover: remover}, nil
}

func (service *CollaborationMutationService) ApplyCollaborationMutation(
	ctx context.Context, command CollaborationMutation,
) (CollaborationMutationResult, error) {
	if service == nil || service.store == nil || service.remover == nil || command.WorkspaceID == 0 || command.PrincipalID == 0 ||
		command.TaskID == 0 || command.TaskScope.TeamID == 0 || command.TaskScope.ProjectSetID == 0 ||
		command.TaskScope.PlanUnitID == 0 || command.TaskScope.RequirementID == 0 || strings.TrimSpace(command.ApplyKey) == "" ||
		strings.TrimSpace(command.ChangeSetID) == "" || len(command.InputDigest) != 64 {
		return CollaborationMutationResult{}, ErrCollaborationMutationInvalid
	}
	switch command.Operation {
	case CommentCreate:
		command.Body = strings.TrimSpace(command.Body)
		if command.Body == "" || len(command.Body) > 100_000 || utf8.RuneCountInString(command.Body) > 20_000 {
			return CollaborationMutationResult{}, ErrCollaborationMutationInvalid
		}
	case CommentDelete:
		if command.CommentID == 0 {
			return CollaborationMutationResult{}, ErrCollaborationMutationInvalid
		}
	case AttachmentDelete:
		if command.AttachmentID == 0 {
			return CollaborationMutationResult{}, ErrCollaborationMutationInvalid
		}
	case WatchUpdate:
	default:
		return CollaborationMutationResult{}, ErrCollaborationMutationInvalid
	}
	result, err := service.store.ApplyCollaborationMutation(ctx, command)
	if err != nil || command.Operation != AttachmentDelete || result.PendingObjectKey == "" {
		return result, err
	}
	if err := service.remover.RemoveObject(ctx, result.PendingObjectKey); err != nil {
		return CollaborationMutationResult{}, err
	}
	return service.store.CompleteAttachmentDeletion(ctx, command)
}

func IsCollaborationMutationBusinessError(err error) bool {
	return errors.Is(err, ErrCollaborationMutationInvalid) || errors.Is(err, ErrCollaborationMutationNotFound) ||
		errors.Is(err, ErrCollaborationMutationConflict) || errors.Is(err, ErrCollaborationMutationForbidden)
}
