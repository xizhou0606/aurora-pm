package aiworker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

// 代码质量审查技能结论必须按 sha 写回提交明细，综合评估落在 code 块；
// 无匹配的输出不得污染输入。
func TestMergeCommitQualityReview(t *testing.T) {
	input := map[string]any{
		"subject": map[string]any{
			"code": map[string]any{
				"commit_reviews": []any{
					map[string]any{"sha": "abcdef1234567890abcdef1234567890abcdef12", "subject": "重构配置"},
					map[string]any{"sha": "99999999", "subject": "无匹配提交"},
				},
			},
		},
	}
	output := json.RawMessage(`{
		"summary": "整体质量良好",
		"commitReviews": [
			{"sha": "abcdef12", "verdict": "良", "summary": "配置重构干净", "issues": ["无代码样本，按提交元信息判断"]},
			{"sha": "deadbeef", "verdict": "差", "summary": "不存在的提交", "issues": []}
		],
		"qualityAssessment": {"score": 82, "conclusion": "结论", "commonIssues": ["样本缺失"], "risks": [], "suggestions": ["补单测"]}
	}`)
	if err := mergeCommitQualityReview(input, output); err != nil {
		t.Fatalf("merge: %v", err)
	}
	code := input["subject"].(map[string]any)["code"].(map[string]any)
	entries := code["commit_reviews"].([]any)
	first := entries[0].(map[string]any)
	if first["ai_verdict"] != "良" || first["ai_summary"] != "配置重构干净" {
		t.Fatalf("matched commit not enriched: %v", first)
	}
	second := entries[1].(map[string]any)
	if _, polluted := second["ai_verdict"]; polluted {
		t.Fatalf("unmatched commit must stay untouched: %v", second)
	}
	assessment, ok := code["quality_assessment"].(map[string]any)
	if !ok || assessment["score"] != 82 || assessment["conclusion"] != "结论" {
		t.Fatalf("quality assessment missing: %v", code["quality_assessment"])
	}
	if code["quality_summary"] != "整体质量良好" {
		t.Fatalf("quality summary missing: %v", code["quality_summary"])
	}
}

func TestMergeCommitQualityReviewNoMatchFails(t *testing.T) {
	input := map[string]any{
		"subject": map[string]any{
			"code": map[string]any{"commit_reviews": []any{map[string]any{"sha": "aaaa"}}},
		},
	}
	output := json.RawMessage(`{"summary":"x","commitReviews":[{"sha":"bbbb","verdict":"中","summary":"","issues":[]}],"qualityAssessment":{"score":50}}`)
	if err := mergeCommitQualityReview(input, output); err == nil {
		t.Fatal("output matching no commits must fail instead of silently claiming success")
	}
}

func TestCommitReviewEntriesNilSafe(t *testing.T) {
	if got := commitReviewEntries(map[string]any{}); got != nil {
		t.Fatalf("empty input must yield nil, got %v", got)
	}
}

// commitQualityMember：subject.target（MemberRef）必须渲染成 {name, roles}，
// role_names 去空白、过滤空角色；target 缺失、姓名角色全空或 JSON 解析失败
// 一律返回 nil，由组装方静默跳过 member 字段。可选 job_title 非空时透传。
func TestCommitQualityMemberExtraction(t *testing.T) {
	member := commitQualityMember(`{"subject":{"target":{"name":" 李四 ","role_names":["测试","QA ",""]}}}`)
	if member == nil || member["name"] != "李四" {
		t.Fatalf("member name not extracted/trimmed: %v", member)
	}
	roles, ok := member["roles"].([]string)
	if !ok || len(roles) != 2 || roles[0] != "测试" || roles[1] != "QA" {
		t.Fatalf("roles not trimmed/filtered: %v", member["roles"])
	}
	if _, present := member["job_title"]; present {
		t.Fatalf("job_title must be omitted when target lacks it: %v", member)
	}
	if got := commitQualityMember("not-json"); got != nil {
		t.Fatalf("unparsable input must yield nil, got %v", got)
	}
	if got := commitQualityMember(`{"subject":{}}`); got != nil {
		t.Fatalf("missing target must yield nil, got %v", got)
	}
	if got := commitQualityMember(`{"subject":{"target":{"role_names":[""]}}}`); got != nil {
		t.Fatalf("target without usable name/roles must yield nil, got %v", got)
	}
	withName := commitQualityMember(`{"subject":{"target":{"name":"王五"}}}`)
	if withName == nil || withName["name"] != "王五" {
		t.Fatalf("name-only target must still produce member: %v", withName)
	}
	if roles, ok := withName["roles"].([]string); !ok || len(roles) != 0 {
		t.Fatalf("name-only member must carry empty roles: %v", withName)
	}
	if _, present := withName["job_title"]; present {
		t.Fatalf("name-only member must not invent job_title: %v", withName)
	}
}

// job_title 是可选飞书职务：非空 JSON 字符串才进 member（去空白）；缺失、
// null、空串、纯空白或非字符串类型一律省略该字段，且不得影响 name/roles
// 的正常提取。
func TestCommitQualityMemberJobTitle(t *testing.T) {
	withTitle := commitQualityMember(`{"subject":{"target":{"name":"张三","role_names":["后端"],"job_title":" 高级后端工程师 "}}}`)
	if withTitle == nil || withTitle["job_title"] != "高级后端工程师" {
		t.Fatalf("job_title not extracted/trimmed: %v", withTitle)
	}
	if withTitle["name"] != "张三" {
		t.Fatalf("job_title extraction broke name: %v", withTitle)
	}
	for name, raw := range map[string]string{
		"missing":    `{"subject":{"target":{"name":"张三"}}}`,
		"null":       `{"subject":{"target":{"name":"张三","job_title":null}}}`,
		"empty":      `{"subject":{"target":{"name":"张三","job_title":""}}}`,
		"whitespace": `{"subject":{"target":{"name":"张三","job_title":"   "}}}`,
		"nonString":  `{"subject":{"target":{"name":"张三","job_title":42}}}`,
		"array":      `{"subject":{"target":{"name":"张三","job_title":["工程师"]}}}`,
	} {
		member := commitQualityMember(raw)
		if member == nil || member["name"] != "张三" {
			t.Fatalf("%s: member must survive job_title anomaly: %v", name, member)
		}
		if _, present := member["job_title"]; present {
			t.Fatalf("%s: unusable job_title must be omitted, got %v", name, member["job_title"])
		}
	}
}

// 请求体组装：run 身份、capability、commits 照常透传；InputJSON 可解析时
// member（含可选 job_title）与 commits 并列进 input.subject，解析失败时
// 静默跳过 member。
func TestCommitQualityRequestPayloadShape(t *testing.T) {
	run := &aigorm.AIRun{ID: 9, WorkspaceID: 2, InputJSON: `{"subject":{"target":{"name":"张三","role_names":["前端"],"job_title":"高级前端工程师"}}}`}
	payload := commitQualityRequestPayload(run, []any{map[string]any{"sha": "abc"}})
	if payload["runId"] != uint(9) || payload["workspaceId"] != uint(2) {
		t.Fatalf("run identity missing: %v", payload)
	}
	if payload["capability"] != commitQualityReviewCapability {
		t.Fatalf("capability mismatch: %v", payload["capability"])
	}
	subject := payload["input"].(map[string]any)["subject"].(map[string]any)
	member, hasMember := subject["member"].(map[string]any)
	if !hasMember || member["name"] != "张三" {
		t.Fatalf("member missing from subject: %v", subject)
	}
	if member["job_title"] != "高级前端工程师" {
		t.Fatalf("job_title not passed through: %v", member["job_title"])
	}
	if commits, ok := subject["commits"].([]any); !ok || len(commits) != 1 {
		t.Fatalf("commits must stay untouched: %v", subject["commits"])
	}
	broken := commitQualityRequestPayload(&aigorm.AIRun{ID: 9, WorkspaceID: 2, InputJSON: "{broken"}, nil)
	subject = broken["input"].(map[string]any)["subject"].(map[string]any)
	if _, hasMember := subject["member"]; hasMember {
		t.Fatal("unparsable InputJSON must skip member silently")
	}
}

// 全链路：技能请求体的 input.subject 必须携带 member（name 来自原始
// InputJSON 的 subject.target，role_names 去空过滤后透传为 roles，可选
// job_title 非空时一并透传），与 commits 并列；member 注入不得影响逐提交
// 结论按 sha 回写。
func TestEnrichReportWithCommitQualityInjectsMember(t *testing.T) {
	var captured []byte
	cfg := config.Config{AI: config.AI{RuntimeBaseURL: "http://ai-runtime.test", InternalSecret: "worker-test-secret-at-least-32-characters", RequestTimeout: 2 * time.Second}}
	worker := New(nil, cfg, nil)
	worker.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		captured = body
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":{"output":{"summary":"整体质量良好","commitReviews":[{"sha":"abcdef12","verdict":"良","summary":"配置重构干净","issues":[]}],"qualityAssessment":{"score":82,"conclusion":"结论","commonIssues":[],"risks":[],"suggestions":[]}}}}`)),
			Request:    request,
		}, nil
	})
	run := &aigorm.AIRun{
		ID:          7,
		WorkspaceID: 3,
		Capability:  "report.weekly",
		InputJSON:   `{"subject":{"target":{"name":"张三","role_names":["前端","","后端"],"job_title":"资深前端工程师"},"code":{"commit_reviews":[{"sha":"abcdef1234567890","subject":"重构配置"}]}}}`,
	}
	input := map[string]any{
		"subject": map[string]any{
			"code": map[string]any{
				"commit_reviews": []any{map[string]any{"sha": "abcdef1234567890", "subject": "重构配置"}},
			},
		},
	}
	worker.enrichReportWithCommitQuality(context.Background(), run, input)
	if len(captured) == 0 {
		t.Fatal("skill request was not sent")
	}
	var payload struct {
		Input struct {
			Subject struct {
				Member map[string]any `json:"member"`
			} `json:"subject"`
		} `json:"input"`
	}
	if err := json.Unmarshal(captured, &payload); err != nil {
		t.Fatalf("decode captured request: %v", err)
	}
	member := payload.Input.Subject.Member
	if member == nil {
		t.Fatal("member missing from skill request subject")
	}
	if member["name"] != "张三" {
		t.Fatalf("member name mismatch: %v", member["name"])
	}
	roles, ok := member["roles"].([]any)
	if !ok || len(roles) != 2 || roles[0] != "前端" || roles[1] != "后端" {
		t.Fatalf("role_names not passed through as roles: %v", member["roles"])
	}
	if member["job_title"] != "资深前端工程师" {
		t.Fatalf("job_title not passed through: %v", member["job_title"])
	}
	first := input["subject"].(map[string]any)["code"].(map[string]any)["commit_reviews"].([]any)[0].(map[string]any)
	if first["ai_verdict"] != "良" || first["ai_summary"] != "配置重构干净" {
		t.Fatalf("merge broken by member injection: %v", first)
	}
}
