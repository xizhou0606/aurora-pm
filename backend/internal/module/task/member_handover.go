package task

import (
	"context"
	"errors"
	"time"
)

var ErrMemberTasksChanged = errors.New("member tasks changed; refresh before confirming")

type MemberTask struct {
	ID         uint      `json:"id"`
	TaskKey    string    `json:"task_key"`
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	AssigneeID uint      `json:"assignee_id"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type MemberTaskResolution struct {
	Tasks      []MemberTask `json:"tasks"`
	AssigneeID *uint        `json:"assignee_user_id"`
}

// MemberHandover participates in the caller's membership transaction.
// The confirmed snapshot must exactly match the current outstanding tasks.
type MemberHandover interface {
	ListMemberTasks(context.Context, uint, uint, []uint) ([]MemberTask, error)
	ResolveMemberTasks(context.Context, uint, uint, uint, []uint, MemberTaskResolution) error
}
