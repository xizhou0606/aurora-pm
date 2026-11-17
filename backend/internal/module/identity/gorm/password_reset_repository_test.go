package identitygorm

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

func TestPasswordResetRepositoryForcesAndCompletesPasswordChange(t *testing.T) {
	db := newPersonnelFixture(t)
	if err := db.AutoMigrate(&PasswordResetTransaction{}); err != nil {
		t.Fatal(err)
	}
	createPersonnelUsers(t, db,
		User{ID: 1, Username: "admin", DisplayName: "Admin", Active: true, Status: string(identity.AccountActive), SessionVersion: 1},
		User{ID: 2, Username: "member", DisplayName: "Member", Active: true, Status: string(identity.AccountActive), SessionVersion: 1},
	)
	repository := NewAccessRepository(db)
	now := time.Date(2026, 9, 9, 4, 0, 0, 0, time.UTC)
	if err := repository.CreatePasswordReset(context.Background(), identity.PasswordResetRequest{
		TokenHash: "first", ActorUserID: 1, TargetUserID: 2,
		CreatedAt: now, ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	var user User
	if err := db.First(&user, 2).Error; err != nil {
		t.Fatal(err)
	}
	if !user.PasswordResetRequired || user.SessionVersion != 2 {
		t.Fatalf("forced user=%+v", user)
	}
	if err := repository.CreatePasswordReset(context.Background(), identity.PasswordResetRequest{
		TokenHash: "second", ActorUserID: 1, TargetUserID: 2,
		CreatedAt: now.Add(time.Minute), ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("new-password-value"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CompletePasswordReset(context.Background(), "first", string(hash), now.Add(2*time.Minute)); err != identity.ErrPasswordResetInvalid {
		t.Fatalf("superseded link err=%v", err)
	}
	if err := repository.CompletePasswordReset(context.Background(), "second", string(hash), now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&user, 2).Error; err != nil {
		t.Fatal(err)
	}
	if user.PasswordResetRequired || user.SessionVersion != 4 || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("new-password-value")) != nil {
		t.Fatalf("completed user=%+v", user)
	}
	if err := repository.CompletePasswordReset(context.Background(), "second", string(hash), now.Add(3*time.Minute)); err != identity.ErrPasswordResetInvalid {
		t.Fatalf("replay err=%v", err)
	}
}

func TestPasswordResetRepositoryRollsBackForcedStateWhenNotificationCannotQueue(t *testing.T) {
	db := newPersonnelFixture(t)
	if err := db.AutoMigrate(&PasswordResetTransaction{}); err != nil {
		t.Fatal(err)
	}
	createPersonnelUsers(t, db,
		User{ID: 1, Username: "admin", Active: true, Status: string(identity.AccountActive), SessionVersion: 1},
		User{ID: 2, Username: "member", Active: true, Status: string(identity.AccountActive), SessionVersion: 1},
	)
	repository := NewAccessRepository(db)
	queueFailure := errors.New("outbox unavailable")
	repository.SetPasswordResetNotificationWriter(func(
		*gorm.DB,
		uint,
		identity.PasswordResetRequest,
		User,
		User,
	) error {
		return queueFailure
	})
	now := time.Date(2026, 9, 9, 4, 0, 0, 0, time.UTC)
	err := repository.CreatePasswordReset(context.Background(), identity.PasswordResetRequest{
		TokenHash: "notify", ActorUserID: 1, TargetUserID: 2, WorkspaceID: 2,
		Notify: true, Link: "https://pm.test/reset-password#token=once",
		CreatedAt: now, ExpiresAt: now.Add(time.Hour),
	})
	if !errors.Is(err, queueFailure) {
		t.Fatalf("create err=%v", err)
	}
	var resetCount int64
	if err := db.Model(&PasswordResetTransaction{}).Count(&resetCount).Error; err != nil {
		t.Fatal(err)
	}
	var user User
	if err := db.First(&user, 2).Error; err != nil {
		t.Fatal(err)
	}
	if resetCount != 0 || user.PasswordResetRequired || user.SessionVersion != 1 {
		t.Fatalf("rollback reset_count=%d user=%+v", resetCount, user)
	}
}
