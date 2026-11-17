package identity

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeLoginRepository struct {
	transactions map[string]LoginTransaction
	codes        map[string]LoginCode
	users        map[uint]User
	tenants      map[string]bool
	feishuUsers  map[string]uint
	external     map[string]uint
	google       map[string]uint
	nextUserID   uint
}

func newFakeLoginRepository() *fakeLoginRepository {
	return &fakeLoginRepository{
		transactions: map[string]LoginTransaction{}, codes: map[string]LoginCode{}, users: map[uint]User{},
		tenants: map[string]bool{}, feishuUsers: map[string]uint{}, external: map[string]uint{}, google: map[string]uint{}, nextUserID: 1,
	}
}

func (f *fakeLoginRepository) CreateLoginTransaction(_ context.Context, value *LoginTransaction) error {
	f.transactions[value.StateHash] = *value
	return nil
}
func (f *fakeLoginRepository) ConsumeLoginTransaction(_ context.Context, flow, hash string, now time.Time) (LoginTransaction, error) {
	value, ok := f.transactions[hash]
	if !ok || value.Flow != flow || value.ConsumedAt != nil || !value.ExpiresAt.After(now) {
		return LoginTransaction{}, errors.New("not found")
	}
	consumed := now
	value.ConsumedAt = &consumed
	f.transactions[hash] = value
	return value, nil
}
func (f *fakeLoginRepository) CreateLoginCode(_ context.Context, value *LoginCode) error {
	f.codes[value.CodeHash] = *value
	return nil
}
func (f *fakeLoginRepository) ConsumeLoginCode(_ context.Context, hash string, now time.Time) (LoginCode, error) {
	value, ok := f.codes[hash]
	if !ok || value.ConsumedAt != nil || !value.ExpiresAt.After(now) {
		return LoginCode{}, errors.New("not found")
	}
	consumed := now
	value.ConsumedAt = &consumed
	f.codes[hash] = value
	return value, nil
}
func (f *fakeLoginRepository) RegisterFeishuTenants(_ context.Context, tenantKeys []string) error {
	for _, tenantKey := range tenantKeys {
		if _, exists := f.tenants[tenantKey]; !exists {
			f.tenants[tenantKey] = true
		}
	}
	return nil
}
func (f *fakeLoginRepository) ProvisionFeishuUser(_ context.Context, provider FeishuIdentity) (User, error) {
	enabled, exists := f.tenants[provider.TenantKey]
	if !exists {
		if len(f.tenants) != 0 {
			return User{}, ErrLoginTenantNotRegistered
		}
		f.tenants[provider.TenantKey] = true
		enabled = true
	}
	if !enabled {
		return User{}, ErrLoginTenantDisabled
	}
	if id, exists := f.feishuUsers[feishuUserKey(provider.TenantKey, provider.DisplayName)]; exists {
		matched, err := f.userByID(id)
		if err != nil {
			return User{}, err
		}
		if !matched.Active {
			return User{}, ErrLoginUserDisabled
		}
		return matched, nil
	}
	id := f.nextUserID
	for {
		if _, exists := f.users[id]; !exists {
			break
		}
		id++
	}
	f.nextUserID = id + 1
	user := User{
		ID: id, ExternalIssuer: "feishu_tenant", ExternalSubject: FeishuExternalSubject(provider.TenantKey, provider.DisplayName),
		Username: FeishuUsername(provider.TenantKey, provider.DisplayName), DisplayName: provider.DisplayName, AvatarURL: provider.AvatarURL, Active: true, Role: "user",
	}
	f.users[id] = user
	f.feishuUsers[feishuUserKey(provider.TenantKey, provider.DisplayName)] = id
	return user, nil
}

func feishuUserKey(tenantKey, displayName string) string { return tenantKey + "\x00" + displayName }
func (f *fakeLoginRepository) FindActiveUserByExternalIdentity(_ context.Context, issuer, subject string) (User, error) {
	return f.userByID(f.external[issuer+"\x00"+subject])
}
func (f *fakeLoginRepository) ResolveOrBindGoogleUser(_ context.Context, provider GoogleIdentity) (User, error) {
	key := provider.Issuer + "\x00" + provider.Subject
	if id := f.google[key]; id != 0 {
		return f.userByID(id)
	}
	return User{}, ErrLoginUserNotFound
}
func (f *fakeLoginRepository) ResolveTelegramUser(_ context.Context, provider TelegramIdentity) (User, error) {
	id := f.google[FlowTelegram+"\x00"+provider.Issuer+"\x00"+provider.Subject]
	if id == 0 {
		return User{}, ErrLoginUserNotFound
	}
	return f.userByID(id)
}
func (f *fakeLoginRepository) FindActiveUserByID(_ context.Context, id uint) (User, error) {
	return f.userByID(id)
}
func (f *fakeLoginRepository) userByID(id uint) (User, error) {
	user, ok := f.users[id]
	if !ok {
		return User{}, errors.New("not found")
	}
	return user, nil
}

type repeatReader struct{ value byte }

func (r repeatReader) Read(p []byte) (int, error) {
	for index := range p {
		p[index] = r.value
	}
	return len(p), nil
}

func TestLoginServiceConsumesStateAndCodeOnce(t *testing.T) {
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	repo := newFakeLoginRepository()
	user := User{ID: 7, Username: "alice", DisplayName: "Alice", Active: true}
	repo.users[user.ID] = user
	service := NewLoginService(repo, LoginOptions{Now: func() time.Time { return now }, Random: repeatReader{value: 7}})
	state, err := service.Start(context.Background(), FlowFeishu, "https://evil.example/steal")
	if err != nil {
		t.Fatal(err)
	}
	returnTo, err := service.ConsumeState(context.Background(), FlowFeishu, state)
	if err != nil || returnTo != "/" {
		t.Fatalf("returnTo=%q err=%v", returnTo, err)
	}
	if _, err := service.ConsumeState(context.Background(), FlowFeishu, state); !errors.Is(err, ErrInvalidLoginState) {
		t.Fatalf("state replay err=%v", err)
	}
	code, err := service.IssueCode(context.Background(), FlowFeishu, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	resolved, flow, err := service.ExchangeCode(context.Background(), code)
	if err != nil || flow != FlowFeishu || resolved.ID != user.ID {
		t.Fatalf("resolved=%+v flow=%q err=%v", resolved, flow, err)
	}
	if _, _, err := service.ExchangeCode(context.Background(), code); !errors.Is(err, ErrInvalidLoginCode) {
		t.Fatalf("code replay err=%v", err)
	}
}

func TestLoginServiceDoesNotConsumeStateForAnotherFlow(t *testing.T) {
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	repo := newFakeLoginRepository()
	service := NewLoginService(repo, LoginOptions{Now: func() time.Time { return now }, Random: repeatReader{value: 9}})
	state, err := service.Start(context.Background(), FlowFeishu, "/projects")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConsumeState(context.Background(), FlowSSO, state); !errors.Is(err, ErrInvalidLoginState) {
		t.Fatalf("wrong-flow consume err=%v", err)
	}
	returnTo, err := service.ConsumeState(context.Background(), FlowFeishu, state)
	if err != nil || returnTo != "/projects" {
		t.Fatalf("state was consumed by wrong flow: returnTo=%q err=%v", returnTo, err)
	}
}

func TestNormalizeReturnToPreservesLocalQueryOnly(t *testing.T) {
	for input, expected := range map[string]string{
		"": "/", "/projects?tab=open": "/projects?tab=open", "//evil.example/x": "/",
		"https://evil.example/x": "/", "projects": "/", "/%2f%2fevil": "/",
	} {
		if got := NormalizeReturnTo(input); got != expected {
			t.Errorf("NormalizeReturnTo(%q)=%q want %q", input, got, expected)
		}
	}
}

func TestLoginServiceUsesExistingFeishuBindingAndSSOBinding(t *testing.T) {
	repo := newFakeLoginRepository()
	user := User{ID: 8, Username: "bob", DisplayName: "Bob", Active: true}
	repo.users[user.ID] = user
	repo.tenants["tenant_bob"] = true
	repo.feishuUsers[feishuUserKey("tenant_bob", "Bob")] = user.ID
	repo.external["one2all\x00one2all|8"] = user.ID
	service := NewLoginService(repo, LoginOptions{Random: strings.NewReader(strings.Repeat("x", 64))})
	if got, err := service.ResolveOrProvisionFeishuUser(context.Background(), FeishuIdentity{TenantKey: "tenant_bob", DisplayName: "Bob"}); err != nil || got.ID != 8 {
		t.Fatalf("Feishu resolve=%+v err=%v", got, err)
	}
	if got, err := service.ResolveSSOUser(context.Background(), "one2all", "one2all|8"); err != nil || got.ID != 8 {
		t.Fatalf("SSO resolve=%+v err=%v", got, err)
	}
	if _, err := service.ResolveSSOUser(context.Background(), "other", "one2all|8"); !errors.Is(err, ErrLoginUserNotFound) {
		t.Fatalf("issuer mismatch err=%v", err)
	}
}

func TestLoginServiceTreatsCompanyAndNameAsTheFeishuUserKey(t *testing.T) {
	repo := newFakeLoginRepository()
	service := NewLoginService(repo, LoginOptions{Random: strings.NewReader(strings.Repeat("x", 64))})
	if err := service.RegisterFeishuTenants(context.Background(), []string{"tenant_one", "tenant_two"}); err != nil {
		t.Fatal(err)
	}
	first, err := service.ResolveOrProvisionFeishuUser(context.Background(), FeishuIdentity{TenantKey: "tenant_one", DisplayName: "Alex"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.ResolveOrProvisionFeishuUser(context.Background(), FeishuIdentity{TenantKey: "tenant_two", DisplayName: "Alex"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatalf("same display name from different companies reused user id %d", first.ID)
	}
	reused, err := service.ResolveOrProvisionFeishuUser(context.Background(), FeishuIdentity{TenantKey: "tenant_one", DisplayName: "Alex"})
	if err != nil || reused.ID != first.ID {
		t.Fatalf("company/name user was not reused: %+v err=%v", reused, err)
	}
}

func TestLoginServiceProvisionsOnlyRegisteredFeishuTenant(t *testing.T) {
	repo := newFakeLoginRepository()
	service := NewLoginService(repo, LoginOptions{Random: strings.NewReader(strings.Repeat("x", 64))})
	if err := service.RegisterFeishuTenants(context.Background(), []string{"tenant_allowed", "tenant_allowed"}); err != nil {
		t.Fatal(err)
	}
	first, err := service.ResolveOrProvisionFeishuUser(context.Background(), FeishuIdentity{
		TenantKey: "tenant_allowed", DisplayName: "Alice", AvatarURL: "https://avatar.test/alice.png",
	})
	if err != nil || first.ID == 0 || first.Username != FeishuUsername("tenant_allowed", "Alice") || first.DisplayName != "Alice" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := service.ResolveOrProvisionFeishuUser(context.Background(), FeishuIdentity{
		TenantKey: "tenant_allowed", DisplayName: "Alice",
	})
	if err != nil || second.ID != first.ID || second.DisplayName != "Alice" {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	if _, err := service.ResolveOrProvisionFeishuUser(context.Background(), FeishuIdentity{TenantKey: "tenant_unknown", DisplayName: "Other"}); !errors.Is(err, ErrLoginTenantNotRegistered) {
		t.Fatalf("unregistered tenant err=%v", err)
	}
	repo.tenants["tenant_disabled"] = false
	if _, err := service.ResolveOrProvisionFeishuUser(context.Background(), FeishuIdentity{TenantKey: "tenant_disabled", DisplayName: "Other"}); !errors.Is(err, ErrLoginTenantDisabled) {
		t.Fatalf("disabled tenant err=%v", err)
	}
}
