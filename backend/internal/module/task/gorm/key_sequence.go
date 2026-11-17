// Package taskgorm contains the GORM-backed persistence helpers for the task
// module. It is intentionally not a cross-domain utility package.
package taskgorm

import (
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var nonTaskKey = regexp.MustCompile(`[^A-Z0-9]+`)

// ProjectScope is the project identity needed to reserve a task key. Keeping
// this value local prevents the task persistence adapter from depending on a
// project-module GORM record.
type ProjectScope struct {
	ID          uint
	WorkspaceID uint
	Code        string
}

// Next reserves the next task key for a project. Keeping this allocator outside
// the HTTP layer lets background jobs create native PM tasks without inventing
// a second sequence contract.
func Next(tx *gorm.DB, project ProjectScope) (string, error) {
	prefix := nonTaskKey.ReplaceAllString(strings.ToUpper(project.Code), "")
	if prefix == "" {
		prefix = fmt.Sprintf("PM%d", project.ID)
	}
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	seed := KeySequence{
		WorkspaceID: project.WorkspaceID, ProjectSetID: project.ID, Prefix: prefix, NextValue: 1,
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "workspace_id"}, {Name: "project_set_id"}, {Name: "prefix"},
		},
		DoNothing: true,
	}).Create(&seed).Error; err != nil {
		return "", err
	}
	var sequence KeySequence
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("workspace_id = ? AND project_set_id = ? AND prefix = ?",
			project.WorkspaceID, project.ID, prefix).
		First(&sequence).Error; err != nil {
		return "", err
	}
	value := sequence.NextValue
	if value == 0 {
		value = 1
	}
	var key string
	for {
		key = fmt.Sprintf("%s-%d", prefix, value)
		var occupied int64
		if err := tx.Model(&WorkTask{}).
			Where("workspace_id = ? AND project_set_id = ? AND task_key = ?",
				project.WorkspaceID, project.ID, key).
			Count(&occupied).Error; err != nil {
			return "", err
		}
		if occupied == 0 {
			break
		}
		if value == ^uint(0) {
			return "", fmt.Errorf("task key sequence exhausted")
		}
		value++
	}
	if value == ^uint(0) {
		return "", fmt.Errorf("task key sequence exhausted")
	}
	if err := tx.Model(&sequence).UpdateColumn("next_value", value+1).Error; err != nil {
		return "", err
	}
	return key, nil
}
