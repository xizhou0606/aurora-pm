package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

type readinessTestStore struct {
	checks int
	err    error
}

func (*readinessTestStore) PutObject(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (s *readinessTestStore) CheckReady(context.Context) error {
	s.checks++
	return s.err
}

type writeOnlyReadinessTestStore struct{}

func (*writeOnlyReadinessTestStore) PutObject(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func TestServerReadinessAllowsDisabledObjectStorage(t *testing.T) {
	app := newTestApp(t, nil)
	app.engine = newEngine(app.cfg, app.db, nil, errors.New("ignored while disabled"))

	response := app.request(t, http.MethodGet, "/health/ready", "", 0, nil)
	requireStatus(t, response, http.StatusOK)
}

func TestServerReadinessCachesSuccessfulObjectStorageCheck(t *testing.T) {
	app := newTestApp(t, nil)
	app.cfg.ObjectStorage.Enabled = true
	store := &readinessTestStore{}
	app.engine = newEngine(app.cfg, app.db, store, nil)

	for range 2 {
		response := app.request(t, http.MethodGet, "/health/ready", "", 0, nil)
		requireStatus(t, response, http.StatusOK)
	}
	if store.checks != 1 {
		t.Fatalf("readiness checks=%d, want 1 cached check", store.checks)
	}
}

func TestServerReadinessRejectsUnavailableObjectStorage(t *testing.T) {
	for _, test := range []struct {
		name       string
		store      materialObjectStore
		storeErr   error
		wantChecks int
	}{
		{
			name: "readiness check failed", store: &readinessTestStore{err: errors.New("bucket unavailable")},
			wantChecks: 1,
		},
		{name: "checker missing", store: &writeOnlyReadinessTestStore{}},
		{name: "store initialization failed", store: &readinessTestStore{}, storeErr: errors.New("invalid credentials")},
		{name: "store missing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := newTestApp(t, nil)
			app.cfg.ObjectStorage.Enabled = true
			app.engine = newEngine(app.cfg, app.db, test.store, test.storeErr)

			response := app.request(t, http.MethodGet, "/health/ready", "", 0, nil)
			requireStatus(t, response, http.StatusServiceUnavailable)
			var envelope map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode readiness response: %v", err)
			}
			if envelope["code"] != "OBJECT_STORAGE_UNAVAILABLE" {
				t.Fatalf("readiness response=%v", envelope)
			}
			if store, ok := test.store.(*readinessTestStore); ok && store.checks != test.wantChecks {
				t.Fatalf("readiness checks=%d, want %d", store.checks, test.wantChecks)
			}
		})
	}
}
