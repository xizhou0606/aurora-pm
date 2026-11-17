package documenthttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

func TestWriteErrorIncludesLatestRevision(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	writeError(context, document.RevisionConflictError{LatestRevision: 7})
	if recorder.Code != 409 {
		t.Fatalf("status=%d", recorder.Code)
	}
	var payload struct {
		Code    string `json:"code"`
		Details struct {
			LatestRevision uint `json:"latest_revision"`
		} `json:"details"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != "DOCUMENT_REVISION_CONFLICT" || payload.Details.LatestRevision != 7 {
		t.Fatalf("payload=%+v", payload)
	}
}

func TestWriteErrorIncludesLatestArchiveImpact(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	writeError(context, document.ArchiveImpactChangedError{Latest: document.ArchiveImpact{
		NodeID: 9, DescendantCount: 4, DocumentCount: 2, RequiresConfirmation: true,
	}})
	if recorder.Code != 409 {
		t.Fatalf("status=%d", recorder.Code)
	}
	var payload struct {
		Code    string `json:"code"`
		Details struct {
			LatestImpact document.ArchiveImpact `json:"latest_impact"`
		} `json:"details"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != "DOCUMENT_ARCHIVE_IMPACT_CHANGED" ||
		payload.Details.LatestImpact.NodeID != 9 ||
		payload.Details.LatestImpact.DescendantCount != 4 {
		t.Fatalf("payload=%+v", payload)
	}
}

func TestPrincipalUsesAuthenticatedWorkspaceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set(pmauth.PrincipalContextKey, pmauth.Principal{UserID: 5})
	context.Set(pmauth.WorkspaceIDContextKey, uint(7))
	context.Set(pmauth.ActorFactUserIDContextKey, uint(9))
	context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	context.Request.Header.Set("X-PM-Resource-Access", "short-lived-context")
	value, ok := principal(context)
	if !ok || value.UserID != 5 || value.ParticipantUserID != 9 || value.WorkspaceID != 7 ||
		value.LinkAccessSecret != "short-lived-context" {
		t.Fatalf("principal=%+v ok=%v", value, ok)
	}
}

func TestWriteErrorMapsCommentConflictsAndAvailability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{document.ErrCommentMentionInvalid, 400, "DOCUMENT_COMMENT_MENTION_INVALID"},
		{document.ErrCommentThreadResolved, 409, "DOCUMENT_COMMENT_THREAD_RESOLVED"},
		{document.ErrCommentTransitionConflict, 409, "DOCUMENT_COMMENT_TRANSITION_CONFLICT"},
		{document.ErrCommentsUnavailable, 503, "DOCUMENT_COMMENTS_UNAVAILABLE"},
	}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		writeError(context, test.err)
		var payload struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if recorder.Code != test.status || payload.Code != test.code {
			t.Fatalf("err=%v status=%d payload=%+v", test.err, recorder.Code, payload)
		}
	}
}

func TestWriteErrorMapsContextActionFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{document.ErrContextActionsUnavailable, 503, "DOCUMENT_CONTEXT_ACTIONS_UNAVAILABLE"},
		{document.ErrOwnerUserInvalid, 400, "DOCUMENT_OWNER_USER_INVALID"},
		{document.ErrShortcutTargetInvalid, 400, "DOCUMENT_SHORTCUT_TARGET_INVALID"},
		{document.ErrSystemManagedNode, 409, "DOCUMENT_SYSTEM_NODE_LOCKED"},
	}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		writeError(context, test.err)
		var payload struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if recorder.Code != test.status || payload.Code != test.code {
			t.Fatalf("err=%v status=%d payload=%+v", test.err, recorder.Code, payload)
		}
	}
}
