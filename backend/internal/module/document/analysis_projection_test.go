package document

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAnalysisProjectionPreservesStructureAndDoesNotFetchReferences(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"验收"}]},{"type":"taskList","content":[{"type":"taskItem","attrs":{"checked":true},"content":[{"type":"paragraph","content":[{"type":"text","text":"完成"}]}]}]},{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableHeader","attrs":{"colspan":2,"rowspan":1},"content":[{"type":"paragraph","content":[{"type":"text","text":"条件"}]}]},{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"结果"}]}]}]}]},{"type":"paragraph","content":[{"type":"mention","attrs":{"label":"小明"}},{"type":"text","text":"链接","marks":[{"type":"link","attrs":{"href":"https://example.com/doc?token=secret#secret"}}]}]},{"type":"image","attrs":{"src":"https://example.com/image?token=secret","alt":"状态图"}},{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"a\nb"}]}]}`)
	value, err := ProjectAnalysisText(raw, SchemaVersionV2, 7)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"## 验收", "- [x] 完成", "[tableHeader colspan=2 rowspan=1]", "[tableCell colspan=1 rowspan=1]", "@小明", "https://example.com/doc; linked content not expanded", "[unexpanded image: 状态图]", "```go\na\nb\n```"} {
		if !strings.Contains(value.Text, part) {
			t.Errorf("missing %q in %s", part, value.Text)
		}
	}
	if strings.Contains(value.Text, "secret") || len(value.Unexpanded) == 0 || len(value.BlockRefs) == 0 || !strings.HasPrefix(value.BlockRefs[0], "revision:7/") {
		t.Fatalf("unsafe or missing metadata: %+v", value)
	}
	again, err := ProjectAnalysisText(raw, SchemaVersionV2, 7)
	if err != nil || again.Text != value.Text || again.ProjectionHash != value.ProjectionHash {
		t.Fatal("projection is not deterministic")
	}
	_, hash, _, err := NormalizeContentForSchema(raw, SchemaVersionV2)
	if err != nil || hash != value.ContentHash || hash == value.ProjectionHash {
		t.Fatal("source and projection hashes conflated")
	}
}
func TestAnalysisProjectionFailsClosed(t *testing.T) {
	deep := `{"type":"text","text":"x"}`
	for i := 0; i < 66; i++ {
		deep = `{"type":"paragraph","content":[` + deep + `]}`
	}
	for _, raw := range []string{`{"type":"doc","content":[{"type":"unsupportedBlock"}]}`, `{"type":"doc","content":[{"type":"unknown"}]}`, `{"type":"doc","content":[` + deep + `]}`, strings.Repeat(" ", analysisMaxBytes+1)} {
		if _, err := ProjectAnalysisText(json.RawMessage(raw), SchemaVersionV3, 1); err == nil {
			t.Fatal("accepted incomplete or oversized content")
		}
	}
}
func TestAnalysisProjectionV1V3AndEmpty(t *testing.T) {
	for _, version := range []uint{SchemaVersionV1, SchemaVersionV2, SchemaVersionV3} {
		p, err := ProjectAnalysisText(DefaultContent(), version, 1)
		if err != nil || p.Text != "" {
			t.Fatalf("empty document: %+v %v", p, err)
		}
	}
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"bitable","attrs":{"schemaVersion":1,"resourceId":"00000000-0000-0000-0000-000000000001","tableId":"00000000-0000-0000-0000-000000000002","viewId":"00000000-0000-0000-0000-000000000003","height":300}}]}`)
	p, err := ProjectAnalysisText(raw, SchemaVersionV3, 1)
	if err != nil || len(p.Unexpanded) != 1 || !strings.Contains(p.Text, "unexpanded bitable") {
		t.Fatalf("bitable expanded silently: %+v %v", p, err)
	}
}
