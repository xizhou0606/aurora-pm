package biscreen

type biTeamMetricSettingsDTO struct {
	Version                       int
	Configured                    bool
	Valid                         bool
	RequirementWorkflowTemplateID uint
	RequirementWorkflowTemplate   string
	RequirementWorkflowVersion    uint
	ApprovalNodeID                string
	ApprovalNodeName              string
	Options                       []biTeamMetricWorkflowOption
}

type biTeamMetricWorkflowOption struct {
	TemplateID       uint
	TemplateName     string
	PublishedVersion uint
	ApprovalNodes    []biTeamMetricApprovalNodeOption
}

type biTeamMetricApprovalNodeOption struct {
	NodeID   string
	NodeName string
}
