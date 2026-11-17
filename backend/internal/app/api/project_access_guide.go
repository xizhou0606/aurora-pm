package api

import (
	"github.com/gin-gonic/gin"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	projecthttp "github.com/xizhou0606/aurora-pm/backend/internal/module/project/http"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"

	resourceaccessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"gorm.io/gorm"
)

func projectGuideService(db *gorm.DB) project.AccessGuideService {
	return project.AccessGuideService{Tasks: taskgorm.NewAccessGuideRepository(db), Bugs: qualitygorm.NewAccessGuideRepository(db), Units: projectgorm.NewAccessGuideRepository(db), Projects: projectgorm.NewAccessGuideRepository(db), Directory: identitygorm.NewAccessGuideDirectory(db)}
}
func buildProjectAccessGuideRoutes(db *gorm.DB, configs ...config.Notification) func(*gin.RouterGroup) {
	cfg := config.Notification{}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	handler := projecthttp.NewAccessGuideHandler(projectGuideService(db))
	store := resourceaccessgorm.NewProjectRequestRepository(db, resourceaccessgorm.ProjectRequestHooks{
		Scope: membershipScope,
		Grant: func(tx *gorm.DB, s project.JoinScope, user, actor uint, reason string) error {
			if s.TargetKind == "workspace" {
				return identitygorm.NewAccessGuideDirectory(tx).AddApprovedWorkspaceMember(tx.Statement.Context, s.WorkspaceID, user, actor)
			}

			return projectgorm.NewAccessGuideRepository(tx).AddApprovedParticipant(tx.Statement.Context, s, user, actor, reason)
		},
		Names: func(tx *gorm.DB, ids []uint) (map[uint]string, error) {
			out := map[uint]string{}
			directory := identitygorm.NewAccessGuideDirectory(tx)
			for _, id := range ids {
				name, err := directory.GuideOwnerName(tx.Statement.Context, id)
				if err != nil {
					return nil, err
				}
				out[id] = name
			}
			return out, nil
		},
		Notify: membershipRequestNotifier(cfg),
	})
	handler.ConfigureRequests(&project.JoinService{Store: store})
	handler.ConfigureMemberships(&membershipGuideReader{db: db, requests: store})
	return handler.RegisterRoutes
}
