package feishu

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai/docsource"
	"testing"
)

type sourceReaderStub struct{ body Document }

func (r sourceReaderStub) Read(context.Context, string) (Document, error) { return r.body, nil }
func TestDocumentSourceAdapterPreservesFeishuBodyAndFailsClosed(t *testing.T) {
	ref := docsource.SourceRef{Kind: docsource.Feishu, URL: "https://example.feishu.cn/docx/abc"}
	for _, scenario := range []string{"complete", "missing configuration", "truncated", "missing revision", "empty"} {
		t.Run(scenario, func(t *testing.T) {
			body := Document{URL: ref.URL, Title: "需求", Content: "完整正文", Revision: "7", ContentHash: "hash", BlockRefs: []string{"block"}, OriginalContentChars: 4}
			switch scenario {
			case "truncated":
				body.ContentTruncated = true
			case "missing revision":
				body.Revision = ""
			case "empty":
				body.Content = ""
			}
			provider := DocumentSourceProvider{Reader: sourceReaderStub{body: body}}
			if scenario == "missing configuration" {
				provider.Reader = nil
			}
			result, err := provider.Read(context.Background(), docsource.ReadContext{}, ref)
			if scenario != "complete" {
				if err == nil {
					t.Fatal("incomplete source accepted")
				}
				return
			}
			if err != nil || result.Type != "feishu_document" || result.Content != body.Content || result.ContentHash != body.ContentHash || result.Revision != "7" || !result.Available {
				t.Fatalf("snapshot %+v err %v", result, err)
			}
		})
	}
}
