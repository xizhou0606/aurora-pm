package toolruntimegorm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"gorm.io/gorm"
)

func historyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := authorizationTestDB(t)
	records := append(ExternalCredentialMigrationRecords(), &ExternalDeviceAuthorization{})
	if err := db.AutoMigrate(records...); err != nil {
		t.Fatal(err)
	}
	return db
}
func seedHistory(t *testing.T, db *gorm.DB, id string, workspace, user uint, status string, now time.Time) *ExternalAuthorizationGrant {
	t.Helper()
	req := ExternalAuthorizationRequest{ID: id, PrincipalID: user, WorkspaceID: workspace, WorkspaceName: "test", ClientID: "pm-cli", ClientName: "PM CLI", Channel: "cli", ConfigurationMode: "window", ToolNames: `["project.list"]`, GrantScopes: `["project:read"]`, EffectiveFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), RequestExpiresAt: now.Add(time.Minute), Decision: status, CreatedAt: now}
	var grant *ExternalAuthorizationGrant
	if status == "authorize" {
		grant = &ExternalAuthorizationGrant{PrincipalID: user, WorkspaceID: workspace, ClientID: "pm-cli", Channel: "cli", AllowedTools: req.ToolNames, AllowedScopes: req.GrantScopes, EffectiveFrom: req.EffectiveFrom, ExpiresAt: req.ExpiresAt, CreatedAt: now}
		if err := db.Create(grant).Error; err != nil {
			t.Fatal(err)
		}
		req.GrantID = &grant.ID
		req.DecidedAt = &now
	}
	if err := db.Create(&req).Error; err != nil {
		t.Fatal(err)
	}
	return grant
}
func TestAuthorizationHistoryPaginationScopeAndSafeProjection(t *testing.T) {
	db := historyTestDB(t)
	now := time.Now().UTC()
	ctx := context.Background()
	repo := NewAuthorizationHistoryRepository(db)
	seedHistory(t, db, "own", 2, 7, "authorize", now)
	seedHistory(t, db, "other", 2, 8, "authorize", now.Add(-time.Second))
	seedHistory(t, db, "foreign", 3, 7, "authorize", now)
	seedHistory(t, db, "pending", 2, 7, "pending", now.Add(-time.Minute))
	seedHistory(t, db, "denied", 2, 7, "deny", now.Add(-time.Minute))
	code, id := "ABCD-EFGH", "d_test"
	if err := db.Create(&ExternalDeviceAuthorization{DeviceCodeDigest: strings.Repeat("a", 64), UserCodeDigest: strings.Repeat("b", 64), RecordID: &id, UserCode: &code, WorkspaceID: 2, ClientID: "pm-cli", Channel: "cli", Audience: "cli", Resource: "cli", Scope: "pm:tools", Status: "pending", ExpiresAt: now.Add(time.Minute), PollIntervalSeconds: 5, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	self := toolruntime.AuthorizationHistoryScope{WorkspaceID: 2, PrincipalID: 7}
	result, err := repo.ListHistory(ctx, self, 1, 10, now)
	if err != nil || result.Total != 3 {
		t.Fatalf("self %+v %v", result, err)
	}
	for _, item := range result.Items {
		if item.Status != "approved" && item.PrincipalID != nil {
			t.Fatalf("unapproved principal leaked %+v", item)
		}
	}
	all := self
	all.All = true
	result, err = repo.ListHistory(ctx, all, 1, 2, now)
	if err != nil || result.Total != 5 || len(result.Items) != 2 {
		t.Fatalf("admin %+v %v", result, err)
	}
	next, err := repo.ListHistory(ctx, all, 2, 2, now)
	if err != nil || len(next.Items) != 2 || next.Items[0].ID == result.Items[0].ID {
		t.Fatal("pagination unstable")
	}
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), strings.Repeat("a", 64)) || strings.Contains(string(raw), strings.Repeat("b", 64)) {
		t.Fatal("credential digest leaked")
	}
	if err := repo.RevokeHistory(ctx, self, "r_other", now); !errors.Is(err, toolruntime.ErrAuthorizationRecordNotFound) {
		t.Fatalf("other user revoked: %v", err)
	}
	if err := repo.RevokeHistory(ctx, all, "r_foreign", now); !errors.Is(err, toolruntime.ErrAuthorizationRecordNotFound) {
		t.Fatalf("foreign workspace revoked: %v", err)
	}
}
func TestAuthorizationHistoryRevokeOnlyMatchingGrantAndFamily(t *testing.T) {
	db := historyTestDB(t)
	now := time.Now().UTC()
	ctx := context.Background()
	repo := NewAuthorizationHistoryRepository(db)
	target := seedHistory(t, db, "one", 2, 7, "authorize", now)
	other := seedHistory(t, db, "two", 2, 7, "authorize", now)
	for i, g := range []*ExternalAuthorizationGrant{target, other} {
		family := ExternalCredentialFamily{ID: strings.Repeat(string(rune('a'+i)), 43), Binding: credentialBindingRecord{PrincipalID: 7, WorkspaceID: 2, GrantID: g.ID, ClientID: "pm-cli", Channel: "cli", Audience: "cli"}, AbsoluteExpiresAt: now.Add(time.Hour), CreatedAt: now}
		if err := db.Create(&family).Error; err != nil {
			t.Fatal(err)
		}
		refresh := ExternalRefreshCredential{TokenDigest: strings.Repeat(string(rune('a'+i)), 64), FamilyID: family.ID, Binding: family.Binding, ExpiresAt: now.Add(time.Hour), CreatedAt: now}
		if err := db.Create(&refresh).Error; err != nil {
			t.Fatal(err)
		}
	}
	scope := toolruntime.AuthorizationHistoryScope{WorkspaceID: 2, PrincipalID: 7}
	if err := repo.RevokeHistory(ctx, scope, "r_one", now); err != nil {
		t.Fatal(err)
	}
	if err := repo.RevokeHistory(ctx, scope, "r_one", now); err != nil {
		t.Fatalf("idempotence: %v", err)
	}
	var grants []ExternalAuthorizationGrant
	db.Order("id").Find(&grants)
	if grants[0].RevokedAt == nil || grants[1].RevokedAt != nil {
		t.Fatal("grant scope incorrect")
	}
	var families []ExternalCredentialFamily
	db.Order("id").Find(&families)
	if families[0].RevokedAt == nil || families[1].RevokedAt != nil {
		t.Fatal("family scope incorrect")
	}
	var refresh []ExternalRefreshCredential
	db.Order("token_digest").Find(&refresh)
	if refresh[0].RevokedAt == nil || refresh[1].RevokedAt != nil {
		t.Fatal("refresh scope incorrect")
	}
}

func TestAuthorizationHistoryBatchQueryCount(t *testing.T) {
	db := historyTestDB(t)
	now := time.Now().UTC()
	for i := 0; i < 20; i++ {
		seedHistory(t, db, strings.Repeat("a", i+1), 2, 7, "authorize", now)
	}
	count := 0
	name := "history_count_" + t.Name()
	if err := db.Callback().Query().After("gorm:query").Register(name, func(*gorm.DB) { count++ }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Callback().Query().Remove(name) })
	result, err := NewAuthorizationHistoryRepository(db).ListHistory(context.Background(), toolruntime.AuthorizationHistoryScope{WorkspaceID: 2, PrincipalID: 7}, 1, 100, now)
	if err != nil || len(result.Items) != 20 {
		t.Fatalf("list %d %v", len(result.Items), err)
	}
	// Requests, codes, grants, families: constant query count, not per row.
	if count != 4 {
		t.Fatalf("page issued %d fact queries; want 4", count)
	}
}
func TestAuthorizationHistoryCodeValidityIsIndependentFromGrant(t *testing.T) {
	now := time.Now().UTC()
	p := uint(7)
	grant := &ExternalAuthorizationGrant{AllowedTools: `[]`, ExpiresAt: now.Add(time.Hour)}
	request := &ExternalAuthorizationRequest{Decision: "pending", PrincipalID: 7, ToolNames: `[]`, RequestExpiresAt: now.Add(time.Minute)}
	item, err := projectHistory("r_test", historyFacts{request: request}, now)
	if err != nil || item.CodeValid || item.PrincipalID != nil {
		t.Fatalf("pending OAuth %+v %v", item, err)
	}
	request.Decision = "authorize"
	item, err = projectHistory("r_test", historyFacts{request: request, grant: grant, codes: []ExternalAuthorizationCode{{ExpiresAt: now.Add(time.Minute)}}}, now)
	if err != nil || !item.CodeValid {
		t.Fatal("issued code not valid")
	}
	for _, status := range []string{"pending", "approved", "consumed"} {
		d := &ExternalDeviceAuthorization{Status: status, PrincipalID: &p, ExpiresAt: now.Add(time.Minute)}
		item, err = projectHistory("d_test", historyFacts{device: d, grant: grant}, now)
		if err != nil || item.CodeValid != (status != "consumed") {
			t.Fatalf("device %s %+v %v", status, item, err)
		}
	}
}

func TestLateExchangeAfterRevokeProducesUnusableCredentials(t *testing.T) {
	db := historyTestDB(t)
	now := time.Now().UTC()
	ctx := context.Background()
	requestID := strings.Repeat("q", 32)
	grant := seedHistory(t, db, requestID, 2, 7, "authorize", now)
	binding := toolruntime.CredentialBinding{PrincipalID: 7, WorkspaceID: 2, GrantID: grant.ID, ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI}
	repo := NewCredentialRepository(db)
	digest := func(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
	accessSecret, refreshSecret := strings.Repeat("a", 43), strings.Repeat("b", 43)
	codeDigest := digest("code")
	if err := repo.CreateAuthorizationCode(ctx, toolruntime.AuthorizationCodeCredential{CodeDigest: codeDigest, AuthorizationRequestID: requestID, Binding: binding, PKCEChallenge: strings.Repeat("c", 43), RedirectURI: "http://127.0.0.1/callback", CreatedAt: now, ExpiresAt: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	// The exchange has already read its valid grant. Revoke commits before the
	// exchange repository writes its new family: deterministic race interleave.
	if err := NewAuthorizationHistoryRepository(db).RevokeHistory(ctx, toolruntime.AuthorizationHistoryScope{WorkspaceID: 2, PrincipalID: 7}, "r_"+requestID, now); err != nil {
		t.Fatal(err)
	}
	family := toolruntime.CredentialFamily{ID: strings.Repeat("f", 43), Binding: binding, AbsoluteExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now}
	access := toolruntime.AccessCredential{TokenDigest: digest(accessSecret), FamilyID: family.ID, Binding: binding, ExpiresAt: now.Add(time.Minute), CreatedAt: now}
	refresh := toolruntime.RefreshCredential{TokenDigest: digest(refreshSecret), FamilyID: family.ID, Binding: binding, ExpiresAt: now.Add(time.Hour), CreatedAt: now}
	if err := repo.ExchangeAuthorizationCode(ctx, codeDigest, now, family, access, refresh); err != nil {
		t.Fatal(err)
	}
	service, err := toolruntime.NewCredentialService(NewGrantRepository(db), NewAuthorizationRequestRepository(db), repo, toolruntime.WorkspaceMembershipVerifierFunc(func(context.Context, uint, uint) error { return nil }), toolruntime.DefaultCredentialPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.VerifyExternalToken(ctx, accessSecret, toolruntime.DeliveryCLI, toolruntime.AudienceCLI); err == nil {
		t.Fatal("late access accepted")
	}
	if _, err := service.RefreshAccessToken(ctx, toolruntime.RefreshAccessTokenInput{RefreshToken: refreshSecret, ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI}); err == nil {
		t.Fatal("late refresh accepted")
	}
}

func TestAuthorizationHistoryPermanentExpiryJSONUsesUTC(t *testing.T) {
	now := time.Now().UTC()
	// PostgreSQL may return timestamptz in the local location. The permanent
	// sentinel is valid in UTC, but its +08 representation is in year 10000.
	local := time.FixedZone("UTC+8", 8*60*60)
	expiry := time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC).In(local)
	request := &ExternalAuthorizationRequest{Decision: "authorize", PrincipalID: 7, ToolNames: `[]`, CreatedAt: now.In(local), DecidedAt: &now, RequestExpiresAt: now.Add(time.Minute)}
	item, err := projectHistory("r_test", historyFacts{request: request, grant: &ExternalAuthorizationGrant{AllowedTools: `[]`, ExpiresAt: expiry}}, now)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("permanent grant broke JSON response: %v", err)
	}
	if !strings.Contains(string(raw), `"expires_at":"9999-12-31T23:59:59Z"`) {
		t.Fatalf("expiry not normalized: %s", raw)
	}
	if item.ExpiresAt.Location() != time.UTC || item.CreatedAt.Location() != time.UTC || item.DecidedAt.Location() != time.UTC {
		t.Fatal("history timestamps must use UTC")
	}
}

func TestAuthorizationHistoryCredentialValidity(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name, status                          string
		family                                bool
		familyRevoked, grantRevoked           bool
		familyExpiry, grantExpiry, grantStart time.Time
		want                                  bool
	}{
		{name: "pending", status: "pending", grantExpiry: now.Add(time.Hour)},
		{name: "approved not exchanged", status: "authorize", grantExpiry: now.Add(time.Hour)},
		{name: "active", status: "authorize", family: true, familyExpiry: now.Add(time.Hour), grantExpiry: now.Add(time.Hour), want: true},
		{name: "family expired", status: "authorize", family: true, familyExpiry: now, grantExpiry: now.Add(time.Hour)},
		{name: "grant expired", status: "authorize", family: true, familyExpiry: now.Add(time.Hour), grantExpiry: now},
		{name: "family revoked", status: "authorize", family: true, familyRevoked: true, familyExpiry: now.Add(time.Hour), grantExpiry: now.Add(time.Hour)},
		{name: "grant revoked", status: "authorize", family: true, grantRevoked: true, familyExpiry: now.Add(time.Hour), grantExpiry: now.Add(time.Hour)},
		{name: "scheduled grant", status: "authorize", family: true, familyExpiry: now.Add(time.Hour), grantExpiry: now.Add(time.Hour), grantStart: now.Add(time.Minute)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := historyFacts{request: &ExternalAuthorizationRequest{Decision: tc.status, PrincipalID: 7, ToolNames: `[]`, CreatedAt: now, RequestExpiresAt: now.Add(time.Minute)}}
			if tc.status == "authorize" {
				facts.grant = &ExternalAuthorizationGrant{AllowedTools: `[]`, ExpiresAt: tc.grantExpiry, EffectiveFrom: tc.grantStart}
				if tc.grantRevoked {
					facts.grant.RevokedAt = &now
				}
			}
			if tc.family {
				family := ExternalCredentialFamily{CreatedAt: now, AbsoluteExpiresAt: tc.familyExpiry}
				if tc.familyRevoked {
					family.RevokedAt = &now
				}
				facts.families = []ExternalCredentialFamily{family}
			}
			item, err := projectHistory("r_test", facts, now)
			if err != nil || item.CredentialValid != tc.want {
				t.Fatalf("valid=%v want=%v err=%v", item.CredentialValid, tc.want, err)
			}
		})
	}
}
