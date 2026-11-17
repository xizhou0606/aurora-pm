package project

import (
	"context"
	"errors"
)

var ErrGuideUnavailable = errors.New("project access guide unavailable")

type AccessGuide struct {
	ProjectSetID   uint   `json:"project_set_id"`
	PlanUnitID     uint   `json:"plan_unit_id"`
	ProjectSetName string `json:"project_set_name"`
	OwnerUserID    uint   `json:"-"`
	TeamID         uint   `json:"-"`
	WorkspaceID    uint   `json:"workspace_id"`
	WorkspaceName  string `json:"workspace_name"`
	ProjectName    string `json:"project_name"`
	OwnerName      string `json:"owner_name"`
}
type GuideLocator interface {
	LocateProject(context.Context, uint) (uint, uint, error)
}
type GuideProjectReader interface {
	ReadGuideProject(context.Context, uint, uint) (AccessGuide, error)
}
type GuideDirectory interface {
	GuideWorkspaceName(context.Context, uint) (string, error)
	GuideOwnerName(context.Context, uint) (string, error)
}
type AccessGuideService struct {
	Tasks, Bugs, Units GuideLocator
	Projects           GuideProjectReader
	Directory          GuideDirectory
}

// This directory lookup deliberately exposes only application-routing metadata
// across workspaces to admitted accounts. It does not authorize object access.
func (s AccessGuideService) Get(ctx context.Context, kind string, id uint) (AccessGuide, error) {
	var result AccessGuide
	locator := s.Tasks
	switch kind {
	case "plan_unit":
		locator = s.Units
	case "task":
	case "quality_bug":
		locator = s.Bugs
	default:
		return result, ErrGuideUnavailable
	}
	w, p, err := locator.LocateProject(ctx, id)
	if err != nil {
		return result, err
	}
	if w == 0 || p == 0 {
		return result, ErrGuideUnavailable
	}
	result, err = s.Projects.ReadGuideProject(ctx, w, p)
	if err != nil {
		return result, err
	}
	if result.ProjectName == "" {
		return result, ErrGuideUnavailable
	}
	workspace, err := s.Directory.GuideWorkspaceName(ctx, w)
	if err != nil {
		return result, err
	}
	if workspace == "" {
		return result, ErrGuideUnavailable
	}
	ownerName, err := s.Directory.GuideOwnerName(ctx, result.OwnerUserID)
	if err != nil {
		return result, err
	}
	result.WorkspaceID = w
	result.WorkspaceName = workspace
	result.OwnerName = ownerName
	return result, nil
}
