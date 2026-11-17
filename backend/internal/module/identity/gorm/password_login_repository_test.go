package identitygorm

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestPasswordLoginUsernameAndHistoricalEmail(t *testing.T) {
	tests := []struct {
		name, input string
		users       []User
		want        uint
	}{
		{"exact username", " Alice ", []User{{ID: 1, Username: "Alice", Email: "first@example.com"}}, 1},
		{"username remains case sensitive", "alice", []User{{ID: 1, Username: "Alice", Email: "first@example.com"}}, 0},
		{"email normalized case whitespace", " FIRST@EXAMPLE.COM ", []User{{ID: 1, Username: "Alice", Email: " First@Example.COM "}}, 1},
		{"duplicate normalized email", "first@example.com", []User{{ID: 1, Username: "Alice", Email: "first@example.com"}, {ID: 2, Username: "Bob", Email: " FIRST@EXAMPLE.COM "}}, 0},
		{"cross user username email collision", "first@example.com", []User{{ID: 1, Username: "first@example.com", Email: "other@example.com"}, {ID: 2, Username: "Bob", Email: "first@example.com"}}, 0},
		{"same user username and email", "first@example.com", []User{{ID: 1, Username: "first@example.com", Email: "first@example.com"}}, 1},
		{"legacy mailbox username", "first@example.com", []User{{ID: 1, Username: "first@example.com"}}, 1},
		{"empty", "  ", []User{{ID: 1, Username: "Alice"}}, 0},
		{"invalid email no fallback", "not-email", []User{{ID: 1, Username: "Alice", Email: "not-email"}}, 0},
		{"display name wrapper no fallback", "Alice <first@example.com>", []User{{ID: 1, Username: "Alice", Email: "first@example.com"}}, 0},
		{"missing", "missing@example.com", []User{{ID: 1, Username: "Alice", Email: "first@example.com"}}, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err = db.AutoMigrate(&User{}); err != nil {
				t.Fatal(err)
			}
			for i := range test.users {
				test.users[i].ExternalSubject = fmt.Sprint(test.users[i].ID)
				test.users[i].PasswordHash = "opaque-password-hash"
				test.users[i].Status = "banned"
				test.users[i].PasswordResetRequired = true
				if err = db.Create(&test.users[i]).Error; err != nil {
					t.Fatal(err)
				}
			}
			user, err := NewLoginRepository(db).FindPasswordLoginUser(context.Background(), test.input)
			if test.want == 0 {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					t.Fatalf("want invalid credentials, user=%d err=%v", user.ID, err)
				}
				return
			}
			if err != nil || user.ID != test.want {
				t.Fatalf("user=%d want=%d err=%v", user.ID, test.want, err)
			}
			if user.PasswordHash != "opaque-password-hash" || user.Status != "banned" || !user.PasswordResetRequired {
				t.Fatal("query must preserve password and state for existing login checks")
			}
		})
	}
}
