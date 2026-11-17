package report

import "strings"

// 岗位与团队的关键词判定口径。系统没有结构化岗位字段，个人依据生效角色名
// （role_names）与飞书职务（job_title）文本、团队依据用户组/部门名称文本
// 匹配。优先级固定为 测试 > 产品 > 设计：「产品设计」按产品岗算，而
// 「UI 设计师」「视觉设计」只命中设计关键词。判定结果决定汇报的事实面板
// 与 AI 评语侧重。
var (
	roleKeywordsTesting = []string{"测试", "qa", "质量"}
	roleKeywordsProduct = []string{"产品"}
	roleKeywordsDesign  = []string{"设计", "ui", "ux", "视觉", "交互"}
)

// roleFocus selects the fact panel and narrative emphasis of one report:
// testing members report raised bugs, product members report owned
// requirements, design members report UI-defect handling, and everything
// else keeps the task/bug/code projection.
type roleFocus string

const (
	focusNone    roleFocus = ""
	focusTesting roleFocus = "testing"
	focusProduct roleFocus = "product"
	focusDesign  roleFocus = "design"
)

// memberRoleFocus classifies one member target; testing wins over product,
// product over design.
func memberRoleFocus(target ReportTarget) roleFocus {
	names := append(append([]string{}, target.RoleNames...), target.JobTitle)
	if matchesRoleKeyword(names, roleKeywordsTesting) {
		return focusTesting
	}
	if matchesRoleKeyword(names, roleKeywordsProduct) {
		return focusProduct
	}
	if matchesRoleKeyword(names, roleKeywordsDesign) {
		return focusDesign
	}
	return focusNone
}

// teamRoleFocus classifies one group/department target by its display name,
// with the same precedence as member classification.
func teamRoleFocus(target ReportTarget) roleFocus {
	names := []string{target.Name}
	if matchesRoleKeyword(names, roleKeywordsTesting) {
		return focusTesting
	}
	if matchesRoleKeyword(names, roleKeywordsProduct) {
		return focusProduct
	}
	if matchesRoleKeyword(names, roleKeywordsDesign) {
		return focusDesign
	}
	return focusNone
}

// matchesRoleKeyword reports whether any haystack contains any keyword,
// case-insensitively so the latin keywords (QA/UI/UX) match lowercased sources.
func matchesRoleKeyword(haystacks []string, keywords []string) bool {
	for _, haystack := range haystacks {
		normalized := strings.ToLower(strings.TrimSpace(haystack))
		if normalized == "" {
			continue
		}
		for _, keyword := range keywords {
			if strings.Contains(normalized, keyword) {
				return true
			}
		}
	}
	return false
}
