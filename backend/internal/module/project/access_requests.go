package project

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrJoinForbidden = errors.New("project request forbidden")
var ErrJoinWorkspaceRequired = errors.New("target workspace membership required")
var ErrJoinConflict = errors.New("project request conflict")
var ErrJoinInvalid = errors.New("invalid project request")

type JoinRequest struct {
	ID              uint      `json:"id"`
	ApplicantUserID uint      `json:"applicant_user_id"`
	ApplicantName   string    `json:"applicant_name"`
	Reason          string    `json:"reason"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       time.Time `json:"expires_at"`
}
type JoinActor struct{ UserID, FactID uint }
type JoinTarget struct {
	Kind string
	ID   uint
}
type JoinScope struct {
	AccessGuide
	TargetKind      string `json:"target_kind"`
	ReviewerIDs     []uint `json:"-"`
	CanReview       bool   `json:"can_review"`
	HasAccess       bool   `json:"has_access"`
	WorkspaceMember bool   `json:"workspace_member"`
}
type JoinContext struct {
	JoinScope
	Items      []JoinRequest `json:"items"`
	CanRequest bool          `json:"can_request"`
	HasMore    bool          `json:"has_more"`
}
type JoinMutation struct {
	Actor     JoinActor
	Target    JoinTarget
	RequestID uint
	Reason    string
	Decision  string
}
type JoinStore interface {
	Context(context.Context, JoinActor, JoinTarget) (JoinContext, error)
	Mutate(context.Context, JoinMutation) (JoinRequest, error)
}
type JoinService struct{ Store JoinStore }

func (s JoinService) Context(ctx context.Context, a JoinActor, t JoinTarget) (JoinContext, error) {
	if a.UserID == 0 || a.FactID == 0 || (t.ID == 0 || (t.Kind != "workspace" && t.Kind != "plan_unit" && t.Kind != "task" && t.Kind != "quality_bug")) {
		return JoinContext{}, ErrJoinInvalid
	}
	return s.Store.Context(ctx, a, t)
}
func (s JoinService) Create(ctx context.Context, a JoinActor, t JoinTarget, reason string) (JoinRequest, error) {
	reason = strings.TrimSpace(reason)
	if a.UserID == 0 || a.FactID == 0 || (t.ID == 0 || (t.Kind != "workspace" && t.Kind != "plan_unit" && t.Kind != "task" && t.Kind != "quality_bug")) || reason == "" || len([]rune(reason)) > 1000 {
		return JoinRequest{}, ErrJoinInvalid
	}
	return s.Store.Mutate(ctx, JoinMutation{Actor: a, Target: t, Reason: reason})
}
func (s JoinService) Review(ctx context.Context, a JoinActor, t JoinTarget, id uint, decision string) (JoinRequest, error) {
	if a.UserID == 0 || a.FactID == 0 || (t.ID == 0 || (t.Kind != "workspace" && t.Kind != "plan_unit" && t.Kind != "task" && t.Kind != "quality_bug")) || id == 0 || (decision != "approved" && decision != "rejected") {
		return JoinRequest{}, ErrJoinInvalid
	}
	return s.Store.Mutate(ctx, JoinMutation{Actor: a, Target: t, RequestID: id, Decision: decision})
}
func ValidateJoinScope(scope JoinScope, review bool) error {
	if review {
		if !scope.CanReview {
			return ErrJoinForbidden
		}
		return nil
	}
	if scope.TargetKind != "workspace" && !scope.WorkspaceMember {
		return ErrJoinWorkspaceRequired
	}
	if scope.HasAccess || len(scope.ReviewerIDs) == 0 {
		return ErrJoinForbidden
	}
	return nil
}
