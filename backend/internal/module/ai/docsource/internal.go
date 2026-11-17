package docsource

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
)

type DocumentReader interface {
	GetDocument(context.Context, document.Principal, uint) (document.Document, error)
}
type InternalProvider struct {
	Documents DocumentReader
	Resources documentlibrary.Resolver
}

func (p InternalProvider) Kind() string { return Internal }
func (p InternalProvider) Read(ctx context.Context, actor ReadContext, ref SourceRef) (Snapshot, error) {
	if p.Documents == nil || p.Resources == nil || actor.UserID == 0 || actor.WorkspaceID == 0 || actor.FactUserID == 0 || ref.ResourceID == "" {
		return Snapshot{}, ErrUnavailable
	}
	if ref.ConverterVersion != "" && ref.ConverterVersion != document.AnalysisProjectionVersion {
		return Snapshot{}, ErrUnavailable
	}
	resource, binding, err := p.Resources.ResolveResource(ctx, actor.WorkspaceID, documentlibrary.ResourceID(ref.ResourceID))
	if err != nil || resource.TypeKey != documentlibrary.TypeRichText || binding.TypeKey != resource.TypeKey || binding.ResourceID != resource.ID {
		return Snapshot{}, ErrUnavailable
	}
	id, err := strconv.ParseUint(binding.PayloadRef, 10, 64)
	if err != nil || id == 0 || uint64(uint(id)) != id {
		return Snapshot{}, ErrUnavailable
	}
	body, err := p.Documents.GetDocument(ctx, document.Principal{WorkspaceID: actor.WorkspaceID, UserID: actor.UserID, ParticipantUserID: actor.FactUserID}, uint(id))
	if err != nil || body.ArchivedAt != nil || body.ResourceID != ref.ResourceID {
		return Snapshot{}, ErrUnavailable
	}
	projection, err := document.ProjectAnalysisText(body.Content, body.SchemaVersion, body.Revision)
	if errors.Is(err, document.ErrAnalysisContentLimit) || errors.Is(err, document.ErrAnalysisUnsupported) {
		return Snapshot{}, err
	}
	if err != nil || projection.ContentHash != body.ContentHash {
		return Snapshot{}, ErrUnavailable
	}
	if strings.TrimSpace(projection.Text) == "" {
		return Snapshot{}, errors.New("站内文档正文为空，无法执行 AI 分析")
	}
	return Snapshot{Type: Internal, ResourceID: ref.ResourceID, URL: "/workspace/documents/" + binding.PayloadRef, Title: resource.Name, Content: projection.Text, Format: "structured_text", Revision: strconv.FormatUint(uint64(body.Revision), 10), ContentHash: body.ContentHash, ProjectionHash: projection.ProjectionHash, ConverterVersion: projection.ConverterVersion, BlockRefs: projection.BlockRefs, Unexpanded: projection.Unexpanded, Available: true, OriginalContentChars: len([]rune(projection.Text))}, nil
}
