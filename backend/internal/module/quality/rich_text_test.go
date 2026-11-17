package quality

import "testing"

func TestRichTextPlainText(t *testing.T) {
	richText := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"请优先处理"}]},{"type":"paragraph","content":[{"type":"image","attrs":{"src":"https://objects.example.test/proof.png"}}]}]}`
	if got := RichTextPlainText(richText); got != "请优先处理\n\n[图片]" {
		t.Fatalf("plain text=%q", got)
	}
	if got := RichTextPlainText("普通说明"); got != "普通说明" {
		t.Fatalf("legacy plain text=%q", got)
	}
}

func TestRichTextPlainTextIncludesMentionDisplayName(t *testing.T) {
	richText := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"mention","attrs":{"id":"47","label":"铁奥","username":"teo"}},{"type":"text","text":" 请看"}]}]}`
	if got := RichTextPlainText(richText); got != "@铁奥 请看" {
		t.Fatalf("rich-text mention plain text=%q", got)
	}
}

func TestQualityRichTextRuneCountIgnoresSerializationOverhead(t *testing.T) {
	richText := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"说明"}]},{"type":"image","attrs":{"src":"https://objects.example.test/proof.png"}}]}`
	if got := RichTextRuneCount(richText); got != 8 {
		t.Fatalf("visible runes=%d want=8", got)
	}
}
