package notification

import (
	"strings"
	"testing"
)

func TestPublicURLsJoinStandaloneOrigin(t *testing.T) {
	t.Parallel()
	base := "https://pm-main.example.invalid/"
	cases := []struct {
		got  string
		want string
	}{
		{got: WorkItemURL(base, 12), want: "https://pm-main.example.invalid/workspace/work-items/12"},
		{got: QualityBugURL(base, 7), want: "https://pm-main.example.invalid/workspace/quality-management/bugs/7"},
		{got: RequirementURL(base, 9), want: "https://pm-main.example.invalid/workspace/requirements/9"},
		{got: WorkflowListURL(base), want: "https://pm-main.example.invalid/workspace/pm/workflows"},
		{got: PublicURL("  ", "workspace", "work-items", "1"), want: ""},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Fatalf("got=%q want=%q", tc.got, tc.want)
		}
	}
}

func TestInteractiveCardUsesPMBotChrome(t *testing.T) {
	t.Parallel()
	body := "缺陷已提交：123\n编号：BUG-4745\n链接：https://pm-main.example.invalid/workspace/quality-management/bugs/4745"
	raw, err := InteractiveCardJSON(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, CardTitle) || !strings.Contains(raw, "lark_md") {
		t.Fatalf("card branding=%s", raw)
	}
	if strings.Contains(raw, "One-2-All") || strings.Contains(raw, "one2all") {
		t.Fatalf("card still uses One2All branding=%s", raw)
	}
	if !strings.Contains(raw, "](https://pm-main.example.invalid/workspace/quality-management/bugs/4745)") {
		t.Fatalf("card link was not markdown=%s", raw)
	}
}
