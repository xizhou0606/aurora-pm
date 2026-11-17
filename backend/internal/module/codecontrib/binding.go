package codecontrib

import "strings"

// The binding pipeline resolves raw author identities (commit author email,
// provider login, co-author trailer) to PM platform user IDs.
//
// 解析顺序固定、结果确定、失败关闭：命中不了的作者进入待确认队列并单独展示
// 为"未归属活动"，绝不并入任何人的分数，也绝不默认猜一个账号。同一个人可以
// 有多条绑定行（多个邮箱、多个 provider 账号），聚合时按 user_id 归组。

// AuthorIdentity is the raw identity claim carried by a fact: a commit
// author's email/name, a provider account login, or a co-author trailer
// email. All three are unverified input.
type AuthorIdentity struct {
	Email string
	Name  string
	Login string
}

// BindingStatus is the pipeline outcome for one identity.
type BindingStatus string

const (
	BindingMatched   BindingStatus = "matched"   // resolved to a user
	BindingBot       BindingStatus = "bot"       // excluded, never in person aggregates
	BindingSuggested BindingStatus = "suggested" // proposal for the confirm queue
	BindingUnmatched BindingStatus = "unmatched" // pending manual confirmation
)

// MatchVia records how a binding was established, for audit and override.
type MatchVia string

const (
	MatchAdminConfirmed MatchVia = "admin_confirmed" // workspace admin decision, highest priority
	MatchEmailExact     MatchVia = "email_exact"     // unique email match against workspace members
	MatchLoginExact     MatchVia = "login_exact"     // unique login==username hit (opt-in)
)

// AuthorBinding mirrors one persisted mapping row. Email and Login are the
// author keys the row covers; either may be empty as long as one is set.
type AuthorBinding struct {
	AuthorEmail string
	AuthorLogin string
	UserID      uint
	MatchedVia  MatchVia
}

// UserCandidate is one workspace member's matching surface, supplied through
// the identity read port. Email and Username are comparison facts only.
type UserCandidate struct {
	UserID   uint
	Email    string
	Username string
}

// BindingContext is the workspace-scoped resolution input.
type BindingContext struct {
	// BotPatterns are case-insensitive substrings matched against email,
	// login and display name (e.g. "[bot]", "dependabot").
	BotPatterns []string
	Bindings    []AuthorBinding
	Candidates  []UserCandidate
	// AllowLoginExact promotes a unique login==username hit to an automatic
	// match. The fail-closed default only emits a suggestion.
	AllowLoginExact bool
}

// Resolution is the pipeline verdict for one identity. UserID is meaningful
// when Status is BindingMatched; SuggestedUserID when BindingSuggested.
// Reason is a human-readable audit line, safe for direct UI display.
type Resolution struct {
	Status          BindingStatus
	UserID          uint
	MatchedVia      MatchVia
	SuggestedUserID uint
	Reason          string
}

// ResolveAuthorIdentity runs the fixed pipeline: bot rules → admin-confirmed
// rows → other persisted rows → unique email match → login tiers（登录名与
// 成员用户名一致，或与某条已确认邮箱绑定的本地部分一致）→ unmatched。
// The first conclusive tier wins, so the outcome is a pure function of
// (identity, context) and stays auditable end to end.
func ResolveAuthorIdentity(context BindingContext, identity AuthorIdentity) Resolution {
	if resolution, hit := matchBotRules(context, identity); hit {
		return resolution
	}
	if resolution, hit := matchBindings(context.Bindings, identity, MatchAdminConfirmed); hit {
		return resolution
	}
	if resolution, hit := matchBindings(context.Bindings, identity, ""); hit {
		return resolution
	}
	if resolution, hit := matchByEmail(context.Candidates, identity); hit {
		return resolution
	}
	if resolution, hit := matchByLogin(context, identity); hit {
		return resolution
	}
	if resolution, hit := matchByBoundEmailLocalPart(context, identity); hit {
		return resolution
	}
	return Resolution{
		Status: BindingUnmatched,
		Reason: "无匹配的邮箱或账号，进入待确认队列",
	}
}

// matchBotRules is tier 1: bots never enter person aggregates, they are
// listed separately as repository activity.
func matchBotRules(context BindingContext, identity AuthorIdentity) (Resolution, bool) {
	haystacks := []string{identity.Email, identity.Login, identity.Name}
	for _, pattern := range context.BotPatterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		for _, haystack := range haystacks {
			if haystack != "" && strings.Contains(strings.ToLower(haystack), pattern) {
				return Resolution{
					Status: BindingBot,
					Reason: "命中机器人规则「" + pattern + "」，计入仓库活动但不计入个人贡献",
				}, true
			}
		}
	}
	return Resolution{}, false
}

// matchBindings is tier 2/3: persisted mapping rows. Admin-confirmed rows are
// consulted first and win over any automatic row covering the same key.
func matchBindings(bindings []AuthorBinding, identity AuthorIdentity, via MatchVia) (Resolution, bool) {
	for _, binding := range bindings {
		if binding.UserID == 0 {
			continue
		}
		if via != "" && binding.MatchedVia != via {
			continue
		}
		if via == "" && binding.MatchedVia == MatchAdminConfirmed {
			continue // already handled by the earlier tier
		}
		if identity.Email != "" && strings.EqualFold(binding.AuthorEmail, identity.Email) {
			return matchedResolution(binding.UserID, binding.MatchedVia), true
		}
		if identity.Login != "" && binding.AuthorLogin != "" &&
			strings.EqualFold(binding.AuthorLogin, identity.Login) {
			return matchedResolution(binding.UserID, binding.MatchedVia), true
		}
	}
	return Resolution{}, false
}

// matchByEmail is tier 4: an exact email hit binds automatically only when it
// matches exactly one workspace member; ambiguity fails closed into the
// confirm queue instead of guessing.
func matchByEmail(candidates []UserCandidate, identity AuthorIdentity) (Resolution, bool) {
	if identity.Email == "" {
		return Resolution{}, false
	}
	hits := make([]UserCandidate, 0, 2)
	for _, candidate := range candidates {
		if candidate.Email != "" && strings.EqualFold(candidate.Email, identity.Email) {
			hits = append(hits, candidate)
		}
	}
	switch len(hits) {
	case 1:
		return Resolution{
			Status:     BindingMatched,
			UserID:     hits[0].UserID,
			MatchedVia: MatchEmailExact,
			Reason:     "邮箱唯一命中工作区成员，自动绑定（可人工覆盖）",
		}, true
	case 0:
		return Resolution{}, false
	default:
		return Resolution{
			Status: BindingUnmatched,
			Reason: "邮箱命中多个成员账号，需人工确认，不自动绑定",
		}, true
	}
}

// matchByLogin is tier 5: a login identical to exactly one member username
// yields a confirm-queue suggestion by default and an automatic match only
// when the workspace opts in. GitHub noreply addresses
// (`12345+login@users.noreply.github.com`) feed the same suggestion tier from
// the embedded login. hit=false 落入下一层（已绑定邮箱本地部分）。
func matchByLogin(context BindingContext, identity AuthorIdentity) (Resolution, bool) {
	login := identity.Login
	if login == "" {
		login = githubNoreplyLogin(identity.Email)
	}
	if login == "" {
		return Resolution{
			Status: BindingUnmatched,
			Reason: "无匹配的邮箱或账号，进入待确认队列",
		}, false
	}
	hits := make([]UserCandidate, 0, 2)
	for _, candidate := range context.Candidates {
		if candidate.Username != "" && strings.EqualFold(candidate.Username, login) {
			hits = append(hits, candidate)
		}
	}
	switch len(hits) {
	case 0:
		return Resolution{
			Status: BindingUnmatched,
			Reason: "账号「" + login + "」无对应成员，进入待确认队列",
		}, false
	case 1:
		if context.AllowLoginExact {
			return Resolution{
				Status:     BindingMatched,
				UserID:     hits[0].UserID,
				MatchedVia: MatchLoginExact,
				Reason:     "账号与用户名唯一一致，自动绑定（工作区已开启该策略）",
			}, true
		}
		return Resolution{
			Status:          BindingSuggested,
			SuggestedUserID: hits[0].UserID,
			Reason:          "账号「" + login + "」与成员用户名一致，建议确认为该成员",
		}, true
	default:
		return Resolution{
			Status: BindingUnmatched,
			Reason: "账号「" + login + "」命中多个成员用户名，需人工确认",
		}, true
	}
}

// matchByBoundEmailLocalPart is tier 6: a login matching the local part of an
// already-bound email（例如 PR 作者登录名 devtwo 对应已绑定的
// devtwo@example.test）强烈暗示同一人，但自动绑定仍 fail closed——只给
// 出建议，由管理员一键确认。命中多个不同用户时维持未匹配，不猜。
func matchByBoundEmailLocalPart(context BindingContext, identity AuthorIdentity) (Resolution, bool) {
	login := strings.ToLower(strings.TrimSpace(identity.Login))
	if login == "" {
		return Resolution{}, false
	}
	hits := make(map[uint]string, 2)
	for _, binding := range context.Bindings {
		if binding.UserID == 0 || binding.AuthorEmail == "" {
			continue
		}
		local, _, found := strings.Cut(strings.ToLower(binding.AuthorEmail), "@")
		if !found || local == "" || local != login {
			continue
		}
		hits[binding.UserID] = binding.AuthorEmail
	}
	if len(hits) != 1 {
		return Resolution{}, false
	}
	for userID, email := range hits {
		return Resolution{
			Status:          BindingSuggested,
			SuggestedUserID: userID,
			Reason:          "账号与已绑定邮箱「" + email + "」的本地部分一致，建议确认为同一人",
		}, true
	}
	return Resolution{}, false
}

// SuggestUserForLogin 是 matchByBoundEmailLocalPart 的纯查询版本：给定
// 已持久化的绑定行与登录名，返回建议归属的用户（读路径实时补建议，
// 不必等下一轮采集重算）。
func SuggestUserForLogin(bindings []AuthorBinding, login string) (uint, bool) {
	trimmed := strings.ToLower(strings.TrimSpace(login))
	if trimmed == "" {
		return 0, false
	}
	hits := make(map[uint]string, 2)
	for _, binding := range bindings {
		if binding.UserID == 0 || binding.AuthorEmail == "" {
			continue
		}
		local, _, found := strings.Cut(strings.ToLower(binding.AuthorEmail), "@")
		if !found || local == "" || local != trimmed {
			continue
		}
		hits[binding.UserID] = binding.AuthorEmail
	}
	if len(hits) != 1 {
		return 0, false
	}
	for userID := range hits {
		return userID, true
	}
	return 0, false
}

// githubNoreplyLogin extracts the login embedded in GitHub's anonymous commit
// email `12345+login@users.noreply.github.com`; empty when the address is not
// that shape. The result only ever feeds the suggestion tier.
func githubNoreplyLogin(email string) string {
	if email == "" {
		return ""
	}
	lower := strings.ToLower(email)
	const suffix = "@users.noreply.github.com"
	if !strings.HasSuffix(lower, suffix) {
		return ""
	}
	local := strings.TrimSuffix(lower, suffix)
	_, login, found := strings.Cut(local, "+")
	if !found || login == "" || strings.Contains(login, "+") {
		return ""
	}
	return login
}

func matchedResolution(userID uint, via MatchVia) Resolution {
	return Resolution{
		Status:     BindingMatched,
		UserID:     userID,
		MatchedVia: via,
		Reason:     "命中已确认的作者映射",
	}
}
