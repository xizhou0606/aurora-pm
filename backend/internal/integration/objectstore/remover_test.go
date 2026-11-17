package objectstore

import (
	"context"
	"errors"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestDisabledRemovalAdapterIsConcreteAndFailsExplicitly(t *testing.T) {
	adapter := NewRemovalAdapter(config.ObjectStorage{Enabled: false})
	if adapter == nil {
		t.Fatal("disabled object storage must still provide a concrete adapter")
	}
	if err := adapter.RemoveObject(context.Background(), "workspace/7/task/9/file"); !errors.Is(err, ErrAttachmentStorageUnavailable) {
		t.Fatalf("RemoveObject error=%v", err)
	}
}
