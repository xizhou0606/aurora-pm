// Package scmapp assembles the code-contribution runtime shared by the API
// server and the standalone worker, mirroring the report scheduler's shared
// assembly pattern: one constructor, one fact load path, module-owned stores.
package scmapp

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	codecontrib "github.com/xizhou0606/aurora-pm/backend/internal/module/codecontrib"
	codecontribgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/codecontrib/gorm"
	codecontribhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/codecontrib/http"
	githubcollector "github.com/xizhou0606/aurora-pm/backend/internal/integration/github"
	gitlabcollector "github.com/xizhou0606/aurora-pm/backend/internal/integration/gitlab"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
)

// memberCandidateSource adapts the identity read service to the codecontrib
// port so the two modules never import each other.
type memberCandidateSource struct{ service *identity.MemberCandidateService }

func (source memberCandidateSource) FindActiveMemberCandidates(
	ctx context.Context, workspaceID uint,
) ([]codecontrib.MemberCandidate, error) {
	members, err := source.service.FindActiveMemberCandidates(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	candidates := make([]codecontrib.MemberCandidate, 0, len(members))
	for _, member := range members {
		candidates = append(candidates, codecontrib.MemberCandidate{
			FactID: member.FactID, Username: member.Username, Email: member.Email,
		})
	}
	return candidates, nil
}

// Runtime bundles every wiring product the two process assemblies need.
type Runtime struct {
	Sync        *codecontrib.SyncService
	Admin       *codecontrib.AdminService
	Facts       *codecontribgorm.FactsRepository
	ReportReads *codecontribgorm.ReportReadAdapter
	Handler     *codecontribhttp.Handler
}

// Assemble wires the stores, collectors, member candidates and services.
// Collectors are the only provider-aware components; everything downstream
// consumes neutral facts.
func Assemble(db *gorm.DB) (*Runtime, error) {
	if db == nil {
		return nil, fmt.Errorf("codecontrib bootstrap error: db is nil")
	}
	adminRepository := codecontribgorm.NewAdminRepository(db)
	factsRepository := codecontribgorm.NewFactsRepository(db)
	collectors := map[codecontrib.Provider]codecontrib.Collector{
		codecontrib.ProviderGitHub: githubcollector.NewCollector(""),
		codecontrib.ProviderGitLab: gitlabcollector.NewCollector(),
	}
	members, err := identity.NewMemberCandidateService(identitygorm.NewMemberCandidateRepository(db))
	if err != nil {
		return nil, fmt.Errorf("codecontrib member candidates bootstrap error: %w", err)
	}
	sync, err := codecontrib.NewSyncService(codecontrib.SyncDependencies{
		Connections:  adminRepository,
		Repositories: adminRepository,
		Settings:     adminRepository,
		Bindings:     factsRepository,
		Facts:        factsRepository,
		Collectors:   collectors,
		Members:      memberCandidateSource{service: members},
	})
	if err != nil {
		return nil, fmt.Errorf("codecontrib sync bootstrap error: %w", err)
	}
	admin, err := codecontrib.NewAdminService(codecontrib.AdminDependencies{
		Connections:  adminRepository,
		Repositories: adminRepository,
		Settings:     adminRepository,
		Bindings:     factsRepository,
		Collectors:   collectors,
		Members:      memberCandidateSource{service: members},
		Reaggregator: sync,
	})
	if err != nil {
		return nil, fmt.Errorf("codecontrib admin bootstrap error: %w", err)
	}
	handler, err := codecontribhttp.NewHandler(admin)
	if err != nil {
		return nil, fmt.Errorf("codecontrib handler bootstrap error: %w", err)
	}
	return &Runtime{
		Sync:        sync,
		Admin:       admin,
		Facts:       factsRepository,
		ReportReads: codecontribgorm.NewReportReadAdapter(factsRepository),
		Handler:     handler,
	}, nil
}
