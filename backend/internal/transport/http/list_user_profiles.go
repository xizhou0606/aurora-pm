package httpapi

import (
	"sort"

	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

type listUserProfile struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

func (s *Server) listUserProfiles(ids []uint) ([]listUserProfile, error) {
	users, err := s.findActivePMUsersByFactIDs(ids)
	if err != nil {
		return nil, err
	}
	profiles := make([]listUserProfile, 0, len(users))
	for id, user := range users {
		profiles = append(profiles, listUserProfile{
			ID: id, Username: user.Username, DisplayName: user.DisplayName, AvatarURL: user.AvatarURL,
		})
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	return profiles, nil
}

func requirementListUserIDs(requirements []requirementgorm.Requirement) []uint {
	var ids []uint
	for _, requirement := range requirements {
		ids = mergeSortedUserIDs(ids,
			pointerUserID(requirement.ProductOwnerUserID),
			pointerUserID(requirement.DesignOwnerUserID),
			pointerUserID(requirement.FrontendOwnerUserID),
			pointerUserID(requirement.BackendOwnerUserID),
		)
		ids = mergeSortedUserIDs(ids, []uint(requirement.ProductOwnerUserIDs)...)
		ids = mergeSortedUserIDs(ids, []uint(requirement.DesignOwnerUserIDs)...)
		ids = mergeSortedUserIDs(ids, []uint(requirement.FrontendOwnerUserIDs)...)
		ids = mergeSortedUserIDs(ids, []uint(requirement.BackendOwnerUserIDs)...)
		ids = mergeSortedUserIDs(ids, []uint(requirement.TesterUserIDs)...)
		ids = mergeSortedUserIDs(ids, []uint(requirement.DeveloperUserIDs)...)
	}
	return ids
}

func taskListUserIDs(tasks []taskgorm.WorkTask) []uint {
	var ids []uint
	for _, task := range tasks {
		ids = mergeSortedUserIDs(ids, pointerUserID(task.AssigneeID), pointerUserID(task.ReporterUserID))
	}
	return ids
}

func pointerUserID(id *uint) uint {
	if id == nil {
		return 0
	}
	return *id
}
