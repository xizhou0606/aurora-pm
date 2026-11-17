package workflowtestcase

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	CSVContentType = "text/csv; charset=utf-8"
	MaxCSVBytes    = 20 * 1024 * 1024
)

var categoryNames = map[string]string{
	"positive": "正向", "negative": "异常", "boundary": "边界", "permission": "权限",
	"state": "状态", "duplicate_submit": "重复提交", "concurrency": "并发", "timeout": "超时",
	"retry": "重试", "rollback": "回滚", "stale_data": "旧数据", "integration": "集成", "regression": "回归",
}

type testCaseOutput struct {
	TestCases      []testCase      `json:"testCases"`
	TestCaseDesign *testCaseDesign `json:"testCaseDesign"`
}

type testCase struct {
	CaseID          string   `json:"caseId"`
	Title           string   `json:"title"`
	Category        string   `json:"category"`
	Priority        string   `json:"priority"`
	Trace           []string `json:"trace"`
	Preconditions   []string `json:"preconditions"`
	TestData        []string `json:"testData"`
	Steps           []string `json:"steps"`
	ExpectedResult  string   `json:"expectedResult"`
	ExpectedResults []string `json:"expectedResults"`
	Notes           []string `json:"notes"`
	Evidence        []string `json:"evidence"`
}

type testCaseDesign struct {
	SmokeCaseIDs               []string `json:"smokeCaseIds"`
	ExecutionOrder             []string `json:"executionOrder"`
	AutomationCandidateCaseIDs []string `json:"automationCandidateCaseIds"`
	ReleaseBlockingCaseIDs     []string `json:"releaseBlockingCaseIds"`
}

// BuildCSV intentionally mirrors WorkflowTestCasePreview.buildTestCaseCSV so
// browser downloads and server-delivered artifacts have the same golden bytes.
func BuildCSV(raw json.RawMessage) ([]byte, error) {
	if err := ValidateProvenance("qa.test_cases", raw); err != nil {
		return nil, err
	}
	var output testCaseOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil, fmt.Errorf("%w: decode test cases", ErrOutputInvalid)
	}
	if len(output.TestCases) == 0 {
		return nil, fmt.Errorf("%w: no test cases", ErrOutputInvalid)
	}
	headers := []string{
		"用例ID", "用例名称", "优先级", "追踪引用", "类型", "前置条件", "测试数据",
		"测试步骤", "预期结果", "备注", "事实依据", "冒烟", "执行顺序", "自动化候选", "发布阻塞",
	}
	rows := make([][]string, 0, len(output.TestCases)+1)
	rows = append(rows, headers)
	for index, item := range output.TestCases {
		caseID := strings.TrimSpace(item.CaseID)
		if caseID == "" {
			caseID = fmt.Sprintf("TC-%03d", index+1)
		}
		category, valid := categoryNames[item.Category]
		if !valid {
			return nil, fmt.Errorf("%w: unsupported category %q", ErrOutputInvalid, item.Category)
		}
		expected := item.ExpectedResults
		if len(expected) == 0 && item.ExpectedResult != "" {
			expected = []string{item.ExpectedResult}
		}
		rows = append(rows, []string{
			caseID, item.Title, item.Priority, strings.Join(item.Trace, "\n"), category,
			strings.Join(item.Preconditions, "\n"), strings.Join(item.TestData, "\n"),
			numbered(item.Steps), numbered(expected), strings.Join(item.Notes, "\n"), strings.Join(item.Evidence, "\n"),
			yesNo(contains(output.TestCaseDesign, caseID, "smoke")), executionPosition(output.TestCaseDesign, caseID),
			yesNo(contains(output.TestCaseDesign, caseID, "automation")), yesNo(contains(output.TestCaseDesign, caseID, "release")),
		})
	}
	var builder strings.Builder
	builder.WriteString("\ufeff")
	for rowIndex, row := range rows {
		if rowIndex > 0 {
			builder.WriteString("\r\n")
		}
		for columnIndex, value := range row {
			if columnIndex > 0 {
				builder.WriteByte(',')
			}
			builder.WriteString(safeCSVCell(value))
		}
	}
	result := []byte(builder.String())
	if len(result) > MaxCSVBytes {
		return nil, fmt.Errorf("%w: generated CSV exceeds 20MB", ErrOutputInvalid)
	}
	return result, nil
}

func safeCSVCell(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	trimmed := strings.TrimLeft(value, "\t ")
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		value = "'" + value
	}
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func numbered(values []string) string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = fmt.Sprintf("%d. %s", index+1, value)
	}
	return strings.Join(result, "\n")
}

func yesNo(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func contains(design *testCaseDesign, caseID, field string) bool {
	if design == nil {
		return false
	}
	values := design.SmokeCaseIDs
	switch field {
	case "automation":
		values = design.AutomationCandidateCaseIDs
	case "release":
		values = design.ReleaseBlockingCaseIDs
	}
	for _, value := range values {
		if value == caseID {
			return true
		}
	}
	return false
}

func executionPosition(design *testCaseDesign, caseID string) string {
	if design == nil {
		return ""
	}
	for index, value := range design.ExecutionOrder {
		if value == caseID {
			return fmt.Sprint(index + 1)
		}
	}
	return ""
}
