package httpapi

import (
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
)

// Notification copy resolves only PM internal IDs, even when a different
// user has the same external import ID.
func TestNotificationPeopleLabelResolvesInternalUserID(t *testing.T) {
	app := newTestApp(t, nil)
	factUser := identitygorm.User{
		ID: 7, ExternalSubject: "ext:7", Username: "operator_ext", DisplayName: "张三", Active: true,
	}
	localOne := identitygorm.User{
		ID: 1, ExternalSubject: "local:1",
		Username: "local_one", DisplayName: "本地一号", Active: true,
	}
	pmOnly := identitygorm.User{
		ID: 3, ExternalSubject: "local:3",
		Username: "pm_only", DisplayName: "李四", Active: true,
	}
	for _, user := range []identitygorm.User{factUser, localOne, pmOnly} {
		if err := app.db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}

	ctx := notificationUsersTx(app.db, 1, 3, 7)
	if label := notificationPeopleLabel(app.db, ctx, 7); label != "张三" {
		t.Fatalf("fact user label = %q, want 张三", label)
	}
	if label := notificationPeopleLabel(app.db, ctx, 3); label != "李四" {
		t.Fatalf("PM-only user label = %q, want 李四", label)
	}
	if label := workflowNotificationUserLabel(app.db, 7); label != "张三" {
		t.Fatalf("workflow fact user label = %q, want 张三", label)
	}
	if label := notificationPeopleLabel(app.db, ctx, 1); label != "本地一号" {
		t.Fatalf("external collision hijacked primary identity: %q", label)
	}
	if label := workflowNotificationUserLabel(app.db, 99); label != "用户 #99" {
		t.Fatalf("missing user label = %q, want 用户 #99", label)
	}
}
