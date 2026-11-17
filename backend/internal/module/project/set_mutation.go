package project

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrProjectReferenceInvalid = errors.New("project mutation reference is invalid")

type SetMutationOperation string

const (
	SetCreate        SetMutationOperation = "set.create"
	SetUpdate        SetMutationOperation = "set.update"
	SetDelete        SetMutationOperation = "set.delete"
	SetTransferOwner SetMutationOperation = "set.transfer_owner"
)

type SetMutation struct {
	Operation   SetMutationOperation
	WorkspaceID uint
	ActorUserID uint
	ApplyKey    string
	InputDigest string
	ID          uint

	TeamID          uint
	TeamIDSet       bool
	Name            string
	NameSet         bool
	Code            string
	Description     string
	DescriptionSet  bool
	OwnerUserID     *uint
	OwnerSet        bool
	PlannedStartAt  *time.Time
	PlannedStartSet bool
	PlannedEndAt    *time.Time
	PlannedEndSet   bool
	Status          string
	StatusSet       bool

	NewOwnerUserID uint
	TransferReason string
}

type SetMutationResult struct {
	ID             uint       `json:"id"`
	WorkspaceID    uint       `json:"workspace_id"`
	TeamID         uint       `json:"team_id"`
	Name           string     `json:"name"`
	Code           string     `json:"code"`
	Description    string     `json:"description"`
	OwnerUserID    *uint      `json:"owner_user_id"`
	PlannedStartAt *time.Time `json:"planned_start_at"`
	PlannedEndAt   *time.Time `json:"planned_end_at"`
	Status         string     `json:"status"`
	Deleted        bool       `json:"deleted,omitempty"`
}

type ProjectReferenceValidator interface {
	ProjectReferencesValid(context.Context, uint, uint, []uint) (bool, error)
}

type SetMutationStore interface {
	ApplyProjectSet(context.Context, SetMutation) (SetMutationResult, error)
}

type SetMutationService struct {
	store      SetMutationStore
	references ProjectReferenceValidator
}

func NewSetMutationService(store SetMutationStore, references ProjectReferenceValidator) (*SetMutationService, error) {
	if store == nil || references == nil {
		return nil, ErrMutationInvalid
	}
	return &SetMutationService{store: store, references: references}, nil
}

func (service *SetMutationService) ApplyProjectSet(ctx context.Context, command SetMutation) (SetMutationResult, error) {
	command.Name = strings.TrimSpace(command.Name)
	command.Code = strings.TrimSpace(command.Code)
	command.Description = strings.TrimSpace(command.Description)
	command.Status = strings.ToLower(strings.TrimSpace(command.Status))
	command.TransferReason = strings.TrimSpace(command.TransferReason)
	if service == nil || service.store == nil || service.references == nil || command.WorkspaceID == 0 ||
		command.ActorUserID == 0 || command.ApplyKey == "" || len(command.InputDigest) != 64 {
		return SetMutationResult{}, ErrMutationInvalid
	}
	teamID := uint(0)
	users := make([]uint, 0, 2)
	switch command.Operation {
	case SetCreate:
		if command.ID != 0 || !command.TeamIDSet || command.TeamID == 0 || !command.NameSet || command.Name == "" || len(command.Name) > 160 ||
			len(command.Code) > 40 || !validProjectSetStatus(command.Status, command.StatusSet) || invalidSchedule(command.PlannedStartAt, command.PlannedEndAt) {
			return SetMutationResult{}, ErrMutationInvalid
		}
		teamID = command.TeamID
		if !command.OwnerSet {
			owner := command.ActorUserID
			command.OwnerUserID = &owner
			command.OwnerSet = true
		}
		if command.OwnerUserID != nil {
			users = append(users, *command.OwnerUserID)
		}
	case SetUpdate:
		if command.ID == 0 || command.Code != "" || command.NameSet && (command.Name == "" || len(command.Name) > 160) ||
			command.TeamIDSet && command.TeamID == 0 || !validProjectSetStatus(command.Status, command.StatusSet) ||
			command.PlannedStartSet && command.PlannedEndSet && invalidSchedule(command.PlannedStartAt, command.PlannedEndAt) {
			return SetMutationResult{}, ErrMutationInvalid
		}
		if command.TeamIDSet {
			teamID = command.TeamID
		}
		if command.OwnerSet && command.OwnerUserID != nil {
			users = append(users, *command.OwnerUserID)
		}
	case SetTransferOwner:
		if command.ID == 0 || command.NewOwnerUserID == 0 || len(command.TransferReason) > 1000 {
			return SetMutationResult{}, ErrMutationInvalid
		}
		users = append(users, command.NewOwnerUserID)
	default:
		return SetMutationResult{}, ErrMutationInvalid
	}
	if teamID > 0 || len(users) > 0 {
		valid, err := service.references.ProjectReferencesValid(ctx, command.WorkspaceID, teamID, users)
		if err != nil {
			return SetMutationResult{}, err
		}
		if !valid {
			return SetMutationResult{}, ErrProjectReferenceInvalid
		}
	}
	return service.store.ApplyProjectSet(ctx, command)
}

func validProjectSetStatus(value string, set bool) bool {
	return !set || value == "active" || value == "archived"
}

func invalidSchedule(start, end *time.Time) bool {
	return start != nil && end != nil && start.After(*end)
}
