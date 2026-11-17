package documenthttp

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"net/http"
	"net/http/httptest"
	"testing"
)

type displayReaderStub struct {
	calls int
	ids   []uint
}

func (r *displayReaderStub) ReadUserDisplays(_ context.Context, ids []uint) (map[uint]identity.UserDisplay, error) {
	r.calls++
	r.ids = ids
	return map[uint]identity.UserDisplay{109: {ID: 109, Name: "Administrator"}}, nil
}
func TestAuthorizedResponseBatchesHistoricalAuthors(t *testing.T) {
	reader := &displayReaderStub{}
	handler := &Handler{displays: reader}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	handler.writeData(c, http.StatusOK, document.CommentThreadPage{Items: []document.CommentThread{{Comments: []document.Comment{{AuthorUserID: 109, CanEdit: false}, {AuthorUserID: 109, CanDelete: false}}}}})
	if reader.calls != 1 || len(reader.ids) != 1 || reader.ids[0] != 109 {
		t.Fatalf("unexpected profile query: %+v", reader)
	}
	var response struct {
		Data document.CommentThreadPage `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	for _, comment := range response.Data.Items[0].Comments {
		if comment.Author == nil || comment.Author.Name != "Administrator" || comment.CanEdit || comment.CanDelete {
			t.Fatalf("comment=%+v", comment)
		}
	}
}

func TestTreeReturnsCreatorProfileForThirdPartyViewer(t *testing.T) {
	reader := &displayReaderStub{}
	handler := &Handler{displays: reader}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	handler.writeData(c, http.StatusOK, document.Tree{Nodes: []document.Node{{ID: 1, CreatedBy: 109}, {ID: 2, CreatedBy: 109}}})
	var response struct {
		Data document.Tree `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if reader.calls != 1 || len(reader.ids) != 1 {
		t.Fatalf("unbatched lookup: %+v", reader)
	}
	for _, node := range response.Data.Nodes {
		if node.Creator == nil || node.Creator.ID != 109 || node.Creator.Name != "Administrator" {
			t.Fatalf("creator: %+v", node.Creator)
		}
	}
}
