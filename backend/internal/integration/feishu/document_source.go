package feishu

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai/docsource"
	"strings"
)

type DocumentSourceProvider struct{ Reader Reader }

func (p DocumentSourceProvider) Kind() string { return docsource.Feishu }
func (p DocumentSourceProvider) Read(ctx context.Context, _ docsource.ReadContext, ref docsource.SourceRef) (docsource.Snapshot, error) {
	if p.Reader == nil {
		return docsource.Snapshot{}, docsource.ErrUnavailable
	}
	body, err := p.Reader.Read(ctx, ref.URL)
	if err != nil {
		return docsource.Snapshot{}, err
	}
	if strings.TrimSpace(body.Content) == "" || body.ContentTruncated || body.BlockRefsTruncated || body.Revision == "" || body.ContentHash == "" {
		return docsource.Snapshot{}, docsource.ErrUnavailable
	}
	return docsource.Snapshot{Type: "feishu_document", URL: body.URL, Title: body.Title, Content: body.Content, Format: "plain_text", Revision: body.Revision, ContentHash: body.ContentHash, BlockRefs: body.BlockRefs, Available: true, OriginalContentChars: body.OriginalContentChars}, nil
}
