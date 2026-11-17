package toolruntime

import (
	"testing"
	"time"
)

func TestOAuthServerConfigurationUsesOneCanonicalIssuerForMCPAndCLI(t *testing.T) {
	configuration, err := NewOAuthServerConfiguration("http://localhost:8081/")
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Issuer != "http://localhost:8081" ||
		configuration.MCPResource != "http://localhost:8081/api/v1/mcp" ||
		configuration.CLIResource != "http://localhost:8081/api/v1/tool-runtime" ||
		configuration.AuthorizationEndpoint != "http://localhost:8081/workspace/mcp-cli/authorize" ||
		configuration.DeviceAuthorizationEndpoint != "http://localhost:8081/api/v1/tool-auth/device" ||
		configuration.DeviceVerificationEndpoint != "http://localhost:8081/workspace/mcp-cli/device" {
		t.Fatalf("configuration=%+v", configuration)
	}
	if configuration.Validate() != nil {
		t.Fatal("canonical configuration did not validate")
	}
}

func TestOAuthConfigurationAndRedirectsFailClosed(t *testing.T) {
	for _, value := range []string{
		"http://pm.example.test", "https://user:secret@pm.example.test", "https://pm.example.test?token=secret",
	} {
		if _, err := NewOAuthServerConfiguration(value); err == nil {
			t.Fatalf("unsafe public base %q was accepted", value)
		}
	}
	if !redirectURIMatches("http://127.0.0.1/callback", "http://127.0.0.1:43123/callback") {
		t.Fatal("registered loopback redirect did not accept an ephemeral port")
	}
	for _, value := range []string{
		"https://attacker.example/callback", "http://127.0.0.1:43123/other", "http://localhost:43123/callback?token=secret",
	} {
		if redirectURIMatches("http://127.0.0.1/callback", value) {
			t.Fatalf("unsafe redirect %q matched", value)
		}
	}
}

func TestOAuthScopeCanonicalizationRequiresToolAccess(t *testing.T) {
	for input, expected := range map[string]string{
		"pm:tools":                "pm:tools",
		"offline_access pm:tools": "pm:tools offline_access",
	} {
		actual, ok := normalizeOAuthScope(input)
		if !ok || actual != expected {
			t.Fatalf("scope %q => %q, %v", input, actual, ok)
		}
	}
	for _, input := range []string{"", "offline_access", "pm:tools unknown", "pm:tools pm:tools"} {
		if _, ok := normalizeOAuthScope(input); ok {
			t.Fatalf("invalid scope %q was accepted", input)
		}
	}
}

func TestMainstreamAgentDynamicClientAuthorizationFactsValidate(t *testing.T) {
	configuration, err := NewOAuthServerConfiguration("http://localhost:8081")
	if err != nil {
		t.Fatal(err)
	}
	clients := []struct {
		name       string
		registered string
		actual     string
	}{
		{name: "Codex", registered: "http://127.0.0.1/callback/random-path", actual: "http://127.0.0.1:65151/callback/random-path"},
		{name: "Claude Code", registered: "http://127.0.0.1/callback", actual: "http://127.0.0.1:65152/callback"},
		{name: "Goose", registered: "http://127.0.0.1/oauth_callback", actual: "http://127.0.0.1:65153/oauth_callback"},
		{name: "OpenCode", registered: "http://127.0.0.1/mcp/oauth/callback", actual: "http://127.0.0.1:65154/mcp/oauth/callback"},
	}
	for _, test := range clients {
		t.Run(test.name, func(t *testing.T) {
			client := OAuthClient{
				ID: "pm_mcp_dynamic", Name: test.name, Channel: DeliveryMCP,
				RedirectURIs: []string{test.registered}, CreatedAt: time.Now().UTC(),
			}
			if err := client.Validate(); err != nil {
				t.Fatal(err)
			}
			if !redirectURIMatches(client.RedirectURIs[0], test.actual) {
				t.Fatalf("%s loopback redirect did not match its DCR registration", test.name)
			}
			context := OAuthAuthorizationContext{
				State: "opaque-state", RedirectURI: test.actual,
				PKCEChallenge: "QQoT61gdsBT-ovLL9CTSTw4EJJvl652dUKEubD-9VPs",
				Resource:      configuration.MCPResource, Scope: "pm:tools offline_access",
			}
			if err := context.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
	if err := validateOAuthResource(configuration, DeliveryMCP, "http://localhost:8081/api/v1/mcp"); err != nil {
		t.Fatal(err)
	}
}
