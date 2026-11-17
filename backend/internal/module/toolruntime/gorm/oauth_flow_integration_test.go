package toolruntimegorm

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

func TestOAuthAuthorizationPersistsCredentialAcrossRequestsWithinUATWindow(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	records := []any{
		&ExternalAuthorizationGrant{}, &ExternalAuthorizationRequest{}, &ExternalToolingWorkspaceConfiguration{}, &ExternalDeviceAuthorization{},
	}
	records = append(records, OAuthMigrationRecords()...)
	records = append(records, ExternalCredentialMigrationRecords()...)
	if err := db.AutoMigrate(records...); err != nil {
		t.Fatal(err)
	}
	catalog, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: []string{"project.set.get"}})
	if err != nil {
		t.Fatal(err)
	}
	requests := NewAuthorizationRequestRepository(db)
	grants := NewGrantRepository(db)
	configurations := NewWorkspaceConfigurationRepository(db)
	authorizations, err := toolruntime.NewAuthorizationService(catalog, requests, grants, configurations)
	if err != nil {
		t.Fatal(err)
	}
	principal := toolruntime.ManagementPrincipal{PrincipalID: 7, WorkspaceID: 11, WorkspaceName: "PM UAT"}
	now := time.Now().UTC()
	if _, err := authorizations.SaveWorkspaceConfiguration(context.Background(), principal, toolruntime.WorkspaceConfigurationInput{
		Mode: toolruntime.WorkspaceConfigurationWindow, ToolNames: []string{"project.set.get"},
		EffectiveFrom: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	credentials, err := toolruntime.NewCredentialService(
		grants, requests, NewCredentialRepository(db),
		toolruntime.WorkspaceMembershipVerifierFunc(func(context.Context, uint, uint) error { return nil }),
		toolruntime.DefaultCredentialPolicy(),
	)
	if err != nil {
		t.Fatal(err)
	}
	server, err := toolruntime.NewOAuthServerConfiguration("http://localhost:8081")
	if err != nil {
		t.Fatal(err)
	}
	oauth, err := toolruntime.NewOAuthService(authorizations, credentials, NewOAuthClientRepository(db), server)
	if err != nil {
		t.Fatal(err)
	}
	client, err := oauth.RegisterClient(context.Background(), toolruntime.OAuthClientRegistrationInput{
		Name: "Codex", RedirectURIs: []string{"http://127.0.0.1:43123/callback"},
	})
	if err != nil {
		t.Fatal(err)
	}
	verifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	request, err := oauth.StartAuthorization(context.Background(), principal, toolruntime.OAuthAuthorizationStartInput{
		ResponseType: "code", ClientID: client.ID, RedirectURI: client.RedirectURIs[0],
		PKCEChallenge: challenge, PKCEChallengeMethod: toolruntime.PKCEMethodS256,
		State: "opaque-state", Resource: server.MCPResource,
		Scope: toolruntime.OAuthScopeTools + " " + toolruntime.OAuthScopeOfflineAccess,
	})
	if err != nil {
		t.Fatal(err)
	}
	completion, err := oauth.CompleteAuthorization(context.Background(), principal, request.ID, toolruntime.AuthorizationAuthorize)
	if err != nil {
		t.Fatal(err)
	}
	callback, err := url.Parse(completion.RedirectURL)
	if err != nil || callback.Query().Get("state") != "opaque-state" || callback.Query().Get("iss") != server.Issuer {
		t.Fatalf("callback=%q err=%v", completion.RedirectURL, err)
	}
	pair, err := credentials.ExchangeAuthorizationCode(context.Background(), toolruntime.ExchangeAuthorizationCodeInput{
		Code: callback.Query().Get("code"), PKCEVerifier: verifier, RedirectURI: client.RedirectURIs[0],
		ClientID: client.ID, Channel: toolruntime.DeliveryMCP,
	})
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		authorization, err := credentials.VerifyExternalToken(
			context.Background(), pair.AccessToken, toolruntime.DeliveryMCP, toolruntime.AudienceMCP,
		)
		if err != nil || authorization.PrincipalID != principal.PrincipalID || authorization.WorkspaceID != principal.WorkspaceID {
			t.Fatalf("request %d authorization=%+v err=%v", index+1, authorization, err)
		}
	}

	history := NewAuthorizationHistoryRepository(db)
	scope := toolruntime.AuthorizationHistoryScope{WorkspaceID: principal.WorkspaceID, PrincipalID: principal.PrincipalID}
	if err := history.RevokeHistory(context.Background(), scope, "r_"+request.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := credentials.VerifyExternalToken(context.Background(), pair.AccessToken, toolruntime.DeliveryMCP, toolruntime.AudienceMCP); err == nil {
		t.Fatal("revoked authorization access token remained valid")
	}
	if _, err := credentials.RefreshAccessToken(context.Background(), toolruntime.RefreshAccessTokenInput{RefreshToken: pair.RefreshToken, ClientID: client.ID, Channel: toolruntime.DeliveryMCP}); err == nil {
		t.Fatal("revoked authorization refresh token remained valid")
	}
	var accessRows int64
	if err := db.Model(&ExternalAccessCredential{}).Count(&accessRows).Error; err != nil || accessRows != 1 {
		t.Fatalf("access credential rows=%d err=%v", accessRows, err)
	}
}
