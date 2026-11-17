package httpapi

import (
	"regexp"
	"testing"
)

func TestAPIErrorCodeRegistry(t *testing.T) {
	codes := []string{
		apiErrorAuthUpstreamUnavailable,
		apiErrorWorkspaceForbidden,
		apiErrorIdentityNotProvisioned,
		apiErrorPasswordResetRequired,
		apiErrorSessionExpired,
		apiErrorObjectStorageUnavailable,
		apiErrorObjectStorageUploadFailed,
		apiErrorPMUserRegistryUnavailable,
		apiErrorQualityBugBatchNotFound,
		apiErrorQualityBugBatchValidationFailed,
		apiErrorTaskBatchNotFound,
		apiErrorTaskBatchValidationFailed,
		apiErrorTaskChildrenBatchValidationFailed,
		apiErrorTaskBatchDescendantsRequired,
		apiErrorTaskDeleteForbidden,
		apiErrorRequirementCoverUnavailable,
		apiErrorFeishuCoverUnavailable,
		apiErrorRequirementStale,
		apiErrorWorkflowScheduleConfirmation,
		apiErrorWorkflowScheduleConflict,
		apiErrorScheduleSubjectStale,
		apiErrorBITeamMetricWorkflowInvalid,
		apiErrorBITeamMetricApprovalNodeInvalid,
		apiErrorBITeamMetricSettingsConflict,
		apiErrorBITeamScoreVersionConflict,
	}

	machineCode := regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if !machineCode.MatchString(code) {
			t.Fatalf("invalid API error code %q", code)
		}
		if _, exists := seen[code]; exists {
			t.Fatalf("duplicate API error code %q", code)
		}
		seen[code] = struct{}{}
	}
}
