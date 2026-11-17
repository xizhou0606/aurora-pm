package requirementhttp

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPoolPage(t *testing.T) {
	for _, testCase := range []struct {
		query                 string
		wantLimit, wantOffset int
		wantAll               bool
	}{
		{query: "", wantLimit: 20, wantOffset: 0},
		{query: "?limit=all", wantAll: true},
		{query: "?limit=600&offset=-1", wantLimit: 500, wantOffset: 0},
	} {
		t.Run(testCase.query, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest("GET", "/requirements"+testCase.query, nil)
			limit, offset, all := PoolPage(context)
			if limit != testCase.wantLimit || offset != testCase.wantOffset || all != testCase.wantAll {
				t.Fatalf("PoolPage() = (%d, %d, %t), want (%d, %d, %t)", limit, offset, all, testCase.wantLimit, testCase.wantOffset, testCase.wantAll)
			}
		})
	}
}
