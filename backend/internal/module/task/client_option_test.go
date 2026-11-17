package task

import (
	"context"
	"errors"
	"testing"
)

type clientOptionRepositoryFake struct {
	defaultsWorkspace uint
	defaultsActor     uint
	rows              []ClientOptionRecord
	usage             map[uint]int64
	deleted           uint
}

func (fake *clientOptionRepositoryFake) EnsureDefaults(_ context.Context, workspaceID, actorUserID uint, _ []string) error {
	fake.defaultsWorkspace, fake.defaultsActor = workspaceID, actorUserID
	return nil
}
func (fake *clientOptionRepositoryFake) List(context.Context, uint) ([]ClientOptionRecord, error) {
	return fake.rows, nil
}
func (fake *clientOptionRepositoryFake) UsageCounts(context.Context, uint) (map[uint]int64, error) {
	return fake.usage, nil
}
func (fake *clientOptionRepositoryFake) Create(_ context.Context, _, _ uint, name, normalized string) (ClientOptionRecord, error) {
	return ClientOptionRecord{ID: 4, Name: name, NormalizedName: normalized}, nil
}
func (fake *clientOptionRepositoryFake) Update(_ context.Context, _, id uint, name, normalized string) (ClientOptionRecord, error) {
	return ClientOptionRecord{ID: id, Name: name, NormalizedName: normalized}, nil
}
func (fake *clientOptionRepositoryFake) Delete(_ context.Context, _, id uint) error {
	fake.deleted = id
	return nil
}

func TestClientOptionServiceSeedsDefaultsAndReportsUsage(t *testing.T) {
	repository := &clientOptionRepositoryFake{
		rows:  []ClientOptionRecord{{ID: 1, Name: "桌面端", NormalizedName: "桌面端"}},
		usage: map[uint]int64{1: 3},
	}
	service, err := NewClientOptionService(repository)
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.List(context.Background(), 7, 11)
	if err != nil || repository.defaultsWorkspace != 7 || repository.defaultsActor != 11 {
		t.Fatalf("items=%v workspace=%d actor=%d err=%v", items, repository.defaultsWorkspace, repository.defaultsActor, err)
	}
	if len(items) != 1 || items[0].UsageCount != 3 {
		t.Fatalf("items=%+v", items)
	}
}

func TestClientOptionServiceNormalizesNamesAndBlocksInUseDelete(t *testing.T) {
	repository := &clientOptionRepositoryFake{usage: map[uint]int64{4: 2}}
	service, _ := NewClientOptionService(repository)
	created, err := service.Create(context.Background(), 7, 11, "  Desktop  App ")
	if err != nil || created.Name != "Desktop App" {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	err = service.Delete(context.Background(), 7, 4)
	var inUse *ClientOptionInUseError
	if !errors.As(err, &inUse) || inUse.UsageCount != 2 || repository.deleted != 0 {
		t.Fatalf("err=%v deleted=%d", err, repository.deleted)
	}
}
