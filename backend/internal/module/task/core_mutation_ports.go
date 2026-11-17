package task

import "context"

type CoreMutationScope struct {
	WorkspaceID        uint
	TeamID             uint
	ProjectSetID       uint
	ProjectCode        string
	PlanUnitID         uint
	RequirementID      uint
	RequirementVersion uint
}

type CoreMutationFacts interface {
	ResolveCoreMutationScope(context.Context, uint, uint) (CoreMutationScope, error)
	ValidateActiveTaskUsers(context.Context, uint, []uint) error
}

type CoreMutationEffect struct {
	PreviousStatus string
	WorkspaceID    uint
	PrincipalID    uint
	TeamID         uint
	ProjectSetID   uint
	PlanUnitID     uint
	TaskID         uint
	TaskKey        string
	Action         string
	Target         string
	Payload        map[string]any
	AssigneeID     *uint
	NotifyAssignee bool
}

type CoreMutationEffectSink interface {
	RecordCoreTaskMutation(context.Context, CoreMutationEffect) error
}

// TaskDependentReferenceFacts freezes the protocol-neutral identifiers that
// owning domains need before task-owned testing rows are deleted.
type TaskDependentReferenceFacts struct {
	WorkspaceID      uint
	TaskIDs          []uint
	TestExecutionIDs []uint
}

// TaskDependentCleaner participates in the task repository transaction through
// an infrastructure factory. Its contract contains no GORM types and lets
// owning domains clear task references without taskgorm importing their tables.
type TaskDependentCleaner interface {
	ClearTaskReferences(context.Context, TaskDependentReferenceFacts) error
}
