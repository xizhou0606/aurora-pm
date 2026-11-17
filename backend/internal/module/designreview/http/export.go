package designreviewhttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (handler *Handler) exportReview(c *gin.Context) {
	workspaceID, userID, _, ok := requestIdentity(c)
	if !ok {
		return
	}
	detail, err := handler.service.GetReview(c.Request.Context(), workspaceID, userID, c.Param("id"))
	if err != nil {
		failDomain(c, err)
		return
	}
	if c.DefaultQuery("format", "markdown") == "json" {
		raw, marshalErr := json.MarshalIndent(detail, "", "  ")
		if marshalErr != nil {
			failDomain(c, marshalErr)
			return
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="design-review-%s.json"`, detail.ID))
		c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
		return
	}
	var report strings.Builder
	fmt.Fprintf(&report, "# %s\n\n", detail.RequirementTitle)
	fmt.Fprintf(&report, "- 走查轮次：%d\n- 状态：%s\n- 结论：%s\n- 项目集：%s\n- 项目/版本：%s\n- Figma：%s\n\n", detail.RoundNumber, detail.Status, stringValue(detail.Conclusion), detail.ProjectSetName, detail.PlanUnitTitle, detail.FigmaURL)
	if detail.Model != nil {
		fmt.Fprintf(&report, "- 分析模型：%s\n- 推理强度：%s\n- 提示版本：%s\n- 响应 ID：%s\n- 分析耗时：%s\n- 分析尝试：%d\n\n", stringValue(detail.Model), stringValue(detail.ReasoningEffort), stringValue(detail.PromptVersion), stringValue(detail.ResponseID), durationValue(detail.AnalysisDurationMS), detail.AnalysisAttempts)
	}
	fmt.Fprintf(&report, "## 走查摘要\n\n%s\n\n## 差异（%d）\n\n", stringValue(detail.AnalysisSummary), len(detail.Differences))
	for _, difference := range detail.Differences {
		fmt.Fprintf(&report, "### %d. %s\n\n- 分类：%s\n- 严重程度：%s\n- 核验：%s\n- 解决状态：%s\n\n%s\n\n建议：%s\n\n", difference.Ordinal, difference.Title, difference.Category, difference.Severity, difference.VerificationStatus, difference.ResolutionStatus, difference.Description, difference.Recommendation)
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="design-review-%s.md"`, detail.ID))
	c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(report.String()))
}

func durationValue(value *int) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%d ms", *value)
}

func stringValue(value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "-"
	}
	return *value
}
