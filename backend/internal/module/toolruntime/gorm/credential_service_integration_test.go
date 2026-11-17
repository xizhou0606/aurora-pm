package toolruntimegorm_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
)

type fixtureGrantRepository struct {
	grant toolruntime.Grant
}

func (repository *fixtureGrantRepository) Create(context.Context, *toolruntime.Grant) error {
	return nil
}
func (repository *fixtureGrantRepository) FindByID(_ context.Context, id uint) (toolruntime.Grant, error) {
	if id != repository.grant.ID {
		return toolruntime.Grant{}, toolruntime.ErrGrantInvalid
	}
	return repository.grant, nil
}
func (repository *fixtureGrantRepository) Revoke(_ context.Context, _ uint, at time.Time) (toolruntime.Grant, error) {
	repository.grant.RevokedAt = &at
	return repository.grant, nil
}

type fixtureAuthorizationRequestRepository struct {
	request toolruntime.AuthorizationRequest
}

func (*fixtureAuthorizationRequestRepository) Create(context.Context, *toolruntime.AuthorizationRequest) error {
	return nil
}
func (repository *fixtureAuthorizationRequestRepository) FindByID(_ context.Context, id string) (toolruntime.AuthorizationRequest, error) {
	if id != repository.request.ID {
		return toolruntime.AuthorizationRequest{}, toolruntime.ErrAuthorizationRequestNotFound
	}
	return repository.request, nil
}
func (*fixtureAuthorizationRequestRepository) Decide(context.Context, toolruntime.AuthorizationDecisionCommand) (toolruntime.AuthorizationRequest, error) {
	return toolruntime.AuthorizationRequest{}, toolruntime.ErrAuthorizationDecisionInvalid
}

type fixtureMembership struct {
	allowed bool
}

func (membership *fixtureMembership) VerifyWorkspaceMembership(context.Context, uint, uint) error {
	if !membership.allowed {
		return errors.New("not a member")
	}
	return nil
}

type credentialFixture struct {
	service    *toolruntime.CredentialService
	grants     *fixtureGrantRepository
	membership *fixtureMembership
	db         *gorm.DB
	requestID  string
	verifier   string
	challenge  string
}

func newCredentialFixture(t *testing.T) credentialFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(toolruntimegorm.ExternalCredentialMigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	grant, err := toolruntime.NewGrant(toolruntime.GrantRequest{
		PrincipalID: 7, WorkspaceID: 11, ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI,
		AllowedTools: []string{"task.get"}, AllowedScopes: []string{"task:read"},
		EffectiveFrom: now.Add(-time.Minute), ExpiresAt: now.Add(20 * time.Minute),
	}, now.Add(-2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	grant.ID = 13
	requestID := "authorization_request_1234567890"
	decidedAt := now.Add(-30 * time.Second)
	request := toolruntime.AuthorizationRequest{
		ID: requestID, PrincipalID: grant.PrincipalID, WorkspaceID: grant.WorkspaceID,
		WorkspaceName: "PM Dev", ClientID: grant.ClientID, ClientName: "PM CLI", Channel: grant.Channel,
		ConfigurationMode: toolruntime.WorkspaceConfigurationWindow, ConfigurationVersion: 1,
		ToolNames: grant.AllowedTools, GrantScopes: grant.AllowedScopes,
		EffectiveFrom: grant.EffectiveFrom, ExpiresAt: grant.ExpiresAt, RequestExpiresAt: now.Add(4 * time.Minute),
		Decision: toolruntime.AuthorizationAuthorize, DecidedAt: &decidedAt, GrantID: &grant.ID,
		CreatedAt: now.Add(-time.Minute), UpdatedAt: decidedAt,
	}
	grants := &fixtureGrantRepository{grant: grant}
	membership := &fixtureMembership{allowed: true}
	service, err := toolruntime.NewCredentialService(
		grants, &fixtureAuthorizationRequestRepository{request: request},
		toolruntimegorm.NewCredentialRepository(db), membership, toolruntime.DefaultCredentialPolicy(),
	)
	if err != nil {
		t.Fatal(err)
	}
	verifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	digest := sha256.Sum256([]byte(verifier))
	return credentialFixture{
		service: service, grants: grants, membership: membership, db: db, requestID: requestID,
		verifier: verifier, challenge: base64.RawURLEncoding.EncodeToString(digest[:]),
	}
}

func (fixture credentialFixture) issueAndExchange(t *testing.T) toolruntime.IssuedTokenPair {
	t.Helper()
	issued, err := fixture.service.IssueAuthorizationCode(context.Background(), toolruntime.IssueAuthorizationCodeInput{
		PrincipalID: 7, WorkspaceID: 11, RequestID: fixture.requestID,
		PKCEMethod: toolruntime.PKCEMethodS256, PKCEChallenge: fixture.challenge,
		RedirectURI: "http://127.0.0.1:4321/callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	pair, err := fixture.service.ExchangeAuthorizationCode(context.Background(), toolruntime.ExchangeAuthorizationCodeInput{
		Code: issued.Code, PKCEVerifier: fixture.verifier, RedirectURI: "http://127.0.0.1:4321/callback",
		ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pair.AccessExpiresAt.After(fixture.grants.grant.ExpiresAt) || pair.RefreshExpiresAt.After(fixture.grants.grant.ExpiresAt) {
		t.Fatalf("token expiry escaped grant: access=%s refresh=%s grant=%s", pair.AccessExpiresAt, pair.RefreshExpiresAt, fixture.grants.grant.ExpiresAt)
	}
	return pair
}

func TestCredentialServicePKCESingleUseAndBoundVerifier(t *testing.T) {
	fixture := newCredentialFixture(t)
	issued, err := fixture.service.IssueAuthorizationCode(context.Background(), toolruntime.IssueAuthorizationCodeInput{
		PrincipalID: 7, WorkspaceID: 11, RequestID: fixture.requestID,
		PKCEMethod: toolruntime.PKCEMethodS256, PKCEChallenge: fixture.challenge,
		RedirectURI: "http://127.0.0.1:4321/callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.ExchangeAuthorizationCode(context.Background(), toolruntime.ExchangeAuthorizationCodeInput{
		Code: issued.Code, PKCEVerifier: fixture.verifier + "wrong", RedirectURI: "http://127.0.0.1:4321/callback",
		ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI,
	}); !errors.Is(err, toolruntime.ErrAuthorizationCodeInvalid) {
		t.Fatalf("wrong verifier error=%v", err)
	}
	pair, err := fixture.service.ExchangeAuthorizationCode(context.Background(), toolruntime.ExchangeAuthorizationCodeInput{
		Code: issued.Code, PKCEVerifier: fixture.verifier, RedirectURI: "http://127.0.0.1:4321/callback",
		ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.ExchangeAuthorizationCode(context.Background(), toolruntime.ExchangeAuthorizationCodeInput{
		Code: issued.Code, PKCEVerifier: fixture.verifier, RedirectURI: "http://127.0.0.1:4321/callback",
		ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI,
	}); !errors.Is(err, toolruntime.ErrAuthorizationCodeInvalid) {
		t.Fatalf("code replay error=%v", err)
	}
	authorization, err := fixture.service.VerifyExternalToken(context.Background(), pair.AccessToken, toolruntime.DeliveryCLI, toolruntime.AudienceCLI)
	if err != nil || authorization.GrantID != 13 || authorization.WorkspaceID != 11 {
		t.Fatalf("authorization=%+v err=%v", authorization, err)
	}
	if _, err := fixture.service.VerifyExternalToken(context.Background(), pair.AccessToken, toolruntime.DeliveryMCP, toolruntime.AudienceMCP); !errors.Is(err, toolruntime.ErrExternalTokenInvalid) {
		t.Fatalf("cross-audience error=%v", err)
	}
	fixture.membership.allowed = false
	if _, err := fixture.service.VerifyExternalToken(context.Background(), pair.AccessToken, toolruntime.DeliveryCLI, toolruntime.AudienceCLI); !errors.Is(err, toolruntime.ErrExternalWorkspaceMembershipInactive) {
		t.Fatalf("membership loss error=%v", err)
	} else if workspaceID, ok := toolruntime.ExternalWorkspaceID(err); !ok || workspaceID != 11 {
		t.Fatalf("membership workspace=%d found=%t", workspaceID, ok)
	}
	fixture.membership.allowed = true
	revokedAt := time.Now().UTC()
	fixture.grants.grant.RevokedAt = &revokedAt
	if _, err := fixture.service.VerifyExternalToken(context.Background(), pair.AccessToken, toolruntime.DeliveryCLI, toolruntime.AudienceCLI); !errors.Is(err, toolruntime.ErrExternalTokenInvalid) {
		t.Fatalf("grant revocation error=%v", err)
	}
}

func TestCredentialServiceRefreshRotationReplayRevokesFamily(t *testing.T) {
	fixture := newCredentialFixture(t)
	first := fixture.issueAndExchange(t)
	second, err := fixture.service.RefreshAccessToken(context.Background(), toolruntime.RefreshAccessTokenInput{
		RefreshToken: first.RefreshToken, ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI,
	})
	if err != nil || second.RefreshToken == first.RefreshToken {
		t.Fatalf("refresh failed: pair=%+v err=%v", second, err)
	}
	if _, err := fixture.service.RefreshAccessToken(context.Background(), toolruntime.RefreshAccessTokenInput{
		RefreshToken: first.RefreshToken, ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI,
	}); !errors.Is(err, toolruntime.ErrRefreshTokenReplay) {
		t.Fatalf("replay error=%v", err)
	}
	if _, err := fixture.service.VerifyExternalToken(context.Background(), second.AccessToken, toolruntime.DeliveryCLI, toolruntime.AudienceCLI); !errors.Is(err, toolruntime.ErrExternalTokenInvalid) {
		t.Fatalf("family access remained valid after replay: %v", err)
	}
}

func TestCredentialRecordsPersistDigestsOnlyAndOneCodePerRequest(t *testing.T) {
	fixture := newCredentialFixture(t)
	issued, err := fixture.service.IssueAuthorizationCode(context.Background(), toolruntime.IssueAuthorizationCodeInput{
		PrincipalID: 7, WorkspaceID: 11, RequestID: fixture.requestID,
		PKCEMethod: toolruntime.PKCEMethodS256, PKCEChallenge: fixture.challenge,
		RedirectURI: "http://127.0.0.1:4321/callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.IssueAuthorizationCode(context.Background(), toolruntime.IssueAuthorizationCodeInput{
		PrincipalID: 7, WorkspaceID: 11, RequestID: fixture.requestID,
		PKCEMethod: toolruntime.PKCEMethodS256, PKCEChallenge: fixture.challenge,
		RedirectURI: "http://127.0.0.1:4321/callback",
	}); !errors.Is(err, toolruntime.ErrAuthorizationCodeInvalid) {
		t.Fatalf("duplicate issue error=%v", err)
	}
	var row toolruntimegorm.ExternalAuthorizationCode
	if err := fixture.db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.CodeDigest == issued.Code || len(row.CodeDigest) != 64 {
		t.Fatalf("raw authorization code persisted: digest=%q", row.CodeDigest)
	}
	pair, err := fixture.service.ExchangeAuthorizationCode(context.Background(), toolruntime.ExchangeAuthorizationCodeInput{
		Code: issued.Code, PKCEVerifier: fixture.verifier, RedirectURI: "http://127.0.0.1:4321/callback",
		ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI,
	})
	if err != nil {
		t.Fatal(err)
	}
	var accessRow toolruntimegorm.ExternalAccessCredential
	var refreshRow toolruntimegorm.ExternalRefreshCredential
	if err := fixture.db.First(&accessRow).Error; err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.First(&refreshRow).Error; err != nil {
		t.Fatal(err)
	}
	if accessRow.TokenDigest == pair.AccessToken || refreshRow.TokenDigest == pair.RefreshToken ||
		len(accessRow.TokenDigest) != 64 || len(refreshRow.TokenDigest) != 64 {
		t.Fatalf("raw token persisted: access=%q refresh=%q", accessRow.TokenDigest, refreshRow.TokenDigest)
	}
	for _, record := range toolruntimegorm.ExternalCredentialMigrationRecords() {
		columns, err := fixture.db.Migrator().ColumnTypes(record)
		if err != nil {
			t.Fatal(err)
		}
		for _, column := range columns {
			name := column.Name()
			if name == "code" || name == "access_token" || name == "refresh_token" || name == "code_verifier" {
				t.Fatalf("raw secret column %q exists", name)
			}
		}
	}
}
