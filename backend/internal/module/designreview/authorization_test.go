package designreview

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
)

func TestServiceEndpointGroupsFailClosedWithoutTheirAction(t *testing.T) {
	service := &Service{
		actions:      &effectiveActionReaderStub{},
		availability: Availability{ObjectStorage: true, VisualModel: true, Figma: true},
	}
	region := Region{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.2}
	tests := map[string]func() error{
		"requirements": func() error { _, err := service.ListRequirements(context.Background(), 7, testActor(42)); return err },
		"reviews": func() error {
			_, err := service.ListReviews(context.Background(), 7, testActor(42), ReviewListFilter{})
			return err
		},
		"review detail": func() error {
			_, err := service.GetReview(context.Background(), 7, testActor(42), "11111111-1111-4111-8111-111111111111")
			return err
		},
		"create": func() error {
			_, err := service.CreateReview(context.Background(), 7, testActor(42), "User", CreateReviewInput{})
			return err
		},
		"figma preview": func() error {
			_, err := service.PreviewFigma(context.Background(), testActor(42), "https://www.figma.com/design/file?node-id=1-2")
			return err
		},
		"dom normalize": func() error {
			_, err := service.NormalizeDOM(context.Background(), 7, testActor(42), "https://www.figma.com/design/file?node-id=1-2", []byte(`{}`))
			return err
		},
		"asset upload": func() error {
			_, err := service.UploadAsset(context.Background(), 7, testActor(42), "review", "implementation_structure", "application/json", bytes.NewReader([]byte(`{}`)))
			return err
		},
		"asset download": func() error {
			_, _, err := service.Asset(context.Background(), 7, testActor(42), "review", "asset")
			return err
		},
		"delete review": func() error { return service.DeleteReview(context.Background(), 7, testActor(42), "review") },
		"create difference": func() error {
			_, err := service.CreateDifference(context.Background(), 7, testActor(42), "review", DifferenceInput{})
			return err
		},
		"update difference": func() error {
			_, err := service.UpdateDifference(context.Background(), 7, testActor(42), "review", "difference", DifferenceInput{})
			return err
		},
		"delete difference": func() error {
			return service.DeleteDifference(context.Background(), 7, testActor(42), "review", "difference")
		},
		"transition difference": func() error {
			_, err := service.TransitionDifference(context.Background(), 7, testActor(42), "review", "difference", "resolve", "")
			return err
		},
		"queue analysis":   func() error { return service.QueueAnalysis(context.Background(), 7, testActor(42), "review") },
		"retry analysis":   func() error { return service.RetryAnalysis(context.Background(), 7, testActor(42), "review") },
		"abandon analysis": func() error { return service.AbandonAnalysis(context.Background(), 7, testActor(42), "review") },
		"create deep review": func() error {
			_, err := service.CreateDeepReview(context.Background(), 7, testActor(42), "review", region, region, "", nil)
			return err
		},
		"transition deep review": func() error {
			return service.TransitionDeepReview(context.Background(), 7, testActor(42), "review", "job", "cancel")
		},
		"resolve recheck match": func() error {
			return service.ResolveRecheckMatch(context.Background(), 7, testActor(42), "review", "11111111-1111-4111-8111-111111111111", "new")
		},
		"override recheck verification": func() error {
			return service.OverrideRecheckVerification(context.Background(), 7, testActor(42), "review", "11111111-1111-4111-8111-111111111111", nil)
		},
		"list collectors": func() error { _, err := service.ListCollectors(context.Background(), 7, testActor(42)); return err },
		"pairing code":    func() error { _, err := service.CreatePairingCode(context.Background(), 7, testActor(42)); return err },
		"revoke collector": func() error {
			return service.RevokeCollector(context.Background(), 7, testActor(42), "collector")
		},
		"create capture": func() error {
			_, err := service.CreateCapture(context.Background(), 7, testActor(42), "collector", "ios", "device")
			return err
		},
		"list captures": func() error { _, err := service.ListCaptures(context.Background(), 7, testActor(42)); return err },
		"capture asset": func() error {
			_, _, err := service.CaptureAsset(context.Background(), 7, testActor(42), "capture")
			return err
		},
		"consume capture": func() error {
			_, err := service.ConsumeCapture(context.Background(), 7, testActor(42), "capture")
			return err
		},
		"delete capture": func() error { return service.DeleteCapture(context.Background(), 7, testActor(42), "capture") },
	}
	for name, operation := range tests {
		t.Run(name, func(t *testing.T) {
			if err := operation(); !errors.Is(err, ErrForbidden) {
				t.Fatalf("error=%v, want forbidden", err)
			}
		})
	}
}

func TestMutationsAlsoRequireReviewPageAccess(t *testing.T) {
	tests := []struct {
		name   string
		action accesscatalog.ActionCode
		call   func(*Service) error
	}{
		{name: "create", action: accesscatalog.ActionDesignReviewsReviewsCreate, call: func(service *Service) error {
			_, err := service.CreateReview(context.Background(), 7, testActor(42), "User", CreateReviewInput{})
			return err
		}},
		{name: "difference", action: accesscatalog.ActionDesignReviewsDifferencesManage, call: func(service *Service) error {
			_, err := service.CreateDifference(context.Background(), 7, testActor(42), "review", DifferenceInput{Category: "layout", Severity: "major", Title: "title", Description: "description", Recommendation: "recommendation"})
			return err
		}},
		{name: "initial analysis", action: accesscatalog.ActionDesignReviewsReviewsCreate, call: func(service *Service) error {
			return service.QueueAnalysis(context.Background(), 7, testActor(42), "review")
		}},
		{name: "analysis retry", action: accesscatalog.ActionDesignReviewsAnalysisManage, call: func(service *Service) error {
			return service.RetryAnalysis(context.Background(), 7, testActor(42), "review")
		}},
		{name: "delete", action: accesscatalog.ActionDesignReviewsReviewsDelete, call: func(service *Service) error {
			return service.DeleteReview(context.Background(), 7, testActor(42), "review")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &Service{
				actions:      &effectiveActionReaderStub{codes: []string{string(test.action)}},
				availability: Availability{ObjectStorage: true, VisualModel: true, Figma: true},
			}
			if err := test.call(service); !errors.Is(err, ErrForbidden) {
				t.Fatalf("error=%v, want forbidden without reviews.view", err)
			}
		})
	}
}
