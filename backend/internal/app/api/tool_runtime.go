package api

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	ai "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	audit "github.com/xizhou0606/aurora-pm/backend/internal/module/audit"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	toolruntime "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
	toolruntimehttp "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/http"
	toolruntimemcp "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/mcp"
	toolruntimepmtools "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/pmtools"
	workbench "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
	biscreen "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/biscreen"
	workbenchgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/gorm"
	workflow "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	httpapi "github.com/xizhou0606/aurora-pm/backend/internal/transport/http"
)

// ToolRuntimeDependencies is the composition contract for the one canonical
// catalog and its executable handlers. Tests may override the production
// credential verifier, but production always uses the digest-backed service
// assembled from PM-owned identity, Grant, and credential facts.
type ToolRuntimeDependencies struct {
	Catalog               *toolruntime.Catalog
	Handlers              map[string]toolruntime.ToolHandler
	ExternalTokenVerifier toolruntime.ExternalTokenVerifier
	BIScreen              *workbench.BIScreenService
	MCPAuthorizationURL   string
	OAuthConfiguration    toolruntime.OAuthServerConfiguration
	BusinessActions       toolruntime.BusinessActionAuthorizer
}

type toolRuntimeRoutes struct {
	mountManagement func(*gin.RouterGroup)
	mountExternal   func(*gin.RouterGroup)
	mountDiscovery  func(gin.IRoutes)
}

func newPMToolRuntimeDependencies(db *gorm.DB, attachmentRemover task.AttachmentObjectRemover, runtimeConfigs ...config.Config) (ToolRuntimeDependencies, error) {
	if db == nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("PM Tool runtime database is required")
	}
	if attachmentRemover == nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("PM Tool runtime attachment remover is required")
	}
	var runtimeConfig config.Config
	if len(runtimeConfigs) > 0 {
		runtimeConfig = runtimeConfigs[0]
	}
	projectRepository := projectgorm.NewReadRepository(db)
	projects, err := project.NewReadService(projectRepository)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project Tool query bootstrap error: %w", err)
	}
	projectLists, err := project.NewListService(projectgorm.NewListRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project Tool list bootstrap error: %w", err)
	}
	requirements, err := requirement.NewReadService(requirementgorm.NewReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("requirement Tool query bootstrap error: %w", err)
	}
	projectToolAccess, err := newProjectAccessService(db)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project Tool access bootstrap error: %w", err)
	}
	projectToolReads, err := project.NewToolReadService(projectRepository, projectToolAccess)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project Tool read bootstrap error: %w", err)
	}
	projectMutations, err := project.NewMutationService(projectgorm.NewMutationRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project Tool mutation bootstrap error: %w", err)
	}
	projectReferences, err := identity.NewProjectReferenceService(identitygorm.NewProjectReferenceRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project Tool reference bootstrap error: %w", err)
	}
	projectSetMutations, err := project.NewSetMutationService(projectgorm.NewMutationRepository(db), projectReferences)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project set Tool mutation bootstrap error: %w", err)
	}
	projectPlanMutationRepository, err := projectgorm.NewPlanMutationRepository(db, func(tx *gorm.DB) (project.PlanScheduleGuard, error) {
		requirementSchedules, requirementErr := requirement.NewPlanUnitImpactService(requirementgorm.NewProjectImpactRepository(tx))
		if requirementErr != nil {
			return nil, requirementErr
		}
		workflowSchedules, workflowErr := workflow.NewSubjectScheduleService(workflowgorm.NewSubjectScheduleRepository(tx))
		if workflowErr != nil {
			return nil, workflowErr
		}
		return planScheduleGuard{requirements: requirementSchedules, workflows: workflowSchedules}, nil
	})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project plan Tool mutation repository bootstrap error: %w", err)
	}
	projectPlanMutations, err := project.NewPlanMutationService(projectPlanMutationRepository, projectReferences)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project plan Tool mutation bootstrap error: %w", err)
	}
	planWorkflowStarts, err := workflowgorm.NewSubjectStartRepository(db, func(tx *gorm.DB) (workflowgorm.SubjectStartDependencies, error) {
		aiStarts, aiErr := ai.NewWorkflowStartService(aigorm.NewWorkflowStartRepository(tx, runtimeConfig.AI.Enabled, runtimeConfig.AI.Model))
		if aiErr != nil {
			return workflowgorm.SubjectStartDependencies{}, aiErr
		}
		effects, effectErr := audit.NewMutationEffectService(auditgorm.NewMutationEffectRepository(tx))
		if effectErr != nil {
			return workflowgorm.SubjectStartDependencies{}, effectErr
		}
		return workflowgorm.SubjectStartDependencies{
			AI: planWorkflowAIStarter{service: aiStarts}, Assignments: planWorkflowAssignments{tx: tx, config: runtimeConfig.Notification},
			Effects: workflowToolActionEffects{effects: effects, tx: tx},
		}, nil
	})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project plan workflow start bootstrap error: %w", err)
	}
	planWorkflowStartService, err := workflow.NewSubjectStartService(planWorkflowStarts)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project plan workflow start service bootstrap error: %w", err)
	}
	projectPlanCreations, err := project.NewPlanCreationService(projectgorm.NewMutationRepository(db), projectReferences, planWorkflowStarter{workflows: planWorkflowStartService})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project plan create Tool mutation bootstrap error: %w", err)
	}
	projectDeletionImpact, err := newProjectDeletionImpactQueries(db)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project deletion impact bootstrap error: %w", err)
	}
	projectDeletionSteps, err := newProjectDeletionSteps(db)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project deletion step bootstrap error: %w", err)
	}
	projectDeletions, err := project.NewDeletionService(projectgorm.NewMutationRepository(db), projectDeletionImpact, projectDeletionSteps)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project deletion Tool mutation bootstrap error: %w", err)
	}
	planWorkflow, err := workflow.NewProjectDeletionService(workflowgorm.NewProjectDeletionRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, err
	}
	planRequirements, err := requirement.NewPlanUnitImpactService(requirementgorm.NewProjectImpactRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, err
	}
	planAI, err := ai.NewProjectImpactService(aigorm.NewProjectImpactRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, err
	}
	planDeletions, err := project.NewPlanDeletionService(projectgorm.NewMutationRepository(db), &planDeletionImpact{requirements: planRequirements, workflow: planWorkflow, ai: planAI}, planWorkflow)
	if err != nil {
		return ToolRuntimeDependencies{}, err
	}
	projectWriteHandlers, err := toolruntimepmtools.NewProjectWriteHandlers(projectMutations, projectSetMutations, projectDeletions, projectPlanMutations, planDeletions, projectPlanCreations)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project Tool mutation handler bootstrap error: %w", err)
	}
	requirementMutationRepository, err := requirementgorm.NewMutationRepository(db, func(tx *gorm.DB) (requirementgorm.MutationDependencies, error) {
		planScopes, scopeErr := project.NewPlanAssignmentService(projectgorm.NewPlanAssignmentRepository(tx))
		if scopeErr != nil {
			return requirementgorm.MutationDependencies{}, scopeErr
		}
		taskLifecycle, taskErr := task.NewRequirementLifecycleService(taskgorm.NewRequirementLifecycleRepository(tx))
		if taskErr != nil {
			return requirementgorm.MutationDependencies{}, taskErr
		}
		cascadeGuard, guardErr := task.NewCascadeDeletionGuardService(taskgorm.NewCascadeDeletionGuardRepository(tx))
		if guardErr != nil {
			return requirementgorm.MutationDependencies{}, guardErr
		}
		qualityLifecycle, qualityErr := quality.NewRequirementLifecycleService(qualitygorm.NewRequirementLifecycleRepository(tx))
		if qualityErr != nil {
			return requirementgorm.MutationDependencies{}, qualityErr
		}
		aiDeletion, aiErr := ai.NewProjectDeletionService(aigorm.NewProjectDeletionRepository(tx))
		if aiErr != nil {
			return requirementgorm.MutationDependencies{}, aiErr
		}
		workflowCleanup, cleanupErr := workflow.NewProjectDeletionService(workflowgorm.NewProjectDeletionRepository(tx))
		if cleanupErr != nil {
			return requirementgorm.MutationDependencies{}, cleanupErr
		}
		workflowSchedules, scheduleErr := workflow.NewSubjectScheduleService(workflowgorm.NewSubjectScheduleRepository(tx))
		if scheduleErr != nil {
			return requirementgorm.MutationDependencies{}, scheduleErr
		}
		projectReads, readErr := project.NewReadService(projectgorm.NewReadRepository(tx))
		if readErr != nil {
			return requirementgorm.MutationDependencies{}, readErr
		}
		workflowScheduleConfirm, confirmErr := project.NewWorkflowScheduleService(projectgorm.NewWorkflowScheduleRepository(tx))
		if confirmErr != nil {
			return requirementgorm.MutationDependencies{}, confirmErr
		}
		effects, effectErr := audit.NewMutationEffectService(auditgorm.NewMutationEffectRepository(tx))
		if effectErr != nil {
			return requirementgorm.MutationDependencies{}, effectErr
		}
		startsRepo, startErr := workflowgorm.NewSubjectStartRepository(tx, func(startTx *gorm.DB) (workflowgorm.SubjectStartDependencies, error) {
			aiStarts, err := ai.NewWorkflowStartService(aigorm.NewWorkflowStartRepository(startTx, runtimeConfig.AI.Enabled, runtimeConfig.AI.Model))
			if err != nil {
				return workflowgorm.SubjectStartDependencies{}, err
			}
			startEffects, err := audit.NewMutationEffectService(auditgorm.NewMutationEffectRepository(startTx))
			if err != nil {
				return workflowgorm.SubjectStartDependencies{}, err
			}
			return workflowgorm.SubjectStartDependencies{AI: planWorkflowAIStarter{service: aiStarts}, Assignments: planWorkflowAssignments{tx: startTx, config: runtimeConfig.Notification}, Effects: workflowToolActionEffects{effects: startEffects, tx: startTx}}, nil
		})
		if startErr != nil {
			return requirementgorm.MutationDependencies{}, startErr
		}
		starts, startErr := workflow.NewSubjectStartService(startsRepo)
		if startErr != nil {
			return requirementgorm.MutationDependencies{}, startErr
		}
		return requirementgorm.MutationDependencies{
			Scopes:     requirementPlanScopes{plans: planScopes},
			Workflow:   requirementWorkflowLifecycle{starts: starts, cleanup: workflowCleanup, schedules: workflowSchedules, projects: projectReads, confirm: workflowScheduleConfirm},
			Dependents: requirementDependents{tasks: taskLifecycle, guard: cascadeGuard, quality: qualityLifecycle, ai: aiDeletion},
			Effects:    requirementEffects{effects: effects, tx: tx}, Covers: requirementCoverLifecycle{tx: tx, config: runtimeConfig},
			ScopeRevisions: requirementScopeRevisions{tx: tx},
		}, nil
	})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("requirement Tool mutation repository bootstrap error: %w", err)
	}
	requirementMutations, err := requirement.NewMutationService(requirementMutationRepository, requirementUserValidator{references: projectReferences})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("requirement Tool mutation bootstrap error: %w", err)
	}
	requirementWriteHandlers, err := toolruntimepmtools.NewRequirementWriteHandlers(requirementMutations)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("requirement Tool mutation handler bootstrap error: %w", err)
	}
	requirementLists, err := requirement.NewListService(requirementgorm.NewReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("requirement Tool list bootstrap error: %w", err)
	}
	taskRepository := taskgorm.NewReadRepository(db)
	tasks, err := task.NewReadService(taskRepository)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task Tool query bootstrap error: %w", err)
	}
	taskReads, err := task.NewToolReadService(taskRepository)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task Tool list bootstrap error: %w", err)
	}
	taskClientOptions, err := task.NewClientOptionService(taskgorm.NewClientOptionRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task client option Tool bootstrap error: %w", err)
	}
	taskTestingReads, err := task.NewTestingReadService(taskgorm.NewTestingReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task testing Tool read bootstrap error: %w", err)
	}
	taskTestingMutationRepository, err := taskgorm.NewTestingMutationRepository(db)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task testing Tool mutation repository bootstrap error: %w", err)
	}
	taskTestingMutations, err := task.NewTestingMutationService(taskTestingMutationRepository)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task testing Tool mutation bootstrap error: %w", err)
	}
	taskCollaborationRepository, err := taskgorm.NewCollaborationMutationRepository(db, func(tx *gorm.DB) (task.CollaborationNotificationWriter, error) {
		return taskToolCollaborationNotifications{tx: tx, config: runtimeConfig.Notification}, nil
	})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task collaboration Tool mutation repository bootstrap error: %w", err)
	}
	taskCollaborationMutations, err := task.NewCollaborationMutationService(taskCollaborationRepository, attachmentRemover)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task collaboration Tool mutation bootstrap error: %w", err)
	}
	taskCoreMutationRepository, err := taskgorm.NewCoreMutationRepository(
		db,
		func(tx *gorm.DB) (task.CoreMutationFacts, error) {
			return newTaskToolMutationFacts(tx)
		},
		func(tx *gorm.DB) (task.CoreMutationEffectSink, error) {
			return newTaskToolMutationEffects(tx, runtimeConfig.Notification)
		},
		func(tx *gorm.DB) (task.TaskDependentCleaner, error) {
			return newTaskToolDependentCleaner(tx)
		},
	)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task core Tool mutation repository bootstrap error: %w", err)
	}
	taskCoreMutations, err := task.NewCoreMutationService(taskCoreMutationRepository)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task core Tool mutation bootstrap error: %w", err)
	}
	taskDeletions, err := task.NewTaskDeletionService(taskCoreMutationRepository, attachmentRemover)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task deletion Tool mutation bootstrap error: %w", err)
	}
	taskBatchMutationRepository, err := taskgorm.NewBatchMutationRepository(db, func(tx *gorm.DB) (taskgorm.BatchMutationDependencies, error) {
		return newTaskBatchMutationDependencies(tx, runtimeConfig.Notification)
	})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task batch Tool mutation repository bootstrap error: %w", err)
	}
	taskBatchMutations, err := task.NewBatchMutationService(taskBatchMutationRepository, taskDeletions)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task batch Tool mutation bootstrap error: %w", err)
	}
	taskWorkflowReferences, err := task.NewWorkflowReferenceService(taskgorm.NewWorkflowScopeRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task workflow Tool reference bootstrap error: %w", err)
	}
	workflows, err := workflow.NewReadService(workflowgorm.NewReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("workflow Tool query bootstrap error: %w", err)
	}
	workflowAuxReads, err := workflow.NewAuxReadService(workflowgorm.NewAuxReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("workflow auxiliary Tool query bootstrap error: %w", err)
	}
	workflowMutationRepository, err := workflowgorm.NewMutationRepository(db, func(tx *gorm.DB) (workflowgorm.MutationDependencies, error) {
		projectReads, projectErr := project.NewReadService(projectgorm.NewReadRepository(tx))
		if projectErr != nil {
			return workflowgorm.MutationDependencies{}, projectErr
		}
		requirementReads, requirementErr := requirement.NewReadService(requirementgorm.NewReadRepository(tx))
		if requirementErr != nil {
			return workflowgorm.MutationDependencies{}, requirementErr
		}
		memberships, membershipErr := identity.NewWorkspaceMembershipService(identitygorm.NewWorkspaceMembershipRepository(tx))
		if membershipErr != nil {
			return workflowgorm.MutationDependencies{}, membershipErr
		}
		schedules, scheduleErr := project.NewWorkflowScheduleService(projectgorm.NewWorkflowScheduleRepository(tx))
		if scheduleErr != nil {
			return workflowgorm.MutationDependencies{}, scheduleErr
		}
		effects, effectErr := audit.NewMutationEffectService(auditgorm.NewMutationEffectRepository(tx))
		if effectErr != nil {
			return workflowgorm.MutationDependencies{}, effectErr
		}
		aiActions, aiActionErr := ai.NewWorkflowActionService(aigorm.NewWorkflowActionRepository(tx))
		if aiActionErr != nil {
			return workflowgorm.MutationDependencies{}, aiActionErr
		}
		designs, designErr := requirement.NewWorkflowDesignService(requirementgorm.NewWorkflowDesignRepository(tx))
		if designErr != nil {
			return workflowgorm.MutationDependencies{}, designErr
		}
		return workflowgorm.MutationDependencies{
			Subjects:         workflowToolSubjectResolver{projects: projectReads, requirements: requirementReads},
			Users:            workflowToolUserVerifier{memberships: memberships},
			Reports:          workflowToolReportWriter{reports: quality.NewTestReportService(qualitygorm.NewTestReportRepository(tx))},
			Schedule:         workflowToolScheduleWriter{schedules: schedules},
			Effects:          workflowToolActionEffects{effects: effects},
			Assignments:      planWorkflowAssignments{tx: tx, config: runtimeConfig.Notification},
			Invitations:      workflowToolInvitations{tx: tx, config: runtimeConfig.Notification},
			AI:               workflowToolAIActions{actions: aiActions},
			Designs:          workflowToolDesignWriter{designs: designs},
			IntakeLocks:      workflowToolIntakeLocks{tx: tx},
			IntakeEffects:    workflowToolIntakeEffects{effects: effects},
			PlanRequirements: workflowToolPlanRequirements{tx: tx},
		}, nil
	})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("workflow Tool mutation repository bootstrap error: %w", err)
	}
	workflowMutations, err := workflow.NewMutationService(workflowMutationRepository)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("workflow Tool mutation bootstrap error: %w", err)
	}
	workflowWriteHandlers, err := toolruntimepmtools.NewWorkflowWriteHandlers(workflowMutations)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("workflow Tool mutation handler bootstrap error: %w", err)
	}
	baseHandlers, err := toolruntimepmtools.NewReadHandlers(toolruntimepmtools.ReadDependencies{
		Projects: projects, Requirements: requirements, Tasks: tasks, Workflows: workflows,
	})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("PM Tool handler bootstrap error: %w", err)
	}
	projectListHandlers, err := toolruntimepmtools.NewProjectListHandlers(toolruntimepmtools.ProjectListDependencies{
		Projects: projectLists,
		// project.set.list is project data: the membership filter comes from
		// the fact user ID resolved from the tool-runtime principal.
		Access: projectToolAccessUserResolver{access: projectToolAccess, resolver: projectgorm.NewFactUserResolver(db)},
	})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project Tool list handler bootstrap error: %w", err)
	}
	projectAccessHandlers, err := toolruntimepmtools.NewProjectAccessHandlers(toolruntimepmtools.ProjectAccessDependencies{Projects: projects})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project Tool access handler bootstrap error: %w", err)
	}
	taskReadHandlers, err := toolruntimepmtools.NewTaskReadHandlers(taskReads)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task Tool read handler bootstrap error: %w", err)
	}
	taskClientOptionHandlers, err := toolruntimepmtools.NewTaskClientOptionHandlers(taskClientOptions)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task client option Tool handler bootstrap error: %w", err)
	}
	taskTestingReadHandlers, err := toolruntimepmtools.NewTaskTestingReadHandlers(toolruntimepmtools.TaskTestingReadDependencies{
		Projects: projects, Requirements: requirements, Tasks: tasks,
		Testing: taskTestingReads, Workflow: taskWorkflowReferences,
	})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task testing Tool handler bootstrap error: %w", err)
	}
	taskTestingWriteHandlers, err := toolruntimepmtools.NewTaskTestingWriteHandlers(tasks, taskTestingMutations)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task testing Tool mutation handler bootstrap error: %w", err)
	}
	taskCollaborationWriteHandlers, err := toolruntimepmtools.NewTaskCollaborationWriteHandlers(tasks, taskCollaborationMutations)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task collaboration Tool mutation handler bootstrap error: %w", err)
	}
	taskCoreWriteHandlers, err := toolruntimepmtools.NewTaskCoreWriteHandlers(tasks, taskCoreMutations, taskDeletions)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task core Tool mutation handler bootstrap error: %w", err)
	}
	taskBatchWriteHandlers, err := toolruntimepmtools.NewTaskBatchWriteHandlers(taskBatchMutations)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task batch Tool mutation handler bootstrap error: %w", err)
	}
	qualityReads, err := quality.NewBugReadService(qualitygorm.NewBugReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("quality Tool read bootstrap error: %w", err)
	}
	qualityReadHandlers, err := toolruntimepmtools.NewQualityReadHandlers(qualityReads)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("quality Tool read handler bootstrap error: %w", err)
	}
	qualityMutationRepository, err := qualitygorm.NewMutationRepository(db, func(tx *gorm.DB) (requirement.MutationScopeResolver, error) {
		return requirement.NewMutationScopeService(requirementgorm.NewMutationScopeRepository(tx))
	}, func(tx *gorm.DB) (quality.BugMutationEffectSink, error) {
		return qualityToolMutationEffects{tx: tx, config: runtimeConfig.Notification}, nil
	})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("quality Tool mutation repository bootstrap error: %w", err)
	}
	qualityMutations, err := quality.NewBugMutationService(qualityMutationRepository)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("quality Tool mutation bootstrap error: %w", err)
	}
	qualityWriteHandlers, err := toolruntimepmtools.NewQualityWriteHandlers(qualityMutations)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("quality Tool mutation handler bootstrap error: %w", err)
	}
	uploadHandler, err := assembleQualityUploadTool(db, runtimeConfig.ObjectStorage)
	if err != nil {
		return ToolRuntimeDependencies{}, err
	}
	qualityWriteHandlers[toolruntimepmtools.QualityMaterialUpload] = uploadHandler
	audits, err := audit.NewReadService(auditgorm.NewReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("audit Tool read bootstrap error: %w", err)
	}
	identities := identity.NewAccessService(identitygorm.NewAccessRepository(db), nil)
	creatorPolicy, err := task.NewCreatorPolicyService(taskgorm.NewCreatorPolicyRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task creator policy bootstrap error: %w", err)
	}
	taskObjectActions, err := toolruntimepmtools.NewTaskObjectActionAuthorizer(
		tasks, creatorPolicy, projectgorm.NewFactUserResolver(db),
	)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task object Action bootstrap error: %w", err)
	}
	documentHandlers, documentObjectActions, err := assembleDocumentTools(db, identities, runtimeConfig)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("document Tools bootstrap error: %w", err)
	}
	businessActions, err := toolruntime.NewCatalogBusinessActionAuthorizer(identities, toolruntime.ObjectPolicyAuthorizers{"task.creator": taskObjectActions, "document.resource-access": documentObjectActions})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("Tool business Action bootstrap error: %w", err)
	}
	projectReadHandlers, err := toolruntimepmtools.NewProjectReadHandlers(toolruntimepmtools.ProjectReadDependencies{
		Projects:     projectReadQueries{ReadService: projects, ListService: projectLists, ToolReadService: projectToolReads},
		Requirements: requirementLists,
		Tasks:        taskReads,
		Quality:      qualityReads,
		Audits:       audits,
		Identities:   identities,
	})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project Tool read handler bootstrap error: %w", err)
	}
	directory, err := identity.NewToolDirectoryService(identitygorm.NewToolDirectoryRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("identity Tool directory bootstrap error: %w", err)
	}
	accessReadHandlers, err := toolruntimepmtools.NewAccessReadHandlers(directory)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("access Tool read handler bootstrap error: %w", err)
	}
	workflowAuxReadHandlers, err := toolruntimepmtools.NewWorkflowAuxReadHandlers(toolruntimepmtools.WorkflowAuxReadDependencies{
		Projects: projects, Requirements: requirements, Workflow: workflowAuxReads,
		Materials: requirement.NewMaterialService(requirementgorm.NewMaterialRepository(db)),
		Reports:   quality.NewTestReportService(qualitygorm.NewTestReportRepository(db)),
	})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("workflow auxiliary Tool handler bootstrap error: %w", err)
	}
	taskTodos, err := task.NewTodoReadService(taskRepository)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task todo Tool read bootstrap error: %w", err)
	}
	bugTodos, err := quality.NewBugTodoService(qualitygorm.NewBugReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("quality todo Tool read bootstrap error: %w", err)
	}
	workflowTodos, err := workflow.NewTodoReadService(workflowgorm.NewReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("workflow todo Tool read bootstrap error: %w", err)
	}
	aiRuns, err := ai.NewRunReadService(aigorm.NewRunReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("AI run Tool read bootstrap error: %w", err)
	}
	workbenchReads, err := workbench.NewToolReadService(workbench.ToolReadDependencies{
		Tasks: taskTodos, Bugs: bugTodos, Workflows: workflowTodos, AI: aiRuns,
		Identities: directory, Projects: projects, Requirements: requirements,
	})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("workbench Tool read bootstrap error: %w", err)
	}
	workbenchReadHandlers, err := toolruntimepmtools.NewWorkbenchReadHandlers(workbenchReads)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("workbench Tool handler bootstrap error: %w", err)
	}
	biTasks, err := task.NewBIReadService(taskgorm.NewBIReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("task BI read bootstrap error: %w", err)
	}
	biBugs, err := quality.NewBIReadService(qualitygorm.NewBIReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("quality BI read bootstrap error: %w", err)
	}
	biProjects, err := project.NewBIReadService(projectgorm.NewBIReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("project BI read bootstrap error: %w", err)
	}
	biRequirements, err := requirement.NewBIReadService(requirementgorm.NewBIReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("requirement BI read bootstrap error: %w", err)
	}
	biIdentities, err := identity.NewBIReadService(identitygorm.NewBIReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("identity BI read bootstrap error: %w", err)
	}
	biConfiguration, err := workbench.NewBIConfigurationReadService(workbenchgorm.NewBIConfigurationReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("BI configuration read bootstrap error: %w", err)
	}
	biWorkflows, err := workflow.NewBIReadService(workflowgorm.NewBIReadRepository(db))
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("workflow BI read bootstrap error: %w", err)
	}
	biBackend, err := biscreen.NewBIScreenBackend(workbench.BIScreenReadDependencies{
		Tasks: biTasks, Quality: biBugs, Projects: biProjects, Requirements: biRequirements,
		Identities: biIdentities, Configuration: biConfiguration, Workflows: biWorkflows,
	})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("BI screen backend bootstrap error: %w", err)
	}
	biScreen, err := workbench.NewBIScreenService(biBackend)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("BI screen service bootstrap error: %w", err)
	}
	biScreenHandlers, err := toolruntimepmtools.NewBIScreenReadHandlers(biScreen)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("BI screen Tool handler bootstrap error: %w", err)
	}
	handlers, activeNames, err := mergePMToolHandlerGroups(
		pmToolHandlerGroup{names: append(append(append(toolruntimepmtools.DocumentReadHandlerNames(), toolruntimepmtools.DocumentWriteHandlerNames()...), toolruntimepmtools.DocumentAccessReadHandlerNames()...), toolruntimepmtools.DocumentAccessWriteHandlerNames()...), handlers: documentHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.ReadHandlerNames(), handlers: baseHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.ProjectListHandlerNames(), handlers: projectListHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.ProjectAccessHandlerNames(), handlers: projectAccessHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.TaskReadHandlerNames(), handlers: taskReadHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.TaskClientOptionHandlerNames(), handlers: taskClientOptionHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.TaskTestingReadHandlerNames(), handlers: taskTestingReadHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.TaskTestingWriteHandlerNames(), handlers: taskTestingWriteHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.TaskCollaborationWriteHandlerNames(), handlers: taskCollaborationWriteHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.TaskCoreWriteHandlerNames(), handlers: taskCoreWriteHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.TaskBatchWriteHandlerNames(), handlers: taskBatchWriteHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.QualityReadHandlerNames(), handlers: qualityReadHandlers},
		pmToolHandlerGroup{names: append(toolruntimepmtools.QualityWriteHandlerNames(), toolruntimepmtools.QualityMaterialUpload), handlers: qualityWriteHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.ProjectReadHandlerNames(), handlers: projectReadHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.ProjectWriteHandlerNames(), handlers: projectWriteHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.RequirementWriteHandlerNames(), handlers: requirementWriteHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.AccessReadHandlerNames(), handlers: accessReadHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.WorkbenchReadHandlerNames(), handlers: workbenchReadHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.BIScreenReadHandlerNames(), handlers: biScreenHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.WorkflowAuxReadHandlerNames(), handlers: workflowAuxReadHandlers},
		pmToolHandlerGroup{names: toolruntimepmtools.WorkflowWriteHandlerNames(), handlers: workflowWriteHandlers},
	)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("PM Tool handler composition error: %w", err)
	}
	catalog, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{
		ExecutableToolNames: activeNames,
	})
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("PM Tool catalog bootstrap error: %w", err)
	}
	publicBaseURL := runtimeConfig.PublicBaseURL
	if strings.TrimSpace(publicBaseURL) == "" {
		publicBaseURL = "http://localhost"
	}
	oauthConfiguration, err := toolruntime.NewOAuthServerConfiguration(publicBaseURL)
	if err != nil {
		return ToolRuntimeDependencies{}, fmt.Errorf("external OAuth configuration error: %w", err)
	}
	return ToolRuntimeDependencies{
		Catalog: catalog, Handlers: handlers, BIScreen: biScreen,
		MCPAuthorizationURL: mcpAuthorizationURL(publicBaseURL),
		OAuthConfiguration:  oauthConfiguration,
		BusinessActions:     businessActions,
	}, nil
}

func mcpAuthorizationURL(publicBaseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(publicBaseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.Path = "/workspace/mcp-cli/authorize"
	parsed.RawPath = ""
	parsed.RawQuery = url.Values{"channel": {string(toolruntime.DeliveryMCP)}}.Encode()
	parsed.Fragment = ""
	return parsed.String()
}

// projectReadQueries composes only public project-domain services. It does not
// expose GORM records to the protocol adapter or let one domain read another
// domain's tables.
type projectReadQueries struct {
	*project.ReadService
	*project.ListService
	*project.ToolReadService
}

// projectToolAccessUserResolver adapts the project AccessService into the
// pmtools ProjectAccessUserResolver port: it resolves the tool-runtime
// principal (PM-internal users.id) into the fact user ID project data keys
// on, then delegates filter building to the domain access service.
type projectToolAccessUserResolver struct {
	access   project.AccessFilterBuilder
	resolver *projectgorm.FactUserResolver
}

func (r projectToolAccessUserResolver) FactUserID(ctx context.Context, principalID uint) (uint, error) {
	return r.resolver.FactUserID(ctx, principalID)
}

type pmToolHandlerGroup struct {
	names    []string
	handlers map[string]toolruntime.ToolHandler
}

func mergePMToolHandlerGroups(groups ...pmToolHandlerGroup) (map[string]toolruntime.ToolHandler, []string, error) {
	handlers := make(map[string]toolruntime.ToolHandler)
	names := make([]string, 0)
	for _, group := range groups {
		declared := make(map[string]struct{}, len(group.names))
		for _, name := range group.names {
			if name == "" {
				return nil, nil, fmt.Errorf("handler name is required")
			}
			if _, duplicate := declared[name]; duplicate {
				return nil, nil, fmt.Errorf("handler %s is duplicated within its group", name)
			}
			declared[name] = struct{}{}
			handler, exists := group.handlers[name]
			if !exists || handler == nil {
				return nil, nil, fmt.Errorf("handler %s is not implemented", name)
			}
			if _, duplicate := handlers[name]; duplicate {
				return nil, nil, fmt.Errorf("handler %s is registered more than once", name)
			}
			handlers[name] = handler
			names = append(names, name)
		}
		for name := range group.handlers {
			if _, exists := declared[name]; !exists {
				return nil, nil, fmt.Errorf("handler %s is not declared by its factory", name)
			}
		}
	}
	return handlers, names, nil
}

func assembleToolRuntime(db *gorm.DB, dependencies ToolRuntimeDependencies) (toolRuntimeRoutes, error) {
	if db == nil {
		return toolRuntimeRoutes{}, fmt.Errorf("tool runtime database is required")
	}
	grantRepository := toolruntimegorm.NewGrantRepository(db)
	authorizationRequestRepository := toolruntimegorm.NewAuthorizationRequestRepository(db)
	workspaceConfigurations := toolruntimegorm.NewWorkspaceConfigurationRepository(db)
	grantAuthorizer, err := toolruntime.NewGrantAuthorizer(grantRepository, time.Now, workspaceConfigurations)
	if err != nil {
		return toolRuntimeRoutes{}, fmt.Errorf("external tooling grant bootstrap error: %w", err)
	}
	changeSets, err := toolruntime.NewChangeSetService(
		toolruntimegorm.NewChangeSetRepository(db), toolruntime.DefaultChangeSetPolicy(),
	)
	if err != nil {
		return toolRuntimeRoutes{}, fmt.Errorf("Tool ChangeSet bootstrap error: %w", err)
	}
	application, err := toolruntime.NewApplication(
		dependencies.Catalog, grantAuthorizer, dependencies.Handlers,
		toolruntime.WithChangeSetService(changeSets),
		toolruntime.WithBusinessActionAuthorizer(dependencies.BusinessActions),
	)
	if err != nil {
		return toolRuntimeRoutes{}, fmt.Errorf("tool runtime bootstrap error: %w", err)
	}
	authorization, err := toolruntime.NewAuthorizationService(
		dependencies.Catalog, authorizationRequestRepository, grantRepository,
		workspaceConfigurations,
	)
	if err != nil {
		return toolRuntimeRoutes{}, fmt.Errorf("external tooling authorization bootstrap error: %w", err)
	}
	memberships, err := identity.NewWorkspaceMembershipService(identitygorm.NewWorkspaceMembershipRepository(db))
	if err != nil {
		return toolRuntimeRoutes{}, fmt.Errorf("external tooling membership bootstrap error: %w", err)
	}
	credentials, err := toolruntime.NewCredentialService(
		grantRepository,
		authorizationRequestRepository,
		toolruntimegorm.NewCredentialRepository(db),
		memberships,
		toolruntime.DefaultCredentialPolicy(),
	)
	if err != nil {
		return toolRuntimeRoutes{}, fmt.Errorf("external tooling credential bootstrap error: %w", err)
	}
	oauth, err := toolruntime.NewOAuthService(
		authorization, credentials, toolruntimegorm.NewOAuthClientRepository(db), dependencies.OAuthConfiguration,
	)
	if err != nil {
		return toolRuntimeRoutes{}, fmt.Errorf("external OAuth bootstrap error: %w", err)
	}
	deviceAuthorizations, err := toolruntime.NewDeviceAuthorizationService(
		authorization, credentials, toolruntimegorm.NewDeviceAuthorizationRepository(db),
		dependencies.OAuthConfiguration, toolruntime.DefaultDeviceAuthorizationPolicy(),
	)
	if err != nil {
		return toolRuntimeRoutes{}, fmt.Errorf("CLI device authorization bootstrap error: %w", err)
	}
	credentialHandler := toolruntimehttp.NewCredentialHandler(
		credentials, toolruntimehttp.ManagementPrincipalResolverFunc(resolveToolManagementPrincipal), oauth,
	).WithDeviceAuthorization(deviceAuthorizations).WithAuthorizationHistory(toolruntime.NewAuthorizationHistoryService(
		toolruntimegorm.NewAuthorizationHistoryRepository(db),
		toolAuthorizationHistoryAccess{
			actions: identity.NewAccessService(identitygorm.NewAccessRepository(db), nil),
			users:   identitygorm.NewUserDisplayRepository(db),
		},
	))
	oauthHandler := toolruntimehttp.NewOAuthHandler(oauth)
	verifier := dependencies.ExternalTokenVerifier
	if verifier == nil {
		verifier = credentials
	}

	cliResolver, err := httpapi.NewExternalToolAuthorizationResolver(
		verifier, toolruntime.DeliveryCLI, toolruntime.AudienceCLI,
	)
	if err != nil {
		return toolRuntimeRoutes{}, fmt.Errorf("CLI Tool authorization bootstrap error: %w", err)
	}
	mcpResolver, err := httpapi.NewExternalToolAuthorizationResolver(
		verifier, toolruntime.DeliveryMCP, toolruntime.AudienceMCP,
	)
	if err != nil {
		return toolRuntimeRoutes{}, fmt.Errorf("MCP authorization bootstrap error: %w", err)
	}
	cliHandler := toolruntimehttp.NewHandler(application, cliResolver)
	mcpHandler := toolruntimemcp.NewHandler(
		application, mcpResolver, dependencies.MCPAuthorizationURL, dependencies.OAuthConfiguration.ProtectedResourceMetadata,
	)
	routes := toolRuntimeRoutes{}
	routes.mountManagement = func(group *gin.RouterGroup) {
		toolruntimehttp.NewManagementHandler(
			authorization, toolruntimehttp.ManagementPrincipalResolverFunc(resolveToolManagementPrincipal), oauth,
		).RegisterRoutes(group)
		credentialHandler.RegisterManagementRoutes(group)
	}
	routes.mountExternal = func(group *gin.RouterGroup) {
		credentialHandler.RegisterPublicRoutes(group.Group("/tool-auth"))
		oauthHandler.RegisterPublicRoutes(group)
		cliHandler.RegisterRoutes(group.Group("/tool-runtime"))
		mcpHandler.RegisterRoutes(group)
	}
	routes.mountDiscovery = oauthHandler.RegisterDiscoveryRoutes
	return routes, nil
}
