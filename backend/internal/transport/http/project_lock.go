package httpapi

import (
	"errors"
	"sort"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errProjectDeletionInProgress = errors.New("project deletion is in progress")

func lockProjectSetTx(tx *gorm.DB, workspaceID, projectID uint) (projectgorm.ProjectSet, error) {
	var project projectgorm.ProjectSet
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND workspace_id = ?", projectID, workspaceID).
		First(&project).Error
	if err == nil && project.MutationState == "deleting" {
		return projectgorm.ProjectSet{}, errProjectDeletionInProgress
	}
	return project, err
}

func lockProjectSetsTx(tx *gorm.DB, workspaceID uint, projectIDs []uint) (map[uint]projectgorm.ProjectSet, error) {
	unique := make(map[uint]struct{}, len(projectIDs))
	ordered := make([]uint, 0, len(projectIDs))
	for _, projectID := range projectIDs {
		if projectID == 0 {
			continue
		}
		if _, exists := unique[projectID]; exists {
			continue
		}
		unique[projectID] = struct{}{}
		ordered = append(ordered, projectID)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })

	projects := make(map[uint]projectgorm.ProjectSet, len(ordered))
	for _, projectID := range ordered {
		project, err := lockProjectSetTx(tx, workspaceID, projectID)
		if err != nil {
			return nil, err
		}
		projects[projectID] = project
	}
	return projects, nil
}
