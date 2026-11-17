package report

import (
	"context"
)

// DocumentExporter publishes report content as an external document. The
// integration implementation owns the Feishu protocol; this domain only owns
// the business decision of what to publish.
type DocumentExporter interface {
	CreateReportDocument(ctx context.Context, title, content string) (CreatedDocument, error)
}

// CreatedDocument references a created external document.
type CreatedDocument struct {
	DocumentID string
	URL        string
}

// DocumentExporterFunc adapts a plain function to the exporter port.
type DocumentExporterFunc func(ctx context.Context, title, content string) (CreatedDocument, error)

func (fn DocumentExporterFunc) CreateReportDocument(
	ctx context.Context, title, content string,
) (CreatedDocument, error) {
	return fn(ctx, title, content)
}
