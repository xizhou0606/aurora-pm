package resourceaccesshttp

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
)

type policyRuleRequest struct {
	SubjectKind        identity.SubjectKind `json:"subject_kind"`
	SubjectID          string               `json:"subject_id"`
	Level              string               `json:"level"`
	IncludeDescendants bool                 `json:"include_descendants,omitempty"`
}

type replacePolicyRequest struct {
	Mode                   string              `json:"mode"`
	WorkspaceLevel         *string             `json:"workspace_level"`
	Rules                  []policyRuleRequest `json:"rules"`
	ExpectedPolicyRevision *uint64             `json:"expected_policy_revision"`
}

func (request replacePolicyRequest) rules() []resourceaccess.Rule {
	result := make([]resourceaccess.Rule, 0, len(request.Rules))
	for _, rule := range request.Rules {
		result = append(result, resourceaccess.Rule{
			SubjectKind: rule.SubjectKind, SubjectID: rule.SubjectID,
			Level: resourceaccess.Level(rule.Level), IncludeDescendants: rule.IncludeDescendants,
		})
	}
	return result
}

func levelPointer(value *string) *resourceaccess.Level {
	if value == nil {
		return nil
	}
	level := resourceaccess.Level(*value)
	return &level
}
