package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

func TestExternalToolAuthorizationResolverUsesFixedAdapterFacts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var receivedToken string
	verifier := toolruntime.ExternalTokenVerifierFunc(func(
		_ context.Context,
		rawToken string,
		channel toolruntime.DeliveryChannel,
		audience toolruntime.TokenAudience,
	) (toolruntime.ExecutionAuthorization, error) {
		receivedToken = rawToken
		if channel != toolruntime.DeliveryCLI || audience != toolruntime.AudienceCLI {
			t.Fatalf("channel=%s audience=%s", channel, audience)
		}
		return toolruntime.ExecutionAuthorization{
			PrincipalID: 7, WorkspaceID: 11, GrantID: 13, ClientID: "codex",
			Channel: channel, Audience: audience,
		}, nil
	})
	resolver, err := NewExternalToolAuthorizationResolver(verifier, toolruntime.DeliveryCLI, toolruntime.AudienceCLI)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer opaque-secret")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = request
	authorization, err := resolver.ResolveAuthorization(c)
	if err != nil || receivedToken != "opaque-secret" || authorization.GrantID != 13 {
		t.Fatalf("authorization=%+v token=%q err=%v", authorization, receivedToken, err)
	}
}

func TestExternalToolAuthorizationResolverFailsClosedWithoutLeakingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "must-not-escape"
	verifier := toolruntime.ExternalTokenVerifierFunc(func(
		context.Context,
		string,
		toolruntime.DeliveryChannel,
		toolruntime.TokenAudience,
	) (toolruntime.ExecutionAuthorization, error) {
		return toolruntime.ExecutionAuthorization{}, errors.New("private verifier error containing " + secret)
	})
	resolver, err := NewExternalToolAuthorizationResolver(verifier, toolruntime.DeliveryMCP, toolruntime.AudienceMCP)
	if err != nil {
		t.Fatal(err)
	}

	for name, header := range map[string]string{
		"missing":      "",
		"wrong scheme": "Basic value",
		"extra fields": "Bearer one two",
		"verifier":     "Bearer " + secret,
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("Authorization", header)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = request
			_, err := resolver.ResolveAuthorization(c)
			if !errors.Is(err, toolruntime.ErrExternalTokenInvalid) || strings.Contains(err.Error(), secret) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestExternalToolAuthorizationResolverRejectsVerifierChannelDrift(t *testing.T) {
	gin.SetMode(gin.TestMode)
	verifier := toolruntime.ExternalTokenVerifierFunc(func(
		context.Context,
		string,
		toolruntime.DeliveryChannel,
		toolruntime.TokenAudience,
	) (toolruntime.ExecutionAuthorization, error) {
		return toolruntime.ExecutionAuthorization{
			PrincipalID: 7, WorkspaceID: 11, GrantID: 13, ClientID: "codex",
			Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
		}, nil
	})
	resolver, err := NewExternalToolAuthorizationResolver(verifier, toolruntime.DeliveryMCP, toolruntime.AudienceMCP)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer opaque")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = request
	if _, err := resolver.ResolveAuthorization(c); !errors.Is(err, toolruntime.ErrExternalTokenInvalid) {
		t.Fatalf("error=%v", err)
	}
}

func TestExternalToolAuthorizationResolverPreservesWorkspaceMembershipFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	verifier := toolruntime.ExternalTokenVerifierFunc(func(
		context.Context,
		string,
		toolruntime.DeliveryChannel,
		toolruntime.TokenAudience,
	) (toolruntime.ExecutionAuthorization, error) {
		return toolruntime.ExecutionAuthorization{}, toolruntime.ExternalWorkspaceMembershipError{WorkspaceID: 11}
	})
	resolver, err := NewExternalToolAuthorizationResolver(verifier, toolruntime.DeliveryCLI, toolruntime.AudienceCLI)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer opaque")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = request
	_, err = resolver.ResolveAuthorization(c)
	workspaceID, found := toolruntime.ExternalWorkspaceID(err)
	if !errors.Is(err, toolruntime.ErrExternalWorkspaceMembershipInactive) || !found || workspaceID != 11 {
		t.Fatalf("workspace=%d found=%t error=%v", workspaceID, found, err)
	}
}
