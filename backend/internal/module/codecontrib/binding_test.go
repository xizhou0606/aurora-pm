package codecontrib

import "testing"

func testBindingContext() BindingContext {
	return BindingContext{
		BotPatterns: []string{"[bot]", "actions-bot"},
		Bindings: []AuthorBinding{
			// 管理员确认的别名：老邮箱与老账号都指向 alice。
			{AuthorEmail: "old-alias@example.com", AuthorLogin: "alice-legacy", UserID: 1, MatchedVia: MatchAdminConfirmed},
			// 既往自动绑定的行（email_exact 留痕，可覆盖）。
			{AuthorEmail: "auto-bound@example.com", UserID: 2, MatchedVia: MatchEmailExact},
		},
		Candidates: []UserCandidate{
			{UserID: 1, Email: "alice@example.com", Username: "alice"},
			{UserID: 2, Email: "bob@example.com", Username: "bob"},
			{UserID: 3, Email: "alice.zhang@example.com", Username: "alice.zhang"},
			{UserID: 4, Email: "carol@example.com", Username: "carol"},
			// 故意的别名场景：old-alias 同时是某成员的注册邮箱。
			{UserID: 5, Email: "old-alias@example.com", Username: "dave"},
		},
	}
}

func TestResolveAuthorIdentity(t *testing.T) {
	tests := []struct {
		name            string
		identity        AuthorIdentity
		mutate          func(*BindingContext)
		wantStatus      BindingStatus
		wantUserID      uint
		wantSuggestedID uint
		wantVia         MatchVia
	}{
		{
			name:       "bot login is excluded",
			identity:   AuthorIdentity{Email: "bot@example.com", Login: "dependabot[bot]", Name: "dependabot"},
			wantStatus: BindingBot,
		},
		{
			name:       "bot display name is excluded",
			identity:   AuthorIdentity{Email: "ci@example.com", Name: "GitHub Actions-Bot"},
			wantStatus: BindingBot,
		},
		{
			name:       "admin-confirmed alias wins over candidate email",
			identity:   AuthorIdentity{Email: "old-alias@example.com", Name: "Alice"},
			wantStatus: BindingMatched,
			wantUserID: 1,
			wantVia:    MatchAdminConfirmed,
		},
		{
			name:       "admin-confirmed alias by provider login",
			identity:   AuthorIdentity{Login: "alice-legacy"},
			wantStatus: BindingMatched,
			wantUserID: 1,
			wantVia:    MatchAdminConfirmed,
		},
		{
			name:       "persisted email_exact row still resolves",
			identity:   AuthorIdentity{Email: "auto-bound@example.com"},
			wantStatus: BindingMatched,
			wantUserID: 2,
			wantVia:    MatchEmailExact,
		},
		{
			name:       "unique member email auto-binds",
			identity:   AuthorIdentity{Email: "ALICE@example.com"},
			wantStatus: BindingMatched,
			wantUserID: 1,
			wantVia:    MatchEmailExact,
		},
		{
			name:       "co-author trailer email follows the same pipeline",
			identity:   AuthorIdentity{Email: "carol@example.com"},
			wantStatus: BindingMatched,
			wantUserID: 4,
			wantVia:    MatchEmailExact,
		},
		{
			name:     "ambiguous email fails closed",
			identity: AuthorIdentity{Email: "alice.zhang@example.com"},
			mutate: func(context *BindingContext) {
				context.Candidates = append(context.Candidates,
					UserCandidate{UserID: 9, Email: "alice.zhang@example.com", Username: "other"})
			},
			wantStatus: BindingUnmatched,
		},
		{
			name:            "login==username only suggests by default",
			identity:        AuthorIdentity{Email: "mystery@example.com", Login: "alice.zhang"},
			wantStatus:      BindingSuggested,
			wantSuggestedID: 3,
		},
		{
			name:       "opt-in workspace auto-binds unique login",
			identity:   AuthorIdentity{Login: "alice.zhang"},
			mutate:     func(context *BindingContext) { context.AllowLoginExact = true },
			wantStatus: BindingMatched,
			wantUserID: 3,
			wantVia:    MatchLoginExact,
		},
		{
			name:            "github noreply email feeds the suggestion tier",
			identity:        AuthorIdentity{Email: "12345+alice@users.noreply.github.com"},
			wantStatus:      BindingSuggested,
			wantSuggestedID: 1,
		},
		{
			// PR 作者登录名与某条已确认邮箱绑定的本地部分一致 → 建议同人，
			// 不自动绑定（fail closed）。
			name:     "login matching a bound email local part is suggested",
			identity: AuthorIdentity{Login: "devtwo"},
			mutate: func(context *BindingContext) {
				context.Bindings = append(context.Bindings, AuthorBinding{
					AuthorEmail: "devtwo@example.test", UserID: 8, MatchedVia: MatchEmailExact,
				})
			},
			wantStatus:      BindingSuggested,
			wantSuggestedID: 8,
		},
		{
			// 本地部分命中多个不同用户 → 语义歧义，维持未匹配，不猜。
			name:     "ambiguous local-part hits stay unmatched",
			identity: AuthorIdentity{Login: "devtwo"},
			mutate: func(context *BindingContext) {
				context.Bindings = append(context.Bindings,
					AuthorBinding{AuthorEmail: "devtwo@example.test", UserID: 8, MatchedVia: MatchEmailExact},
					AuthorBinding{AuthorEmail: "devtwo@other.com", UserID: 9, MatchedVia: MatchEmailExact},
				)
			},
			wantStatus: BindingUnmatched,
		},
		{
			// 已驳回/未归属行（user_id 空）不参与本地部分匹配。
			name:     "rejected rows do not feed local-part suggestions",
			identity: AuthorIdentity{Login: "devtwo"},
			mutate: func(context *BindingContext) {
				context.Bindings = append(context.Bindings, AuthorBinding{
					AuthorEmail: "devtwo@example.test", UserID: 0,
				})
			},
			wantStatus: BindingUnmatched,
		},
		{
			name:       "unknown identity lands in the confirm queue",
			identity:   AuthorIdentity{Email: "nobody@example.com", Login: "nobody"},
			wantStatus: BindingUnmatched,
		},
		{
			name:       "identity without email or login is unmatched",
			identity:   AuthorIdentity{Name: "Ghost"},
			wantStatus: BindingUnmatched,
		},
		{
			name:       "empty context on any identity stays unmatched",
			identity:   AuthorIdentity{Email: "alice@example.com"},
			mutate:     func(context *BindingContext) { *context = BindingContext{} },
			wantStatus: BindingUnmatched,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context := testBindingContext()
			if test.mutate != nil {
				test.mutate(&context)
			}
			got := ResolveAuthorIdentity(context, test.identity)
			if got.Status != test.wantStatus {
				t.Fatalf("status = %q (%s), want %q", got.Status, got.Reason, test.wantStatus)
			}
			if test.wantStatus == BindingMatched {
				if got.UserID != test.wantUserID || got.MatchedVia != test.wantVia {
					t.Fatalf("matched = (user %d via %s), want (user %d via %s)",
						got.UserID, got.MatchedVia, test.wantUserID, test.wantVia)
				}
			}
			if test.wantStatus == BindingSuggested && got.SuggestedUserID != test.wantSuggestedID {
				t.Fatalf("suggested user = %d, want %d", got.SuggestedUserID, test.wantSuggestedID)
			}
			if got.Reason == "" {
				t.Fatalf("resolution must carry an auditable reason")
			}
		})
	}
}
