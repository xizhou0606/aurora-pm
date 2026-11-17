package reporthttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	ai "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	report "github.com/xizhou0606/aurora-pm/backend/internal/module/report"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type stubDirectory struct{}

func (stubDirectory) ListWorkspaceMembers(context.Context, uint) ([]identity.DirectoryMember, error) {
	return []identity.DirectoryMember{
		{ID: 11, FactID: 101, Username: "alpha", DisplayName: "Alpha", GroupID: 21, GroupName: "研发组", AvatarURL: "https://example.test/a.png"},
	}, nil
}

func (stubDirectory) FindWorkspaceMember(_ context.Context, _ uint, factID uint) (identity.DirectoryMember, error) {
	if factID == 101 {
		return identity.DirectoryMember{ID: 11, FactID: 101, Username: "alpha", DisplayName: "Alpha", GroupName: "研发组"}, nil
	}
	return identity.DirectoryMember{}, identity.ErrReportingDirectoryMemberNotFound
}

func (stubDirectory) ListEnabledGroups(context.Context) ([]identity.UserGroup, error) {
	return []identity.UserGroup{
		{ID: 21, Code: "dev", Name: "研发组", Enabled: true},
	}, nil
}

func (stubDirectory) ListFeishuDepartments(context.Context) ([]identity.DirectoryDepartment, error) {
	return []identity.DirectoryDepartment{
		{OpenID: "od-dev", ParentID: "0", Name: "研发组", MemberCount: 1},
	}, nil
}

func (stubDirectory) FindWorkspaceDepartment(
	context.Context, uint, uint,
) (identity.WorkspaceDepartment, error) {
	return identity.WorkspaceDepartment{}, identity.ErrReportingDirectoryMemberNotFound
}

func (stubDirectory) FindWorkspaceGroup(_ context.Context, _ uint, groupID uint) (identity.WorkspaceGroup, error) {
	if groupID == 21 {
		return identity.WorkspaceGroup{ID: 21, Code: "dev", Name: "研发组", Members: []identity.DirectoryMember{
			{ID: 11, FactID: 101, Username: "alpha", DisplayName: "Alpha", GroupID: 21, GroupName: "研发组"},
		}}, nil
	}
	return identity.WorkspaceGroup{}, identity.ErrReportingDirectoryMemberNotFound
}

type stubTaskFacts struct{}

func (stubTaskFacts) ListBIFacts(context.Context, uint) ([]task.Snapshot, []task.ActivitySnapshot, error) {
	return []task.Snapshot{}, nil, nil
}

type stubBugFacts struct{}

func (stubBugFacts) ListBIFacts(context.Context, uint) ([]quality.BugSnapshot, []quality.BugActivitySnapshot, error) {
	return []quality.BugSnapshot{}, nil, nil
}

type stubRuns struct{ err error }

func (stub *stubRuns) StartReportRun(context.Context, ai.ReportRunStartCommand) (ai.ReportRunStartResult, error) {
	if stub.err != nil {
		return ai.ReportRunStartResult{}, stub.err
	}
	return ai.ReportRunStartResult{RunID: 77, Status: "queued", Enabled: true}, nil
}

type stubRunReader struct {
	result   ai.ReportRunResult
	notFound bool
}

func (stub stubRunReader) FindReportRunWithInput(context.Context, uint, uint) (ai.ReportRunResult, error) {
	if stub.notFound {
		return ai.ReportRunResult{}, ai.ErrReportRunNotFound
	}
	return stub.result, nil
}

func (stub stubRunReader) FindReportRun(context.Context, uint, uint) (ai.ReportRunResult, error) {
	if stub.notFound {
		return ai.ReportRunResult{}, ai.ErrReportRunNotFound
	}
	return stub.result, nil
}

func (stub stubRunReader) FindLatestReportRun(context.Context, ai.ReportRunLatestQuery) (ai.ReportRunResult, error) {
	if stub.notFound {
		return ai.ReportRunResult{}, ai.ErrReportRunNotFound
	}
	return stub.result, nil
}

type stubExporter struct{ err error }

func (stub *stubExporter) CreateReportDocument(context.Context, string, string) (report.CreatedDocument, error) {
	if stub.err != nil {
		return report.CreatedDocument{}, stub.err
	}
	return report.CreatedDocument{DocumentID: "doc-1", URL: "https://example.test/docx/doc-1"}, nil
}

func newTestHandler(t *testing.T, runs *stubRuns, reader stubRunReader, exporter report.DocumentExporter) *Handler {
	t.Helper()
	service, err := report.NewService(report.Dependencies{
		Directory:  stubDirectory{},
		Tasks:      stubTaskFacts{},
		Bugs:       stubBugFacts{},
		Runs:       runs,
		RunResults: reader,
		Exporter:   exporter,
	})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(service)
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func perform(handler *Handler, method, path, body string, withContext bool) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	if withContext {
		// The real middleware chain sets both keys; here they are injected
		// directly to exercise the handler contract.
		engine.Use(func(c *gin.Context) {
			c.Set(pmauth.PrincipalContextKey, pmauth.Principal{UserID: 9, Username: "alpha"})
			c.Set(pmauth.WorkspaceIDContextKey, uint(7))
			c.Next()
		})
	}
	handler.RegisterRoutes(engine.Group("/api/v1/platform"))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestMemberDirectoryRequiresWorkspaceContext(t *testing.T) {
	handler := newTestHandler(t, &stubRuns{}, stubRunReader{}, nil)
	recorder := perform(handler, http.MethodGet, "/api/v1/platform/reports/members", "", false)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMemberDirectoryReturnsGroupedMembersWithAvatars(t *testing.T) {
	handler := newTestHandler(t, &stubRuns{}, stubRunReader{}, nil)
	recorder := perform(handler, http.MethodGet, "/api/v1/platform/reports/members", "", true)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, fragment := range []string{`"group_name":"研发组"`, `"id":101`, `"avatar_url":"https://example.test/a.png"`} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("body=%s", body)
		}
	}
}

// 预览随行该视图最近一次 run：定时生成的汇报没有前端回执，页面靠它把
// 评语与正文自动带出来；无 run 时字段省略而不是报错。
func TestMemberReportPreviewCarriesLatestRun(t *testing.T) {
	completed := ai.ReportRunResult{
		ID: 77, Status: "completed", Summary: "本周完成 5 项任务",
		OverallComment: "整体推进稳健。", SubjectID: 101,
	}
	handler := newTestHandler(t, &stubRuns{}, stubRunReader{result: completed}, nil)
	recorder := perform(handler, http.MethodGet, "/api/v1/platform/reports/members/101/preview?type=daily", "", true)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, fragment := range []string{`"report":{`, `"latest_run":{`, `"overall_comment":"整体推进稳健。"`, `"summary":"本周完成 5 项任务"`} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("body=%s", body)
		}
	}

	// 该视图尚无 run：字段省略，预览本身照常返回。
	missing := newTestHandler(t, &stubRuns{}, stubRunReader{notFound: true}, nil)
	recorder = perform(missing, http.MethodGet, "/api/v1/platform/reports/members/101/preview?type=daily", "", true)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "latest_run") {
		t.Fatalf("latest_run must be omitted when absent: %s", recorder.Body.String())
	}
}

func TestMemberAndGroupReportPreviewsValidateInput(t *testing.T) {
	handler := newTestHandler(t, &stubRuns{}, stubRunReader{}, nil)
	cases := []struct {
		path   string
		status int
	}{
		{"/api/v1/platform/reports/members/101/preview?type=daily", http.StatusOK},
		{"/api/v1/platform/reports/groups/21/preview?type=daily", http.StatusOK},
		{"/api/v1/platform/reports/members/101/preview", http.StatusBadRequest},
		{"/api/v1/platform/reports/members/101/preview?type=yearly", http.StatusBadRequest},
		{"/api/v1/platform/reports/members/not-a-number/preview?type=daily", http.StatusBadRequest},
		{"/api/v1/platform/reports/members/999/preview?type=daily", http.StatusNotFound},
		{"/api/v1/platform/reports/groups/999/preview?type=daily", http.StatusNotFound},
	}
	for _, testCase := range cases {
		recorder := perform(handler, http.MethodGet, testCase.path, "", true)
		if recorder.Code != testCase.status {
			t.Fatalf("%s status=%d body=%s", testCase.path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestMemberReportGenerateQueuesRun(t *testing.T) {
	runs := &stubRuns{}
	handler := newTestHandler(t, runs, stubRunReader{}, nil)
	recorder := perform(handler, http.MethodPost,
		"/api/v1/platform/reports/members/101/generate", `{"type":"weekly","date":"2026-09-02"}`, true)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data struct {
			RunID  uint          `json:"run_id"`
			Status string        `json:"status"`
			Report report.Report `json:"report"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.RunID != 77 || envelope.Data.Status != "queued" || envelope.Data.Report.Target.ID != 101 {
		t.Fatalf("envelope=%+v", envelope.Data)
	}

	group := perform(handler, http.MethodPost,
		"/api/v1/platform/reports/groups/21/generate", `{"type":"daily"}`, true)
	if group.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", group.Code, group.Body.String())
	}
}

func TestMemberReportGenerateFailsClosedWithoutAI(t *testing.T) {
	handler := newTestHandler(t, &stubRuns{err: ai.ErrReportRunUnavailable}, stubRunReader{}, nil)
	recorder := perform(handler, http.MethodPost,
		"/api/v1/platform/reports/members/101/generate", `{"type":"daily"}`, true)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "REPORT_EXPORT_UNAVAILABLE") &&
		!strings.Contains(recorder.Body.String(), "AI report run is unavailable") {
		t.Fatalf("body=%s", recorder.Body.String())
	}
}

func TestMemberReportRunStatusScopesAndMapsNotFound(t *testing.T) {
	handler := newTestHandler(t, &stubRuns{}, stubRunReader{
		result: ai.ReportRunResult{ID: 77, Status: "completed", Summary: "本周完成 5 项任务", SubjectID: 101},
	}, nil)
	recorder := perform(handler, http.MethodGet, "/api/v1/platform/reports/runs/77", "", true)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "本周完成 5 项任务") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	missing := newTestHandler(t, &stubRuns{}, stubRunReader{notFound: true}, nil)
	recorder = perform(missing, http.MethodGet, "/api/v1/platform/reports/runs/404", "", true)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestExportReportRunFeishuDoc(t *testing.T) {
	completed := ai.ReportRunResult{
		ID: 77, Status: "completed", Capability: ai.CapabilityReportDaily,
		SubjectType: ai.ReportSubjectMember, SubjectID: 101, Summary: "今日完成 3 项任务",
		InputJSON: `{"subject":{"target":{"name":"Alpha"},"period":{"type":"daily","anchor_date":"2026-09-02"}}}`,
	}
	handler := newTestHandler(t, &stubRuns{}, stubRunReader{result: completed}, &stubExporter{})
	recorder := perform(handler, http.MethodPost, "/api/v1/platform/reports/runs/77/feishu-doc", "", true)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "doc-1") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	pending := completed
	pending.Status = "running"
	notReady := newTestHandler(t, &stubRuns{}, stubRunReader{result: pending}, &stubExporter{})
	recorder = perform(notReady, http.MethodPost, "/api/v1/platform/reports/runs/77/feishu-doc", "", true)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	withoutExporter := newTestHandler(t, &stubRuns{}, stubRunReader{result: completed}, nil)
	recorder = perform(withoutExporter, http.MethodPost, "/api/v1/platform/reports/runs/77/feishu-doc", "", true)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestExportReportFactsFeishuDoc(t *testing.T) {
	handler := newTestHandler(t, &stubRuns{}, stubRunReader{}, &stubExporter{})
	body := `{"target_kind":"member","target_id":101,"type":"daily","date":"2026-09-02"}`
	recorder := perform(handler, http.MethodPost, "/api/v1/platform/reports/runs/0/feishu-doc", body, true)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "doc-1") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	// The facts document title comes from the aggregation target.
	if !strings.Contains(recorder.Body.String(), "Alpha") {
		t.Fatalf("facts export must carry the target title, body=%s", recorder.Body.String())
	}

	// Unknown members fail closed through the directory lookup.
	unknown := `{"target_kind":"member","target_id":999,"type":"daily"}`
	recorder = perform(handler, http.MethodPost, "/api/v1/platform/reports/runs/0/feishu-doc", unknown, true)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown member status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	// Invalid period type is a bad request.
	recorder = perform(handler, http.MethodPost, "/api/v1/platform/reports/runs/0/feishu-doc",
		`{"target_kind":"member","target_id":101,"type":"quarterly"}`, true)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("bad period status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	// Without the Feishu exporter the endpoint fails closed.
	withoutExporter := newTestHandler(t, &stubRuns{}, stubRunReader{}, nil)
	recorder = perform(withoutExporter, http.MethodPost, "/api/v1/platform/reports/runs/0/feishu-doc", body, true)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("no exporter status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
