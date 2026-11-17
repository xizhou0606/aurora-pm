package workflowtestcase

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParseTargetStrictContract(t *testing.T) {
	target, found, err := ParseTarget(`{"subject":{},"test_case_delivery":{"instance_id":12,"node_id":" cases "}}`)
	if err != nil || !found || target.InstanceID != 12 || target.NodeID != "cases" {
		t.Fatalf("target=%+v found=%v err=%v", target, found, err)
	}
	if _, found, err := ParseTarget(`{"subject":{}}`); err != nil || found {
		t.Fatalf("missing target found=%v err=%v", found, err)
	}
	for _, raw := range []string{
		`{"test_case_delivery":{"instance_id":0,"node_id":"cases"}}`,
		`{"test_case_delivery":{"instance_id":12,"node_id":""}}`,
		`{"test_case_delivery":{"instance_id":12,"node_id":"cases","workspace_id":1}}`,
	} {
		if _, found, err := ParseTarget(raw); !found || !errors.Is(err, ErrTargetInvalid) {
			t.Fatalf("raw=%s found=%v err=%v", raw, found, err)
		}
	}
}

func TestBuildCSVMatchesFrontendGolden(t *testing.T) {
	raw := json.RawMessage(validOutputJSON(`{
		"caseId":"","title":"=SUM(A1)","category":"concurrency","priority":"P1",
		"trace":["REQ-1","RISK-1"],"preconditions":["登录"],"testData":["+danger"],
		"steps":["点击\"提交\"","等待"],"expectedResults":["请求已受理","页面显示成功"],
		"notes":["@derived"],"evidence":["subject.title"]
	}`, `{
		"smokeCaseIds":["TC-001"],"executionOrder":["TC-001"],
		"automationCandidateCaseIds":["TC-001"],"releaseBlockingCaseIds":[]
	}`))
	csv, err := BuildCSV(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := "\ufeff\"用例ID\",\"用例名称\",\"优先级\",\"追踪引用\",\"类型\",\"前置条件\",\"测试数据\",\"测试步骤\",\"预期结果\",\"备注\",\"事实依据\",\"冒烟\",\"执行顺序\",\"自动化候选\",\"发布阻塞\"\r\n" +
		"\"TC-001\",\"'=SUM(A1)\",\"P1\",\"REQ-1\nRISK-1\",\"并发\",\"登录\",\"'+danger\",\"1. 点击\"\"提交\"\"\n2. 等待\",\"1. 请求已受理\n2. 页面显示成功\",\"'@derived\",\"subject.title\",\"是\",\"1\",\"是\",\"否\""
	if string(csv) != want {
		t.Fatalf("CSV golden mismatch\ngot:  %q\nwant: %q", string(csv), want)
	}
	if len(csv) < 3 || string(csv[:3]) != "\xef\xbb\xbf" {
		t.Fatalf("CSV BOM=%x", csv[:min(3, len(csv))])
	}
}

func TestBuildCSVRejectsInvalidSkillProvenance(t *testing.T) {
	raw := validOutputJSON(`{"title":"A","category":"positive","priority":"P1","preconditions":[],"steps":["打开"],"expectedResults":["可见"],"evidence":["subject.title"]}`, `null`)
	raw = strings.Replace(raw, SampleSkillCommit, "not-a-valid-git-commit-sha", 1)
	if _, err := BuildCSV(json.RawMessage(raw)); !errors.Is(err, ErrOutputInvalid) {
		t.Fatalf("error=%v", err)
	}
}

func validOutputJSON(testCaseJSON, designJSON string) string {
	return `{"summary":"已生成","testCases":[` + testCaseJSON + `],"testCaseDesign":` + designJSON +
		`,"skillProvenance":{"name":"` + SkillName + `","repository":"` + SkillRepository +
		`","path":"` + SkillPath + `","commit":"` + SampleSkillCommit + `","packageSha256":"` + SampleSkillPackageSHA256 +
		`","mode":"advanced","resources":["` + StructuredResource + `","` + AdvancedResource + `","` + WritingResource + `"]}}`
}
