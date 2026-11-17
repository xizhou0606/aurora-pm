package designreview

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	domain "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func TestRetryableJobError(t *testing.T) {
	for name, test := range map[string]struct {
		err       error
		retryable bool
	}{
		"network":              {err: errors.New("connection reset"), retryable: true},
		"request timeout":      {err: runtimeHTTPError{status: http.StatusRequestTimeout}, retryable: true},
		"rate limited":         {err: runtimeHTTPError{status: http.StatusTooManyRequests}, retryable: true},
		"runtime unavailable":  {err: runtimeHTTPError{status: http.StatusServiceUnavailable}, retryable: true},
		"runtime bad request":  {err: runtimeHTTPError{status: http.StatusBadRequest}, retryable: false},
		"runtime unauthorized": {err: runtimeHTTPError{status: http.StatusUnauthorized}, retryable: false},
		"invalid contract":     {err: domain.ErrInvalid, retryable: false},
		"missing evidence":     {err: domain.ErrEvidenceRequired, retryable: false},
		"missing review":       {err: domain.ErrNotFound, retryable: false},
		"stale completion":     {err: domain.ErrConflict, retryable: false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := retryableJobError(test.err); got != test.retryable {
				t.Fatalf("retryable=%v, want %v", got, test.retryable)
			}
		})
	}
}

func TestRuntimeHTTPErrorIncludesSafeValidationContext(t *testing.T) {
	err := newRuntimeHTTPError(http.StatusBadRequest, []byte(`{"error":"invalid design review request","details":{"formErrors":[],"fieldErrors":{"implementationStructure":["invalid bounds"],"scope":["invalid region"]}}}`))
	message := err.Error()
	for _, expected := range []string{"design review runtime returned 400", "invalid design review request", "implementationStructure", "scope"} {
		if !strings.Contains(message, expected) {
			t.Fatalf("error %q does not contain %q", message, expected)
		}
	}
}

func TestObjectCleanupRetryDelayIsBounded(t *testing.T) {
	for _, test := range []struct {
		attempts int
		want     time.Duration
	}{
		{attempts: 0, want: time.Minute},
		{attempts: 1, want: time.Minute},
		{attempts: 3, want: 4 * time.Minute},
		{attempts: 9, want: 32 * time.Minute},
	} {
		if got := objectCleanupRetryDelay(test.attempts); got != test.want {
			t.Fatalf("attempts=%d delay=%s want=%s", test.attempts, got, test.want)
		}
	}
}
