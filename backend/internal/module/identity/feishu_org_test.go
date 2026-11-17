package identity

import (
	"context"
	"testing"
	"time"
)

type fakeOrgSource struct {
	departments []FeishuDepartment
	users       map[string][]FeishuDepartmentUser
}

func (fake fakeOrgSource) ListFeishuDepartments(context.Context) ([]FeishuDepartment, error) {
	return fake.departments, nil
}

func (fake fakeOrgSource) ListFeishuDepartmentUsers(
	_ context.Context,
	departmentID string,
) ([]FeishuDepartmentUser, error) {
	return fake.users[departmentID], nil
}

type fakeOrgStore struct {
	departments     []FeishuDepartment
	assignments     []FeishuUserDepartmentAssignment
	cleared         bool
	byOpenID        map[string]uint
	byName          map[string]uint
	syncedAtExists  bool
	replaceErr      error
	replaceReceived []FeishuDepartment
	kept            []uint
}

func (store *fakeOrgStore) ReplaceDepartments(_ context.Context, departments []FeishuDepartment) error {
	store.replaceReceived = departments
	if store.replaceErr != nil {
		return store.replaceErr
	}
	store.departments = departments
	return nil
}

func (store *fakeOrgStore) DepartmentSyncedAt(context.Context) (time.Time, bool, error) {
	return time.Time{}, store.syncedAtExists, nil
}

func (store *fakeOrgStore) ResolveUsersByOpenID(
	context.Context, []string,
) (map[string]uint, error) {
	return store.byOpenID, nil
}

func (store *fakeOrgStore) ResolveUniqueUsersByName(
	context.Context, []string,
) (map[string]uint, error) {
	return store.byName, nil
}

func (store *fakeOrgStore) SetUserDepartments(
	_ context.Context, assignments []FeishuUserDepartmentAssignment,
) error {
	store.assignments = assignments
	return nil
}

func (store *fakeOrgStore) ClearUserDepartments(_ context.Context, keepUserIDs []uint) error {
	store.kept = keepUserIDs
	store.cleared = true
	return nil
}

func (store *fakeOrgStore) FeishuOrgStatus(context.Context) (FeishuOrgSyncStatus, error) {
	return FeishuOrgSyncStatus{}, nil
}

func TestFeishuOrgSyncMapsByOpenIDAndUniqueName(t *testing.T) {
	source := fakeOrgSource{
		departments: []FeishuDepartment{
			{OpenID: "od-a", ParentID: "0", Name: "研发组", MemberCount: 2},
			{OpenID: "od-b", ParentID: "od-a", Name: "平台组"},
		},
		users: map[string][]FeishuDepartmentUser{
			"od-a": {
				{OpenID: "ou-1", Name: "张三", DepartmentID: "od-a", JobTitle: "后端工程师"},
				{OpenID: "ou-2", Name: "李四", DepartmentID: "od-b"},
			},
			"od-b": {
				{OpenID: "ou-2", Name: "李四", DepartmentID: "od-b"},
			},
		},
	}
	store := &fakeOrgStore{
		byOpenID: map[string]uint{"ou-1": 11},
		byName:   map[string]uint{"李四": 12},
	}
	service, err := NewFeishuOrgSyncService(source, store, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.departments) != 2 {
		t.Fatalf("departments must be replaced, got %d", len(store.departments))
	}
	if len(store.assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %+v", store.assignments)
	}
	byUser := map[uint]FeishuUserDepartmentAssignment{}
	for _, assignment := range store.assignments {
		byUser[assignment.UserID] = assignment
	}
	if byUser[11].OpenID != "ou-1" || byUser[11].DepartmentID != "od-a" {
		t.Fatalf("open_id match wrong: %+v", byUser[11])
	}
	if byUser[11].JobTitle != "后端工程师" {
		t.Fatalf("job title must pass through: %+v", byUser[11])
	}
	if byUser[12].JobTitle != "" {
		t.Fatalf("missing job title must stay empty: %+v", byUser[12])
	}
	// 李四 cross-listed in two departments maps exactly once; the first
	// department in Feishu order wins (membership comes from the listing).
	if byUser[12].DepartmentID != "od-a" {
		t.Fatalf("name match wrong: %+v", byUser[12])
	}
	if len(store.kept) != 2 {
		t.Fatalf("kept users = %v", store.kept)
	}
	if !store.cleared {
		t.Fatal("stale clear must run")
	}
}

func TestFeishuOrgSyncAmbiguousNameNeverMapped(t *testing.T) {
	source := fakeOrgSource{
		departments: []FeishuDepartment{{OpenID: "od-a", ParentID: "0", Name: "研发组"}},
		users: map[string][]FeishuDepartmentUser{
			"od-a": {{OpenID: "ou-1", Name: "同名", DepartmentID: "od-a"}},
		},
	}
	store := &fakeOrgStore{byName: map[string]uint{}}
	service, err := NewFeishuOrgSyncService(source, store, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.assignments) != 0 {
		t.Fatalf("ambiguous name must stay unmapped, got %+v", store.assignments)
	}
}

func TestFeishuOrgSyncDisabledFailsClosed(t *testing.T) {
	service, err := NewFeishuOrgSyncService(fakeOrgSource{}, &fakeOrgStore{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sync(context.Background()); err == nil {
		t.Fatal("disabled sync must fail closed")
	}
}

func TestFeishuOrgSyncPropagatesSourceFailure(t *testing.T) {
	store := &fakeOrgStore{replaceErr: context.DeadlineExceeded}
	service, err := NewFeishuOrgSyncService(
		fakeOrgSource{
			departments: []FeishuDepartment{{OpenID: "od-a", ParentID: "0", Name: "研发组"}},
			users:       map[string][]FeishuDepartmentUser{},
		}, store, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sync(context.Background()); err == nil {
		t.Fatal("source failure must propagate")
	}
}
