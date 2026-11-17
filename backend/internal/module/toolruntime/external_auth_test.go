package toolruntime

import "testing"

func TestValidateExternalAuthorizationRequiresExactServerOwnedFacts(t *testing.T) {
	valid := ExecutionAuthorization{
		PrincipalID: 7, WorkspaceID: 11, GrantID: 13, ClientID: "codex",
		Channel: DeliveryCLI, Audience: AudienceCLI,
	}
	if err := ValidateExternalAuthorization(valid, DeliveryCLI, AudienceCLI); err != nil {
		t.Fatal(err)
	}

	tests := map[string]ExecutionAuthorization{
		"principal": func() ExecutionAuthorization { value := valid; value.PrincipalID = 0; return value }(),
		"workspace": func() ExecutionAuthorization { value := valid; value.WorkspaceID = 0; return value }(),
		"grant":     func() ExecutionAuthorization { value := valid; value.GrantID = 0; return value }(),
		"client":    func() ExecutionAuthorization { value := valid; value.ClientID = " "; return value }(),
		"channel": func() ExecutionAuthorization {
			value := valid
			value.Channel, value.Audience = DeliveryMCP, AudienceMCP
			return value
		}(),
		"audience": func() ExecutionAuthorization { value := valid; value.Audience = AudienceMCP; return value }(),
	}
	for name, authorization := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateExternalAuthorization(authorization, DeliveryCLI, AudienceCLI); err == nil {
				t.Fatal("expected fail-closed validation")
			}
		})
	}
}

func TestValidateExternalAuthorizationRejectsMismatchedExpectedPair(t *testing.T) {
	authorization := ExecutionAuthorization{
		PrincipalID: 7, WorkspaceID: 11, GrantID: 13, ClientID: "codex",
		Channel: DeliveryMCP, Audience: AudienceMCP,
	}
	if err := ValidateExternalAuthorization(authorization, DeliveryMCP, AudienceCLI); err == nil {
		t.Fatal("expected invalid expected audience to fail closed")
	}
}
