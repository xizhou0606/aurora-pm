package objectstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

var ErrAttachmentStorageUnavailable = errors.New("attachment object storage is unavailable")

// RemovalAdapter is always a concrete production dependency. Disabled or
// invalid storage remains an explicit retryable runtime failure rather than a
// nil dependency or a successful no-op; a process restart with repaired
// configuration can resume the durable deletion saga.
type RemovalAdapter struct {
	client *Client
	err    error
}

func NewRemovalAdapter(cfg config.ObjectStorage) *RemovalAdapter {
	client, err := New(cfg)
	if err == nil && client == nil {
		err = ErrAttachmentStorageUnavailable
	}
	return &RemovalAdapter{client: client, err: err}
}

func (adapter *RemovalAdapter) RemoveObject(ctx context.Context, objectKey string) error {
	if adapter == nil || adapter.client == nil {
		if adapter != nil && adapter.err != nil {
			return fmt.Errorf("%w: %v", ErrAttachmentStorageUnavailable, adapter.err)
		}
		return ErrAttachmentStorageUnavailable
	}
	if err := adapter.client.RemoveObject(ctx, objectKey); err != nil {
		return fmt.Errorf("%w: %v", ErrAttachmentStorageUnavailable, err)
	}
	return nil
}
