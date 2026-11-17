package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type biScreenBackendStub struct {
	query BIScreenQuery
	err   error
}

func (stub *biScreenBackendStub) BuildBIScreen(_ context.Context, query BIScreenQuery) (BIScreenResult, error) {
	stub.query = query
	return BIScreenResult{Period: query.Period, WorkspaceID: query.WorkspaceID, Compatibility: map[string]any{"summary": map[string]any{}}}, stub.err
}

func TestBIScreenServiceNormalizesProtocolNeutralQuery(t *testing.T) {
	backend := &biScreenBackendStub{}
	service, err := NewBIScreenService(backend)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Get(context.Background(), BIScreenQuery{WorkspaceID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if backend.query.Period != "week" || backend.query.Location != time.UTC || backend.query.Now.IsZero() {
		t.Fatalf("query=%+v", backend.query)
	}
	for _, query := range []BIScreenQuery{
		{}, {WorkspaceID: 7, Period: "quarter"}, {WorkspaceID: 7, ProjectSetIDs: []uint{1, 1}},
	} {
		if _, err := service.Get(context.Background(), query); !errors.Is(err, ErrBIScreenQueryInvalid) {
			t.Fatalf("query=%+v err=%v", query, err)
		}
	}
}

func TestBIScreenResultMergesCompatibilityWithoutCollision(t *testing.T) {
	payload, err := json.Marshal(BIScreenResult{Period: "week", WorkspaceID: 7, Compatibility: map[string]any{"summary": map[string]any{"active_projects": 1}}})
	if err != nil || !json.Valid(payload) {
		t.Fatalf("payload=%s err=%v", payload, err)
	}
	if _, err := json.Marshal(BIScreenResult{Period: "week", Compatibility: map[string]any{"period": "month"}}); err == nil {
		t.Fatal("compatibility collision was accepted")
	}
}
