package bitablehttp

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCommentsHTTPGuards(t *testing.T) {
	path := "/api/v1/platform/bitables/550e8400-e29b-41d4-a716-446655440000/tables/table/records/row/comments"
	for _, tc := range []struct {
		method, suffix, body string
		auth                 bool
		status               int
	}{
		{"GET", "", "", false, 403}, {"GET", "?limit=201", "", true, 400}, {"GET", "?offset=-1", "", true, 400},
		{"POST", "", `{"body":"text","operationId":"550e8400-e29b-41d4-a716-446655440000","author":{"id":2}}`, true, 400},
		{"POST", "", `{"body":"\n","operationId":"550e8400-e29b-41d4-a716-446655440000"}`, true, 400},
	} {
		r, _ := router(tc.auth)
		request := httptest.NewRequest(tc.method, path+tc.suffix, strings.NewReader(tc.body))
		response := httptest.NewRecorder()
		r.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("%s %s got %d", tc.method, tc.suffix, response.Code)
		}
	}
}
