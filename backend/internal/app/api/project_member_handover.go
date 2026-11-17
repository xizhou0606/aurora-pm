package api

import (
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"gorm.io/gorm"
)

func newPlanUnitMemberRepository(db *gorm.DB, notificationConfig config.Notification) *projectgorm.PlanUnitMemberRepository {
	return projectgorm.NewPlanUnitMemberRepository(db, func(tx *gorm.DB) task.MemberHandover {
		return taskgorm.NewMemberHandoverRepository(tx, func(inner *gorm.DB) (taskgorm.BatchMutationDependencies, error) {
			return newTaskBatchMutationDependencies(inner, notificationConfig)
		})
	})
}
