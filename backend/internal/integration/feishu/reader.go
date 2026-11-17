package feishu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
	larkwiki "github.com/larksuite/oapi-sdk-go/v3/service/wiki/v2"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const (
	sdkRequestTimeout     = 8 * time.Second
	documentCoverMaxBytes = 20 * 1024 * 1024
)

var ErrDocumentCoverUnavailable = errors.New("feishu document cover unavailable")

type Document struct {
	URL                  string   `json:"url"`
	DocumentID           string   `json:"document_id"`
	Revision             string   `json:"revision"`
	Title                string   `json:"title"`
	Content              string   `json:"content"`
	ContentHash          string   `json:"content_hash"`
	OriginalContentChars int      `json:"original_content_chars"`
	ContentTruncated     bool     `json:"content_truncated"`
	BlockRefs            []string `json:"block_refs"`
	BlockRefsTruncated   bool     `json:"block_refs_truncated"`
}

type Reader interface {
	Read(context.Context, string) (Document, error)
}

type DocumentCover struct {
	Content      []byte
	ContentType  string
	OffsetRatioX float64
	OffsetRatioY float64
}

type CoverReader interface {
	ReadCover(context.Context, string) (DocumentCover, error)
}

type SDKReader struct {
	cfg                     config.Feishu
	client                  *lark.Client
	httpClient              *http.Client
	coverExportCoordinator  *documentCoverExportCoordinator
	coverMediaCoordinator   *documentCoverMediaCoordinator
	coverRenderer           DocumentCoverRenderer
	coverExportPollInterval time.Duration
	coverExportTimeout      time.Duration
}

type SDKReaderOption func(*SDKReader)

func WithDocumentCoverRenderer(renderer DocumentCoverRenderer) SDKReaderOption {
	return func(reader *SDKReader) {
		reader.coverRenderer = renderer
	}
}

func New(cfg config.Feishu, options ...SDKReaderOption) *SDKReader {
	httpClient := newBoundedFeishuHTTPClient(&http.Client{Timeout: sdkRequestTimeout})
	reader := &SDKReader{
		cfg: cfg, client: lark.NewClient(cfg.AppID, cfg.AppSecret, lark.WithHttpClient(httpClient)),
		httpClient: httpClient, coverExportCoordinator: globalDocumentCoverExportCoordinator,
		coverMediaCoordinator:   globalDocumentCoverMediaCoordinator,
		coverExportPollInterval: documentCoverExportPollInterval,
		coverExportTimeout:      documentCoverExportTimeout,
	}
	for _, option := range options {
		if option != nil {
			option(reader)
		}
	}
	return reader
}

func (r *SDKReader) Read(ctx context.Context, rawURL string) (Document, error) {
	documentID, normalized, metadata, err := r.documentMetadata(ctx, rawURL)
	if err != nil {
		return Document{}, err
	}
	raw, err := r.client.Docx.V1.Document.RawContent(ctx, larkdocx.NewRawContentDocumentReqBuilder().DocumentId(documentID).Build())
	if err != nil {
		return Document{}, fmt.Errorf("read document content: %w", err)
	}
	if !raw.Success() || raw.Data == nil || raw.Data.Content == nil {
		return Document{}, fmt.Errorf("read document content: %s", raw.Msg)
	}
	content, originalContentChars, contentTruncated := truncateRunesWithMetadata(*raw.Data.Content, r.cfg.MaxContentChars)
	refs, blockRefsTruncated, err := r.blockRefs(ctx, documentID)
	if err != nil {
		return Document{}, err
	}
	hash := sha256.Sum256([]byte(content))
	document := Document{
		URL: normalized, DocumentID: documentID, Content: content,
		ContentHash: hex.EncodeToString(hash[:]), OriginalContentChars: originalContentChars,
		ContentTruncated: contentTruncated, BlockRefs: refs, BlockRefsTruncated: blockRefsTruncated,
	}
	if metadata.RevisionId != nil {
		document.Revision = strconv.Itoa(*metadata.RevisionId)
	}
	if metadata.Title != nil {
		document.Title = *metadata.Title
	}
	return document, nil
}

func (r *SDKReader) ReadCover(ctx context.Context, rawURL string) (DocumentCover, error) {
	documentID, _, metadata, err := r.documentMetadata(ctx, rawURL)
	if err != nil {
		return DocumentCover{}, err
	}
	if metadata.Cover == nil || metadata.Cover.Token == nil || strings.TrimSpace(*metadata.Cover.Token) == "" {
		return r.exportDocumentCover(ctx, documentID)
	}
	return r.downloadDocumentNativeCover(ctx, *metadata.Cover.Token,
		metadata.Cover.OffsetRatioX, metadata.Cover.OffsetRatioY)
}

func clampDocumentCoverOffset(value *float64) float64 {
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return 0
	}
	return max(-0.5, min(0.5, *value))
}

func validDocumentCoverContentType(contentType string) bool {
	switch contentType {
	case "image/gif", "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func (r *SDKReader) documentMetadata(ctx context.Context, rawURL string) (string, string, *larkdocx.Document, error) {
	docType, token, normalized, err := parseDocumentURL(rawURL, r.cfg.DomainAllowlist)
	if err != nil {
		return "", "", nil, err
	}
	documentID := token
	if docType == "wiki" {
		request := larkwiki.NewGetNodeSpaceReqBuilder().Token(token).Build()
		response, readErr := r.client.Wiki.V2.Space.GetNode(ctx, request)
		if readErr != nil {
			return "", "", nil, fmt.Errorf("read wiki node: %w", readErr)
		}
		if !response.Success() || response.Data == nil || response.Data.Node == nil || response.Data.Node.ObjToken == nil {
			return "", "", nil, fmt.Errorf("read wiki node: %s", response.Msg)
		}
		if response.Data.Node.ObjType == nil || *response.Data.Node.ObjType != "docx" {
			return "", "", nil, fmt.Errorf("only Feishu docx documents are supported")
		}
		documentID = *response.Data.Node.ObjToken
	}
	metadata, err := r.client.Docx.V1.Document.Get(ctx, larkdocx.NewGetDocumentReqBuilder().DocumentId(documentID).Build())
	if err != nil {
		return "", "", nil, fmt.Errorf("read document metadata: %w", err)
	}
	if !metadata.Success() || metadata.Data == nil || metadata.Data.Document == nil {
		return "", "", nil, fmt.Errorf("read document metadata: %s", metadata.Msg)
	}
	return documentID, normalized, metadata.Data.Document, nil
}

func (r *SDKReader) blockRefs(ctx context.Context, documentID string) ([]string, bool, error) {
	request := larkdocx.NewListDocumentBlockReqBuilder().DocumentId(documentID).PageSize(500).Limit(r.cfg.MaxBlocks + 1).Build()
	iterator, err := r.client.Docx.V1.DocumentBlock.ListByIterator(ctx, request)
	if err != nil {
		return nil, false, fmt.Errorf("list document blocks: %w", err)
	}
	refs := make([]string, 0)
	for {
		hasNext, block, nextErr := iterator.Next()
		if nextErr != nil {
			return nil, false, fmt.Errorf("list document blocks: %w", nextErr)
		}
		if !hasNext {
			break
		}
		if block != nil && block.BlockId != nil {
			refs = append(refs, *block.BlockId)
		}
		if len(refs) > r.cfg.MaxBlocks {
			break
		}
	}
	truncated := len(refs) > r.cfg.MaxBlocks
	if truncated {
		refs = refs[:r.cfg.MaxBlocks]
	}
	return refs, truncated, nil
}

func parseDocumentURL(raw string, allowlist []string) (string, string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || (parsed.Port() != "" && parsed.Port() != "443") {
		return "", "", "", fmt.Errorf("valid Feishu https URL required")
	}
	host := strings.ToLower(parsed.Hostname())
	allowed := false
	for _, suffix := range allowlist {
		suffix = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(suffix)), ".")
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", "", "", fmt.Errorf("Feishu document domain is not allowed")
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) != 2 || (segments[0] != "wiki" && segments[0] != "docx") || strings.TrimSpace(segments[1]) == "" {
		return "", "", "", fmt.Errorf("Feishu wiki or docx URL required")
	}
	parsed.RawQuery, parsed.ForceQuery = "", false
	parsed.Fragment, parsed.RawFragment = "", ""
	return segments[0], segments[1], parsed.String(), nil
}

// NormalizeDocumentURL validates a supported Feishu wiki/docx URL without
// making a network request. Save paths use it to decide whether to enqueue a
// cover job; the worker performs the authoritative provider read later.
func NormalizeDocumentURL(raw string, allowlist []string) (string, error) {
	_, _, normalized, err := parseDocumentURL(raw, allowlist)
	return normalized, err
}

func truncateRunes(value string, limit int) string {
	truncated, _, _ := truncateRunesWithMetadata(value, limit)
	return truncated
}

func truncateRunesWithMetadata(value string, limit int) (string, int, bool) {
	runes := []rune(value)
	if limit <= 0 {
		return "", len(runes), len(runes) > 0
	}
	if len(runes) <= limit {
		return value, len(runes), false
	}
	return string(runes[:limit]), len(runes), true
}
