package httpapi

import (
	"reflect"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestLoginMethodsAdvertiseOnlyEnabledOptionalProviders(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want []string
	}{
		{name: "local", cfg: config.Config{AuthMode: "local"}, want: []string{"credentials", "feishu"}},
		{name: "shared", cfg: config.Config{AuthMode: "shared_jwt"}, want: []string{"feishu"}},
		{name: "hybrid external providers", cfg: config.Config{
			AuthMode: "hybrid", GoogleLogin: config.GoogleLogin{Enabled: true},
			TelegramLogin: config.TelegramLogin{Enabled: true}, SSO: config.SSO{Enabled: true},
		}, want: []string{"credentials", "feishu", "google", "telegram", "sso"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := loginMethods(test.cfg); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("loginMethods()=%v, want %v", got, test.want)
			}
		})
	}
}
