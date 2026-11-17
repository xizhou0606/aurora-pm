package feishu

import (
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

// Allowlist entries are match suffixes (often with a leading dot); the built
// viewer link must never carry that dot into the host.
func TestDocumentURLNormalizesAllowlistSuffix(t *testing.T) {
	cases := []struct {
		allowlist []string
		want      string
	}{
		{allowlist: []string{".feishu.cn"}, want: "https://feishu.cn/docx/ARsy"},
		{allowlist: []string{"feishu.cn"}, want: "https://feishu.cn/docx/ARsy"},
		{allowlist: []string{"https://*.feishu.cn"}, want: "https://feishu.cn/docx/ARsy"},
		{allowlist: nil, want: "https://feishu.cn/docx/ARsy"},
	}
	for _, testCase := range cases {
		reader := &SDKReader{cfg: config.Feishu{DomainAllowlist: testCase.allowlist}}
		if got := reader.documentURL("ARsy"); got != testCase.want {
			t.Fatalf("allowlist=%v got %q, want %q", testCase.allowlist, got, testCase.want)
		}
	}
}
