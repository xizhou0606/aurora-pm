package designreview

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
)

func testActor(userID uint) ActorIdentity {
	return ActorIdentity{PrincipalUserID: userID, FactUserID: userID}
}

type effectiveActionReaderStub struct {
	codes []string
	err   error
	calls int
	users []uint
}

func (stub *effectiveActionReaderStub) EffectiveActionCodes(_ context.Context, userID uint) ([]string, error) {
	stub.calls++
	stub.users = append(stub.users, userID)
	return append([]string(nil), stub.codes...), stub.err
}

func TestCapabilitiesUsePrincipalUserIDForRBAC(t *testing.T) {
	actions := &effectiveActionReaderStub{codes: []string{string(accesscatalog.ActionDesignReviewsReviewsView)}}
	service := &Service{actions: actions}

	capabilities := service.Capabilities(context.Background(), ActorIdentity{PrincipalUserID: 109, FactUserID: 1})

	if !capabilities.CanView || !reflect.DeepEqual(actions.users, []uint{109}) {
		t.Fatalf("capabilities=%+v action users=%v", capabilities, actions.users)
	}
}

func TestCapabilitiesResolveOnlyEffectiveDesignReviewActions(t *testing.T) {
	actions := &effectiveActionReaderStub{codes: []string{
		string(accesscatalog.ActionDesignReviewsReviewsView),
		string(accesscatalog.ActionDesignReviewsDifferencesManage),
		"pm.unrelated.action",
	}}
	service := &Service{actions: actions, availability: Availability{ObjectStorage: true, VisualModel: true, Figma: true}}

	capabilities := service.Capabilities(context.Background(), testActor(42))

	if actions.calls != 1 || !capabilities.CanView || !capabilities.CanManageDifferences || capabilities.CanCreate ||
		capabilities.CanManageAnalysis || capabilities.CanDelete || capabilities.CanManageCollectors {
		t.Fatalf("capabilities=%+v calls=%d", capabilities, actions.calls)
	}
	wantActions := []string{
		string(accesscatalog.ActionDesignReviewsReviewsView),
		string(accesscatalog.ActionDesignReviewsDifferencesManage),
	}
	if !reflect.DeepEqual(capabilities.AllowedActions, wantActions) {
		t.Fatalf("allowed_actions=%v want=%v", capabilities.AllowedActions, wantActions)
	}
	if !capabilities.CreationAvailable || !capabilities.AnalysisAvailable || !capabilities.FigmaPreviewAvailable || !capabilities.ObjectStorageAvailable {
		t.Fatalf("environment capabilities=%+v", capabilities)
	}
}

func TestCapabilitiesFailClosedWhenActionReaderIsUnavailable(t *testing.T) {
	for name, reader := range map[string]EffectiveActionReader{
		"missing": nil,
		"failed":  &effectiveActionReaderStub{err: errors.New("permission lookup failed")},
	} {
		t.Run(name, func(t *testing.T) {
			service := &Service{actions: reader, availability: Availability{ObjectStorage: true, VisualModel: true, Figma: true}}
			capabilities := service.Capabilities(context.Background(), testActor(42))
			if capabilities.CanView || capabilities.CanCreate || capabilities.CanManageDifferences || capabilities.CanManageAnalysis ||
				capabilities.CanDelete || capabilities.CanManageCollectors || len(capabilities.AllowedActions) != 0 {
				t.Fatalf("capabilities must fail closed: %+v", capabilities)
			}
			if !capabilities.CreationAvailable || !capabilities.AnalysisAvailable {
				t.Fatalf("permission failure must not change environment availability: %+v", capabilities)
			}
		})
	}
}

func TestCapabilitiesKeepEnvironmentChecksIndependentFromPermissions(t *testing.T) {
	actions := &effectiveActionReaderStub{codes: []string{string(accesscatalog.ActionDesignReviewsReviewsCreate)}}
	service := &Service{actions: actions, availability: Availability{}}

	capabilities := service.Capabilities(context.Background(), testActor(42))

	if !capabilities.CanCreate || capabilities.CreationAvailable || capabilities.AnalysisAvailable || capabilities.FigmaPreviewAvailable || capabilities.ObjectStorageAvailable {
		t.Fatalf("capabilities=%+v", capabilities)
	}
	if len(capabilities.UnavailableReasons) != 3 {
		t.Fatalf("unavailable_reasons=%v", capabilities.UnavailableReasons)
	}
}
