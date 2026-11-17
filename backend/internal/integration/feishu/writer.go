package feishu

import (
	"context"
	"fmt"
	"strings"

	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
)

// CreatedDocument references a Feishu document created from PM content.
type CreatedDocument struct {
	DocumentID string
	URL        string
}

// CreateTextDocument creates a Feishu docx document titled as requested and
// fills it from the markdown body: headings, callouts, tables, lists, and
// bold/inline-code runs become native docx blocks, so the published report
// reads like a Feishu document instead of a plain-text dump. Unrecognized
// markdown degrades to text paragraphs. It shares the SDK client and tenant
// credentials with the document reader, so the Feishu application must have
// the docx write scope enabled.
func (r *SDKReader) CreateTextDocument(ctx context.Context, title, content string) (CreatedDocument, error) {
	if r == nil || r.client == nil {
		return CreatedDocument{}, ErrFeishuWriterUnavailable
	}
	create, err := r.client.Docx.V1.Document.Create(ctx, larkdocx.NewCreateDocumentReqBuilder().
		Body(larkdocx.NewCreateDocumentReqBodyBuilder().Title(title).Build()).
		Build())
	if err != nil {
		return CreatedDocument{}, fmt.Errorf("create Feishu document: %w", err)
	}
	if create == nil || create.Data == nil || create.Data.Document == nil || create.Data.Document.DocumentId == nil {
		return CreatedDocument{}, fmt.Errorf("create Feishu document: empty response")
	}
	documentID := *create.Data.Document.DocumentId
	// Primary path: Feishu's markdown convert API produces the block tree;
	// the hand-rolled parser only runs when conversion is unavailable.
	converted, convertErr := r.convertMarkdownBlocks(ctx, content)
	if convertErr == nil {
		if err := r.insertConvertedBlocks(ctx, documentID, converted); err != nil {
			return CreatedDocument{}, err
		}
		return CreatedDocument{DocumentID: documentID, URL: r.documentURL(documentID)}, nil
	}
	if err := r.appendParsedBlocks(ctx, documentID, content); err != nil {
		return CreatedDocument{}, err
	}
	return CreatedDocument{DocumentID: documentID, URL: r.documentURL(documentID)}, nil
}

// maxBlocksPerRequest follows the Feishu docx block-children batch limit.
const maxBlocksPerRequest = 50

func (r *SDKReader) createBlocks(ctx context.Context, documentID string, blocks []*larkdocx.Block) error {
	response, err := r.client.Docx.V1.DocumentBlockChildren.Create(ctx, larkdocx.NewCreateDocumentBlockChildrenReqBuilder().
		DocumentId(documentID).
		BlockId(documentID).
		Body(larkdocx.NewCreateDocumentBlockChildrenReqBodyBuilder().Children(blocks).Build()).
		Build())
	if err != nil {
		return fmt.Errorf("write Feishu document content: %w", err)
	}
	if response != nil && response.Code != 0 {
		return fmt.Errorf("write Feishu document content: code %d: %s", response.Code, response.Msg)
	}
	return nil
}

// documentURL builds the canonical viewer URL from the first allowlisted
// domain. Allowlist entries are match suffixes (".feishu.cn" or "feishu.cn"),
// so normalize them into a host: strip protocol, leading dots, and trailing
// slashes — otherwise the link renders as https://.feishu.cn/... and breaks.
func (r *SDKReader) documentURL(documentID string) string {
	for _, entry := range r.cfg.DomainAllowlist {
		domain := strings.ToLower(strings.TrimSpace(entry))
		domain = strings.TrimPrefix(domain, "https://")
		domain = strings.TrimPrefix(domain, "http://")
		domain = strings.TrimPrefix(domain, "*.")
		domain = strings.Trim(domain, "./")
		if domain == "" || strings.Contains(domain, "*") {
			continue
		}
		return "https://" + domain + "/docx/" + documentID
	}
	return "https://feishu.cn/docx/" + documentID
}

// ErrFeishuWriterUnavailable reports that the Feishu writer cannot run.
var ErrFeishuWriterUnavailable = fmt.Errorf("Feishu document writer is unavailable")
