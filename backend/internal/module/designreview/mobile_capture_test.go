package designreview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

type collectorHashRepository struct {
	Repository
	pairingCodeHash string
	tokenHash       string
	authHash        string
}

func (repository *collectorHashRepository) PairMobileCollector(_ context.Context, codeHash string, input MobileCollectorRecord) (MobileCollector, error) {
	repository.pairingCodeHash = codeHash
	repository.tokenHash = input.TokenHash
	return MobileCollector{ID: input.ID, WorkspaceID: 7, UserID: 42}, nil
}

func (repository *collectorHashRepository) AuthenticateCollector(_ context.Context, tokenHash string) (MobileCollector, error) {
	repository.authHash = tokenHash
	return MobileCollector{ID: "collector", WorkspaceID: 7, UserID: 42}, nil
}

func TestCollectorCredentialsAreHashedBeforePersistence(t *testing.T) {
	repository := &collectorHashRepository{}
	service := &Service{repository: repository}
	input := PairCollectorInput{
		Code: "123456", Name: "Local Mac", Version: collectorProtocolVersion,
		MobilewrightVersion: mobilewrightVersion, Hostname: "pm-mac", Architecture: "arm64",
		Inventory: []byte(`{"devices":[],"doctor":{"ready":true,"summary":"ready"}}`),
	}
	result, err := service.PairCollector(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	codeDigest := sha256.Sum256([]byte(input.Code))
	if repository.pairingCodeHash != hex.EncodeToString(codeDigest[:]) || repository.pairingCodeHash == input.Code {
		t.Fatalf("pairing code was not hashed: %q", repository.pairingCodeHash)
	}
	if len(result.Token) != 64 || len(repository.tokenHash) != 64 || repository.tokenHash == result.Token {
		t.Fatalf("collector token persistence is not hash-only: token_len=%d hash=%q", len(result.Token), repository.tokenHash)
	}
	tokenDigest := sha256.Sum256([]byte(result.Token))
	if repository.tokenHash != hex.EncodeToString(tokenDigest[:]) {
		t.Fatal("persisted collector token hash does not match the issued token")
	}
	if _, err := service.AuthenticateCollector(context.Background(), result.Token); err != nil {
		t.Fatal(err)
	}
	if repository.authHash != repository.tokenHash {
		t.Fatalf("authentication hash=%q want=%q", repository.authHash, repository.tokenHash)
	}
}
