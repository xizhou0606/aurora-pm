package collaborationgorm

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	"gorm.io/gorm"
)

func postgresDocumentRepositories(t *testing.T) (*Repository, *Repository) {
	t.Helper()
	first, second := postgresLeaseRepositories(t)
	if err := first.db.AutoMigrate(&DocumentStateRecord{}, &DocumentUpdateRecord{}); err != nil {
		t.Fatal(err)
	}
	return first, second
}

func seedWithRepository(repository *Repository, input collaboration.DocumentSeed) error {
	return repository.db.Transaction(func(tx *gorm.DB) error {
		store, err := NewDocumentTransaction(tx)
		if err != nil {
			return err
		}
		_, err = store.SeedDocument(context.Background(), input)
		return err
	})
}

func commitWithRepository(repository *Repository, input collaboration.DocumentUpdate) error {
	return repository.db.Transaction(func(tx *gorm.DB) error {
		store, err := NewDocumentTransaction(tx)
		if err != nil {
			return err
		}
		_, err = store.CommitDocumentUpdate(context.Background(), collaboration.Principal{WorkspaceID: input.Room.WorkspaceID, UserID: input.ActorUserID}, input)
		return err
	})
}

func TestPostgresDocumentConcurrentSeedIsIdempotent(t *testing.T) {
	first, second := postgresDocumentRepositories(t)
	input := syntheticSeed(collaboration.Room{WorkspaceID: 11, ResourceID: uuid.NewString()}, "same-source")
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, repository := range []*Repository{first, second} {
		repository := repository
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			results <- seedWithRepository(repository, input)
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestPostgresDocumentConcurrentConflictingSeedHasOneWinner(t *testing.T) {
	first, second := postgresDocumentRepositories(t)
	room := collaboration.Room{WorkspaceID: 11, ResourceID: uuid.NewString()}
	inputs := []collaboration.DocumentSeed{syntheticSeed(room, "source-a"), syntheticSeed(room, "source-b")}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for index, repository := range []*Repository{first, second} {
		index, repository := index, repository
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			results <- seedWithRepository(repository, inputs[index])
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	winners, conflicts := 0, 0
	for err := range results {
		if err == nil {
			winners++
		} else if errors.Is(err, collaboration.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatalf("seed winners=%d conflicts=%d", winners, conflicts)
	}
}

func TestPostgresDocumentConcurrentUpdatesHaveOneRevisionWinner(t *testing.T) {
	first, second := postgresDocumentRepositories(t)
	room := collaboration.Room{WorkspaceID: 11, ResourceID: uuid.NewString()}
	seedInput := syntheticSeed(room, "seed")
	if err := seedWithRepository(first, seedInput); err != nil {
		t.Fatal(err)
	}
	state, err := first.LoadDocumentState(context.Background(), room)
	if err != nil {
		t.Fatal(err)
	}
	inputs := []collaboration.DocumentUpdate{
		syntheticUpdate(state, "operation-a", "update-a"),
		syntheticUpdate(state, "operation-b", "update-b"),
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for index, repository := range []*Repository{first, second} {
		index, repository := index, repository
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			results <- commitWithRepository(repository, inputs[index])
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	winners, conflicts := 0, 0
	for err := range results {
		if err == nil {
			winners++
		} else if errors.Is(err, collaboration.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatalf("update winners=%d conflicts=%d", winners, conflicts)
	}
}
