package documenthttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

func TestCommentThreadPageHTTPEnvelopePreservesArrayContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		page       document.CommentThreadPage
		wantBodies []string
	}{
		{
			name:       "empty page",
			page:       document.CommentThreadPage{Items: []document.CommentThread{}, Total: 0},
			wantBodies: []string{`"items":[]`, `"total":0`},
		},
		{
			name: "thread without mentions",
			page: document.CommentThreadPage{
				Items: []document.CommentThread{{
					ID: 31, WorkspaceID: 7, DocumentID: 11,
					Anchor: document.CommentAnchor{Kind: document.CommentAnchorDocument, State: document.CommentAnchorStateDocument},
					Status: document.CommentThreadOpen, CreatedBy: 5,
					Comments: []document.Comment{{
						ID: 41, WorkspaceID: 7, DocumentID: 11, ThreadID: 31,
						AuthorUserID: 5, Body: `{"type":"doc"}`, MentionedUserIDs: []uint{},
					}},
				}},
				Total: 1,
			},
			wantBodies: []string{`"items":[`, `"comments":[`, `"mentioned_user_ids":[]`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			writeData(context, http.StatusOK, test.page)
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			for _, want := range test.wantBodies {
				if !strings.Contains(response.Body.String(), want) {
					t.Fatalf("body=%s missing=%s", response.Body.String(), want)
				}
			}
		})
	}
}
