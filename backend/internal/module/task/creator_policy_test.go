package task

import (
	"context"
	"testing"
)

type creatorPolicyStoreStub struct{ creators map[uint]uint }

func (stub creatorPolicyStoreStub) CreatorIDs(
	context.Context,
	uint,
	[]uint,
) (map[uint]uint, error) {
	return stub.creators, nil
}

func TestCreatorPolicyReturnsPerTaskFailClosedDecisions(t *testing.T) {
	service, err := NewCreatorPolicyService(creatorPolicyStoreStub{
		creators: map[uint]uint{11: 7, 12: 8},
	})
	if err != nil {
		t.Fatal(err)
	}

	allowed, err := service.AllowedCreatedBy(context.Background(), 3, 7, []uint{11, 12, 13})
	if err != nil {
		t.Fatal(err)
	}
	if !allowed[11] || allowed[12] || allowed[13] {
		t.Fatalf("creator decisions = %#v", allowed)
	}
	all, err := service.AllCreatedBy(context.Background(), 3, 7, []uint{11, 12})
	if err != nil {
		t.Fatal(err)
	}
	if all {
		t.Fatal("mixed-creator batch must fail closed")
	}
}
