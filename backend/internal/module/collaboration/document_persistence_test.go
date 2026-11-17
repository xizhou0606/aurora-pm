package collaboration

import "testing"

func TestCanonicalJSONHashUsesStableCrossLanguageEncoding(t *testing.T) {
	got, err := CanonicalJSONHash([]byte("{\"z\":1,\"a\":\"<tag>&\\u2028\\u2029\"}"))
	if err != nil {
		t.Fatal(err)
	}
	want := SHA256Hex([]byte(`{"a":"<tag>&\u2028\u2029","z":1}`))
	if got != want {
		t.Fatalf("canonical hash = %q, want %q", got, want)
	}
}

func TestDocumentEnvelopeValidationRejectsUntrustedIdentifiers(t *testing.T) {
	seed := DocumentSeed{
		Room: Room{WorkspaceID: 1, ResourceID: " resource"}, SourceRevision: 1,
		SourceHash: SHA256Hex([]byte(`{}`)), Snapshot: []byte("snapshot"), SnapshotHash: SHA256Hex([]byte("snapshot")),
		StateVector: []byte("vector"), StateVectorHash: SHA256Hex([]byte("vector")),
		Projection: []byte(`{}`), ProjectionHash: SHA256Hex([]byte(`{}`)), ActorUserID: 1, PolicyRevision: 1,
	}
	if err := ValidateDocumentSeed(seed); err != ErrInvalidInput {
		t.Fatalf("invalid room accepted: %v", err)
	}
	seed.Room.ResourceID = "resource"
	if err := ValidateDocumentSeed(seed); err != nil {
		t.Fatalf("valid envelope rejected: %v", err)
	}
	update := DocumentUpdate{
		Room: seed.Room, OperationID: "../../escape", ExpectedDurableRevision: 1,
		BaseStateVectorHash: seed.StateVectorHash, Update: []byte("update"), UpdateHash: SHA256Hex([]byte("update")),
		Snapshot: seed.Snapshot, SnapshotHash: seed.SnapshotHash, StateVector: seed.StateVector,
		StateVectorHash: seed.StateVectorHash, Projection: seed.Projection, ProjectionHash: seed.ProjectionHash,
		ActorUserID: 1, PolicyRevision: 1,
	}
	if err := ValidateDocumentUpdate(update); err != ErrInvalidInput {
		t.Fatalf("invalid operation accepted: %v", err)
	}
}
