package httpapi

import (
	"reflect"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
)

func TestCommentMentionRecipientIDsSupportsPlainAndRichMentions(t *testing.T) {
	ctx := workspaceContext{Users: map[uint]identitygorm.User{
		42: {Username: "teo"},
		43: {Username: "Alice"},
	}}
	plain := commentMentionRecipientIDs("@teo 请复核 @ALICE", ctx)
	if !reflect.DeepEqual(plain, []uint{42, 43}) {
		t.Fatalf("plain mention ids=%v", plain)
	}
	rich := commentMentionRecipientIDs(
		`{"type":"doc","content":[{"type":"mention","attrs":{"id":"77"}}]}`,
		ctx,
	)
	if !reflect.DeepEqual(rich, []uint{77}) {
		t.Fatalf("rich mention ids=%v", rich)
	}
}

func TestCommentNotificationRecipientsExcludeActorAndDedupe(t *testing.T) {
	got := commentNotificationRecipients(42, []uint{41, 42}, []uint{41, 43}, []uint{43, 44})
	if !reflect.DeepEqual(got, []uint{41, 43, 44}) {
		t.Fatalf("recipients=%v", got)
	}
}
