package requirement

import "context"

// DocumentSource is the complete association fact, not a filtered UI candidate list.
type DocumentSource struct {
	RequirementID uint
	Version       uint
	ProjectID     uint
	ProductDocURL string
	ResourceIDs   []string
}
type DocumentSourceReader interface {
	ReadDocumentSource(context.Context, uint, uint) (DocumentSource, error)
}
