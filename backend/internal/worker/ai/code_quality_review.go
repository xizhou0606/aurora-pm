package aiworker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
)

// 代码质量审查技能编排：report.* 运行携带逐提交证据时，先调用专用技能
// code.commit_quality_review（ai-runtime 内置技能，输出逐提交结论与综合
// 评估），把结论写回报告输入供叙述与导出引用。技能调用失败只降级——
// 叙述仍可依据提交主题与行数点评——绝不阻断报告生成。
const commitQualityReviewCapability = "code.commit_quality_review"

func (w *Worker) enrichReportWithCommitQuality(ctx context.Context, run *aigorm.AIRun, input map[string]any) {
	if !strings.HasPrefix(strings.TrimSpace(run.Capability), "report.") {
		return
	}
	reviews := commitReviewEntries(input)
	if len(reviews) == 0 {
		return
	}
	raw, err := json.Marshal(commitQualityRequestPayload(run, reviews))
	if err != nil {
		log.Printf("code quality review: marshal request: %v", err)
		return
	}
	if err := validateRuntimeRequestSize(commitQualityReviewCapability, len(raw)); err != nil {
		log.Printf("code quality review: %v", err)
		return
	}
	status, body, err := w.callRuntime(ctx, raw)
	if err != nil {
		log.Printf("code quality review: runtime call failed: %v", err)
		return
	}
	if status != http.StatusOK {
		log.Printf("code quality review: runtime returned status %d", status)
		return
	}
	var envelope struct {
		Data struct {
			Output json.RawMessage `json:"output"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &envelope) != nil || len(envelope.Data.Output) == 0 {
		log.Printf("code quality review: invalid runtime response payload")
		return
	}
	if err := mergeCommitQualityReview(input, envelope.Data.Output); err != nil {
		log.Printf("code quality review: merge failed: %v", err)
	}
}

// commitQualityRequestPayload 组装技能请求体。成员姓名、岗位角色与可选飞书
// 职务（job_title，缺失即省略）来自原始 run.InputJSON 里的 subject.target
// （MemberRef），这里独立解析，不依赖解码后 input 的生命周期；解析失败静默
// 跳过 member，技能调用退化为仅提交证据。
func commitQualityRequestPayload(run *aigorm.AIRun, reviews []any) map[string]any {
	subject := map[string]any{"commits": reviews}
	if member := commitQualityMember(run.InputJSON); member != nil {
		subject["member"] = member
	}
	return map[string]any{
		"runId":       run.ID,
		"workspaceId": run.WorkspaceID,
		"capability":  commitQualityReviewCapability,
		"input":       map[string]any{"subject": subject},
	}
}

// commitQualityMember 从原始 run input JSON 提取 subject.target（MemberRef，
// 结构 {name, role_names[], job_title?, ...}），渲染为 {name, roles, job_title?}；
// 姓名与角色全空、target 缺失或 JSON 解析失败都返回 nil，由调用方静默跳过
// member 字段。job_title 是可选的飞书原始职务（由报告主体并行链路写入，可能
// 整体缺失），缺失、null、非字符串或空白时直接省略，不影响 name/roles。
func commitQualityMember(inputJSON string) map[string]any {
	var payload struct {
		Subject struct {
			Target struct {
				Name      string          `json:"name"`
				RoleNames []string        `json:"role_names"`
				JobTitle  json.RawMessage `json:"job_title"`
			} `json:"target"`
		} `json:"subject"`
	}
	if json.Unmarshal([]byte(inputJSON), &payload) != nil {
		return nil
	}
	name := strings.TrimSpace(payload.Subject.Target.Name)
	roles := make([]string, 0, len(payload.Subject.Target.RoleNames))
	for _, role := range payload.Subject.Target.RoleNames {
		if role = strings.TrimSpace(role); role != "" {
			roles = append(roles, role)
		}
	}
	if name == "" && len(roles) == 0 {
		return nil
	}
	member := map[string]any{"name": name, "roles": roles}
	if jobTitle := commitQualityJobTitle(payload.Subject.Target.JobTitle); jobTitle != "" {
		member["job_title"] = jobTitle
	}
	return member
}

// commitQualityJobTitle 解码可选的 job_title 原始 JSON：只有 JSON 字符串才
// 接受（去空白后非空），缺失、null、非字符串类型都返回空串。用 RawMessage
// 解码，异常类型只丢弃 job_title 自身，不会让整个 member 解析失败。
func commitQualityJobTitle(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var title string
	if json.Unmarshal(raw, &title) != nil {
		return ""
	}
	return strings.TrimSpace(title)
}

// commitReviewEntries extracts subject.code.commit_reviews from the decoded
// run input; missing pieces simply yield nil.
func commitReviewEntries(input map[string]any) []any {
	subject, _ := input["subject"].(map[string]any)
	if subject == nil {
		return nil
	}
	code, _ := subject["code"].(map[string]any)
	if code == nil {
		return nil
	}
	entries, _ := code["commit_reviews"].([]any)
	return entries
}

type commitQualityReviewOutput struct {
	Summary           string                      `json:"summary"`
	CommitReviews     []commitQualityReviewCommit `json:"commitReviews"`
	QualityAssessment commitQualityAssessment     `json:"qualityAssessment"`
}

type commitQualityReviewCommit struct {
	SHA     string   `json:"sha"`
	Verdict string   `json:"verdict"`
	Summary string   `json:"summary"`
	Issues  []string `json:"issues"`
}

type commitQualityAssessment struct {
	Score        int      `json:"score"`
	Conclusion   string   `json:"conclusion"`
	CommonIssues []string `json:"commonIssues"`
	Risks        []string `json:"risks"`
	Suggestions  []string `json:"suggestions"`
}

// mergeCommitQualityReview writes the skill's verdicts back into the run
// input: per-commit ai_verdict/ai_summary/ai_issues matched by sha, plus the
// code-level quality_assessment block for the narrative and export.
func mergeCommitQualityReview(input map[string]any, output json.RawMessage) error {
	var review commitQualityReviewOutput
	if err := json.Unmarshal(output, &review); err != nil {
		return fmt.Errorf("decode skill output: %w", err)
	}
	reviews := commitReviewEntries(input)
	used := make(map[int]bool, len(review.CommitReviews))
	code := commitCodeBlock(input)
	merged := 0
	for _, entry := range reviews {
		item, _ := entry.(map[string]any)
		if item == nil {
			continue
		}
		sha, _ := item["sha"].(string)
		match, index, ok := matchCommitReview(review.CommitReviews, sha, used)
		if !ok {
			continue
		}
		used[index] = true
		item["ai_verdict"] = match.Verdict
		item["ai_summary"] = match.Summary
		if match.Issues != nil {
			item["ai_issues"] = match.Issues
		}
		merged++
	}
	if merged == 0 {
		return fmt.Errorf("skill output matched none of %d commits", len(reviews))
	}
	code["quality_summary"] = review.Summary
	code["quality_assessment"] = map[string]any{
		"score":         review.QualityAssessment.Score,
		"conclusion":    review.QualityAssessment.Conclusion,
		"common_issues": review.QualityAssessment.CommonIssues,
		"risks":         review.QualityAssessment.Risks,
		"suggestions":   review.QualityAssessment.Suggestions,
	}
	return nil
}

// matchCommitReview 按.sha 前缀匹配技能结论与输入提交：模型经常回填短
// SHA（7-8 位），全量等值比较会把全部结论判成无主而丢弃。同一结论只消费
// 一次（used），避免多条短 SHA 重复命中同一条输入。
func matchCommitReview(
	reviews []commitQualityReviewCommit, sha string, used map[int]bool,
) (commitQualityReviewCommit, int, bool) {
	sha = strings.ToLower(strings.TrimSpace(sha))
	if sha == "" {
		return commitQualityReviewCommit{}, 0, false
	}
	for index, review := range reviews {
		if used[index] {
			continue
		}
		candidate := strings.ToLower(strings.TrimSpace(review.SHA))
		if candidate == "" {
			continue
		}
		if strings.HasPrefix(sha, candidate) || strings.HasPrefix(candidate, sha) {
			return review, index, true
		}
	}
	return commitQualityReviewCommit{}, 0, false
}

func commitCodeBlock(input map[string]any) map[string]any {
	subject, _ := input["subject"].(map[string]any)
	if subject == nil {
		subject = map[string]any{}
		input["subject"] = subject
	}
	code, _ := subject["code"].(map[string]any)
	if code == nil {
		code = map[string]any{}
		subject["code"] = code
	}
	return code
}

// injectCommitQualityIntoOutput embeds the skill assessment into the run's
// stored output envelope so the poll endpoint can surface it to the page.
// input 是执行期已富化的输入(技能结论在内存 map 里),不是原始 InputJSON。
func injectCommitQualityIntoOutput(input map[string]any, output json.RawMessage) json.RawMessage {
	if input == nil {
		return output
	}
	code := commitCodeBlock(input)
	assessment, ok := code["quality_assessment"].(map[string]any)
	if !ok {
		return output
	}
	quality := map[string]any{
		"score":         assessment["score"],
		"conclusion":    assessment["conclusion"],
		"common_issues": assessment["common_issues"],
		"risks":         assessment["risks"],
		"suggestions":   assessment["suggestions"],
	}
	verdicts := []any{}
	for _, entry := range commitReviewEntries(input) {
		item, _ := entry.(map[string]any)
		if item == nil {
			continue
		}
		verdict, _ := item["ai_verdict"].(string)
		if verdict == "" {
			continue
		}
		summary, _ := item["ai_summary"].(string)
		issues, _ := item["ai_issues"].([]any)
		verdicts = append(verdicts, map[string]any{
			"sha": item["sha"], "verdict": verdict, "summary": summary, "issues": issues,
		})
	}
	quality["commits"] = verdicts
	var envelope map[string]any
	if json.Unmarshal(output, &envelope) != nil || envelope == nil {
		return output
	}
	envelope["commit_quality"] = quality
	merged, err := json.Marshal(envelope)
	if err != nil {
		return output
	}
	return merged
}
