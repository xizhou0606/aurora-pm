package feishu

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	lark "github.com/larksuite/oapi-sdk-go/v3"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestListFeishuDepartmentUsersCapturesJobTitle(t *testing.T) {
	base := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body string
		header := make(http.Header)
		header.Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			body = `{"code":0,"tenant_access_token":"tenant-token","expire":7200}`
		case request.URL.Path == "/open-apis/contact/v3/users/find_by_department":
			if request.URL.Query().Get("department_id") != "od-a" {
				body = `{"code":400230,"msg":"wrong department"}`
				break
			}
			// The second contact omits job_title entirely, so the reader
			// must stay nil-safe and keep the field empty.
			body = `{"code":0,"data":{"items":[
				{"open_id":"ou-1","name":"张三","job_title":"后端工程师","department_ids":["od-a"]},
				{"open_id":"ou-2","name":"李四"}
			],"has_more":false}}`
		default:
			return &http.Response{StatusCode: http.StatusNotFound,
				Header: make(http.Header), Body: io.NopCloser(strings.NewReader("not found")), Request: request}, nil
		}
		return &http.Response{StatusCode: http.StatusOK,
			Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})
	reader := &SDKReader{
		cfg: config.Feishu{},
		client: lark.NewClient("org-client", "app-secret",
			lark.WithOpenBaseUrl("https://open.test"), lark.WithOAuthBaseUrl("https://open.test"),
			lark.WithHttpClient(&http.Client{Transport: base})),
	}

	users, err := reader.ListFeishuDepartmentUsers(context.Background(), "od-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %+v", users)
	}
	if users[0].OpenID != "ou-1" || users[0].Name != "张三" || users[0].DepartmentID != "od-a" {
		t.Fatalf("first user wrong: %+v", users[0])
	}
	if users[0].JobTitle != "后端工程师" {
		t.Fatalf("job title must be captured: %+v", users[0])
	}
	if users[1].OpenID != "ou-2" || users[1].JobTitle != "" || users[1].DepartmentID != "" {
		t.Fatalf("missing job title/department must stay empty: %+v", users[1])
	}
}
