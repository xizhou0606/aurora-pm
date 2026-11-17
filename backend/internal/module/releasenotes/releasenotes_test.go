package releasenotes

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	received  Note
	upsertErr error
	latest    Note
	latestOK  bool
	latestErr error
	list      []Note
	listErr   error
}

func (store *fakeStore) UpsertByVersion(_ context.Context, note Note) (Note, error) {
	store.received = note
	return note, store.upsertErr
}

func (store *fakeStore) Latest(_ context.Context) (Note, bool, error) {
	return store.latest, store.latestOK, store.latestErr
}

func (store *fakeStore) List(_ context.Context) ([]Note, error) {
	return store.list, store.listErr
}

func TestPublishRejectsBlankVersionAndContent(t *testing.T) {
	service := NewService(&fakeStore{})
	cases := []PublishInput{
		{Version: "", ContentMd: "# notes"},
		{Version: "   ", ContentMd: "# notes"},
		{Version: "1.2.3", ContentMd: ""},
		{Version: "1.2.3", ContentMd: " \t "},
	}
	for _, input := range cases {
		if _, err := service.Publish(context.Background(), input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("input=%+v err=%v, want ErrInvalidInput", input, err)
		}
	}
}

func TestPublishTrimsVersionAndForwardsWrite(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	note, err := service.Publish(context.Background(), PublishInput{
		Version: " 1.2.3 ", ReleaseSHA: " abc123 ",
		ContentMd: "# release\n\n- fix", Source: " deploy ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if note.Version != "1.2.3" || note.ReleaseSHA != "abc123" || note.Source != "deploy" {
		t.Fatalf("note=%+v", note)
	}
	if store.received.ContentMd != "# release\n\n- fix" {
		t.Fatalf("content must be stored verbatim, got %q", store.received.ContentMd)
	}
	if !strings.Contains(note.ContentMd, "release") {
		t.Fatalf("note=%+v", note)
	}
}

func TestPublishPropagatesStoreConflict(t *testing.T) {
	store := &fakeStore{upsertErr: ErrVersionConflict}
	service := NewService(store)
	_, err := service.Publish(context.Background(), PublishInput{Version: "1.2.3", ContentMd: "x"})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("err=%v, want ErrVersionConflict", err)
	}
}

func TestListDelegatesToStore(t *testing.T) {
	want := []Note{{Version: "2.0.0", CreatedAt: time.Now().UTC()}}
	store := &fakeStore{list: want}
	service := NewService(store)
	got, err := service.List(context.Background())
	if err != nil || len(got) != 1 || got[0].Version != "2.0.0" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestLatestDelegatesToStore(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	if _, found, err := service.Latest(context.Background()); err != nil || found {
		t.Fatalf("empty store: found=%v err=%v, want found=false", found, err)
	}
	store.latest, store.latestOK = Note{Version: "1.8.0"}, true
	note, found, err := service.Latest(context.Background())
	if err != nil || !found || note.Version != "1.8.0" {
		t.Fatalf("note=%+v found=%v err=%v", note, found, err)
	}
}
