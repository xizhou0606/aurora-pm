package identity

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// The Feishu contact org sync keeps a local display projection of the
// company's Feishu department tree (feishu_departments) and maps workspace
// members onto it (users.feishu_department_id). It is presentation data for
// the reporting directory — authorization never reads it.
//
// Mapping order per Feishu user: an existing users.feishu_open_id match wins;
// otherwise a UNIQUE active display_name match may adopt the open_id. Users
// that vanish from the contact data lose their department on the next sync.

// FeishuDepartment is the synced department node.
type FeishuDepartment struct {
	OpenID      string
	ParentID    string
	Name        string
	Order       int64
	MemberCount int
}

// FeishuDepartmentUser is one contact user read from a department, carrying
// its primary department id ("0"/empty at tenant root) and job title.
type FeishuDepartmentUser struct {
	OpenID       string
	Name         string
	DepartmentID string
	JobTitle     string
}

// FeishuOrgSource reads the org tree from the Feishu contact API.
type FeishuOrgSource interface {
	ListFeishuDepartments(ctx context.Context) ([]FeishuDepartment, error)
	ListFeishuDepartmentUsers(ctx context.Context, departmentID string) ([]FeishuDepartmentUser, error)
}

// FeishuUserDepartmentAssignment maps one PM user to a Feishu department and
// adopts the Feishu open_id when the user has none yet.
type FeishuUserDepartmentAssignment struct {
	UserID       uint
	OpenID       string
	DepartmentID string
	JobTitle     string
}

// FeishuOrgStore persists the synced org projection.
type FeishuOrgStore interface {
	// ReplaceDepartments swaps the whole department table atomically.
	ReplaceDepartments(ctx context.Context, departments []FeishuDepartment) error
	// DepartmentSyncedAt reports the latest successful sync time.
	DepartmentSyncedAt(ctx context.Context) (time.Time, bool, error)
	// ResolveUsersByOpenID maps Feishu open ids to active PM user ids.
	ResolveUsersByOpenID(ctx context.Context, openIDs []string) (map[string]uint, error)
	// ResolveUniqueUsersByName maps display names to active PM user ids that
	// have no open_id yet; ambiguous names resolve to nothing.
	ResolveUniqueUsersByName(ctx context.Context, names []string) (map[string]uint, error)
	// SetUserDepartments writes the user→department mappings and adopts open ids.
	SetUserDepartments(ctx context.Context, assignments []FeishuUserDepartmentAssignment) error
	// ClearUserDepartments empties the department of every mapped user not in
	// the kept set (departed or no longer matched contacts).
	ClearUserDepartments(ctx context.Context, keepUserIDs []uint) error
	// FeishuOrgStatus reports the operator-facing projection state.
	FeishuOrgStatus(ctx context.Context) (FeishuOrgSyncStatus, error)
}

// feishuOrgRefreshInterval bounds how often the background refresh runs.
const feishuOrgRefreshInterval = 6 * time.Hour

// ErrFeishuOrgSyncInProgress reports that a manual sync was requested while
// another sync is already running.
var ErrFeishuOrgSyncInProgress = errors.New("feishu org sync is already in progress")

// FeishuOrgSyncStats summarises one completed sync pass for operator feedback.
type FeishuOrgSyncStats struct {
	Departments      int `json:"departments"`
	Contacts         int `json:"contacts"`
	MatchedUsers     int `json:"matched_users"`
	UnmatchedContact int `json:"unmatched_contacts"`
}

// FeishuOrgSyncStatus is the operator-facing state of the org projection.
type FeishuOrgSyncStatus struct {
	HasSynced     bool      `json:"has_synced"`
	SyncedAt      time.Time `json:"synced_at"`
	Departments   int       `json:"departments"`
	MappedUsers   int       `json:"mapped_users"`
	UnmappedUsers int       `json:"unmapped_users"`
}

// FeishuOrgSyncService syncs the Feishu org projection and keeps it fresh.
type FeishuOrgSyncService struct {
	source    FeishuOrgSource
	store     FeishuOrgStore
	enabled   bool
	inFlight  atomic.Bool
	manualMux sync.Mutex
	now       func() time.Time
}

func NewFeishuOrgSyncService(source FeishuOrgSource, store FeishuOrgStore, enabled bool) (*FeishuOrgSyncService, error) {
	if source == nil || store == nil {
		return nil, ErrReportingDirectoryQueryInvalid
	}
	return &FeishuOrgSyncService{source: source, store: store, enabled: enabled, now: time.Now}, nil
}

// Enabled reports whether the sync may run.
func (service *FeishuOrgSyncService) Enabled() bool {
	return service != nil && service.enabled
}

// Sync pulls the department tree and every department's users, then rewrites
// the local projection and member mappings in one pass.
func (service *FeishuOrgSyncService) Sync(ctx context.Context) (FeishuOrgSyncStats, error) {
	if !service.Enabled() {
		return FeishuOrgSyncStats{}, ErrReportingDirectoryQueryInvalid
	}
	departments, err := service.source.ListFeishuDepartments(ctx)
	if err != nil {
		return FeishuOrgSyncStats{}, err
	}
	// Collect every contact user across departments. The department a user
	// is found in IS their mapped department (first department wins for
	// cross-listed users): the contact payload's department_ids field is not
	// returned without extra field-level scopes, so it must not be relied on.
	type contactHit struct {
		user           FeishuDepartmentUser
		departmentOpen string
	}
	contacts := map[string]contactHit{}
	contactOrder := []string{}
	for _, department := range departments {
		users, err := service.source.ListFeishuDepartmentUsers(ctx, department.OpenID)
		if err != nil {
			return FeishuOrgSyncStats{}, err
		}
		for _, user := range users {
			if _, seen := contacts[user.OpenID]; seen {
				continue
			}
			contacts[user.OpenID] = contactHit{user: user, departmentOpen: department.OpenID}
			contactOrder = append(contactOrder, user.OpenID)
		}
	}

	openIDs := make([]string, 0, len(contacts))
	for _, openID := range contactOrder {
		openIDs = append(openIDs, openID)
	}
	byOpenID, err := service.store.ResolveUsersByOpenID(ctx, openIDs)
	if err != nil {
		return FeishuOrgSyncStats{}, err
	}
	// Name fallback only for contacts without an open_id match, and only
	// against active users that have no open_id yet.
	unmatchedNames := []string{}
	for _, openID := range contactOrder {
		if _, resolved := byOpenID[openID]; !resolved {
			unmatchedNames = append(unmatchedNames, contacts[openID].user.Name)
		}
	}
	byName, err := service.store.ResolveUniqueUsersByName(ctx, unmatchedNames)
	if err != nil {
		return FeishuOrgSyncStats{}, err
	}

	assignments := []FeishuUserDepartmentAssignment{}
	assigned := map[uint]bool{}
	kept := []uint{}
	for _, openID := range contactOrder {
		contact := contacts[openID]
		userID, resolved := byOpenID[openID]
		if !resolved {
			userID, resolved = byName[contact.user.Name]
		}
		if !resolved {
			continue
		}
		if assigned[userID] {
			continue
		}
		assigned[userID] = true
		kept = append(kept, userID)
		assignments = append(assignments, FeishuUserDepartmentAssignment{
			UserID: userID, OpenID: contact.user.OpenID, DepartmentID: contact.departmentOpen,
			JobTitle: contact.user.JobTitle,
		})
	}

	if err := service.store.ReplaceDepartments(ctx, departments); err != nil {
		return FeishuOrgSyncStats{}, err
	}
	if err := service.store.SetUserDepartments(ctx, assignments); err != nil {
		return FeishuOrgSyncStats{}, err
	}
	if err := service.store.ClearUserDepartments(ctx, kept); err != nil {
		return FeishuOrgSyncStats{}, err
	}
	return FeishuOrgSyncStats{
		Departments:      len(departments),
		Contacts:         len(contacts),
		MatchedUsers:     len(assignments),
		UnmatchedContact: len(contacts) - len(assignments),
	}, nil
}

// EnsureFresh triggers a background sync when the projection is older than
// the refresh interval (or empty). It never blocks the caller.
func (service *FeishuOrgSyncService) EnsureFresh(ctx context.Context) {
	if !service.Enabled() || !service.inFlight.CompareAndSwap(false, true) {
		return
	}
	last, exists, err := service.store.DepartmentSyncedAt(ctx)
	if err == nil && exists && service.now().Sub(last) < feishuOrgRefreshInterval {
		service.inFlight.Store(false)
		return
	}
	go func() {
		defer service.inFlight.Store(false)
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if _, err := service.Sync(bgCtx); err != nil {
			log.Printf("[feishu-org] background sync failed: %v", err)
		}
	}()
}

// SyncOnce runs a foreground sync guarded against concurrent runs; callers
// that need the fresh data immediately (startup) use it.
func (service *FeishuOrgSyncService) SyncOnce(ctx context.Context) (FeishuOrgSyncStats, error) {
	if !service.Enabled() {
		return FeishuOrgSyncStats{}, ErrReportingDirectoryQueryInvalid
	}
	if !service.inFlight.CompareAndSwap(false, true) {
		return FeishuOrgSyncStats{}, ErrFeishuOrgSyncInProgress
	}
	defer service.inFlight.Store(false)
	return service.Sync(ctx)
}

// SyncManually is the operator-triggered sync behind the admin button: it
// serialises against other manual syncs and fails fast with a dedicated
// error when one is already running, so double clicks never double-fetch.
func (service *FeishuOrgSyncService) SyncManually(ctx context.Context) (FeishuOrgSyncStats, error) {
	if !service.Enabled() {
		return FeishuOrgSyncStats{}, ErrReportingDirectoryQueryInvalid
	}
	if !service.manualMux.TryLock() {
		return FeishuOrgSyncStats{}, ErrFeishuOrgSyncInProgress
	}
	defer service.manualMux.Unlock()
	return service.Sync(ctx)
}

// Status reports the operator-facing org projection state.
func (service *FeishuOrgSyncService) Status(ctx context.Context) (FeishuOrgSyncStatus, error) {
	if !service.Enabled() {
		return FeishuOrgSyncStatus{}, ErrReportingDirectoryQueryInvalid
	}
	return service.store.FeishuOrgStatus(ctx)
}
