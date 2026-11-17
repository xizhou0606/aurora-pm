package bitable

import "context"

type MutationEffect struct {
	Principal                              Principal
	ResourceID, TableID, OperationID, Kind string
	RecordIDs                              []string
}

// Implementations are created with the storage transaction by the app layer.
type MutationRecorder interface {
	RecordBitableMutation(context.Context, MutationEffect) error
}
