// Package identityorgworker runs the scheduled Feishu contact org sync on the
// shared business cadence: a one-minute tick drives the trigger judgment, and
// the tick that crosses 09:00 in the report schedule's business timezone syncs
// the Feishu org projection for every workspace owning a Feishu-bound member.
// The identity module stays the sole owner of the projection: this package
// only drives the identity-owned sync service and enumerates the covered
// workspaces with a read-only SELECT.
package identityorgworker

import (
	"context"
	"errors"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishu"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	report "github.com/xizhou0606/aurora-pm/backend/internal/module/report"
)

// syncTimeout bounds one workspace's sync pass so a hung Feishu call can never
// pin the worker loop; it matches the background refresh budget the identity
// module itself applies.
const syncTimeout = 5 * time.Minute

// OrgSyncer is the sync surface the worker drives. *identity.FeishuOrgSyncService
// satisfies it; tests substitute an in-memory fake.
type OrgSyncer interface {
	SyncOnce(ctx context.Context) (identity.FeishuOrgSyncStats, error)
}

// WorkspaceSource enumerates the workspaces the daily pass covers.
type WorkspaceSource interface {
	ListFeishuBoundWorkspaceIDs(ctx context.Context) ([]uint, error)
}

type Worker struct {
	sync       OrgSyncer
	workspaces WorkspaceSource
	tick       time.Duration
	now        func() time.Time
}

// New wires the worker to the identity org sync service and to the workspace
// source backing the daily 09:00 pass.
func New(sync OrgSyncer, workspaces WorkspaceSource) *Worker {
	return &Worker{sync: sync, workspaces: workspaces, tick: time.Minute, now: time.Now}
}

// Run blocks until ctx is done. The tick that crosses 09:00 in the schedule
// location runs one sync pass per Feishu-bound workspace. The anchor dedup
// keeps one occurrence firing at most once per process lifetime, and failures
// only log: the next day's occurrence retries naturally.
func (worker *Worker) Run(ctx context.Context) {
	log.Printf("identityorg worker: started (tick %s, feishu org sync daily 09:00)", worker.tick)
	nowFn := worker.now
	if nowFn == nil {
		nowFn = time.Now
	}
	// 触发判定复用 report 域的调度契约(Due + TriggerAnchor):代码同步、
	// 报告生成与组织同步在同一业务时区的同一 09:00 节奏下工作。
	location := report.ScheduleLocation()
	lastAnchor := ""
	previous := nowFn()
	ticker := time.NewTicker(worker.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("identityorg worker: stopped")
			return
		case <-ticker.C:
		}
		now := nowFn()
		anchor, due := dueAt(location, previous, now, lastAnchor)
		previous = now
		if !due {
			continue
		}
		lastAnchor = anchor
		worker.runScheduledSync(ctx)
	}
}

// dueAt reports the 09:00 trigger anchor when the daily 09:00 occurrence was
// crossed inside the half-open window (previous, now] and this process has
// not synced that occurrence yet — the same per-granularity anchor dedup the
// report and codecontrib workers apply.
func dueAt(location *time.Location, previous, now time.Time, lastAnchor string) (string, bool) {
	if !report.Due(report.PeriodDaily, location, previous, now) {
		return "", false
	}
	anchor, ok := report.TriggerAnchor(report.PeriodDaily, location, now)
	if !ok || anchor == lastAnchor {
		return "", false
	}
	return anchor, true
}

// runScheduledSync syncs every workspace owning a Feishu-bound member.
// Per-workspace failures stay visible in the log without aborting the pass
// (fail-visible): a failed workspace simply waits for the next day's pass.
func (worker *Worker) runScheduledSync(ctx context.Context) {
	if worker.sync == nil || worker.workspaces == nil {
		return
	}
	workspaceIDs, err := worker.workspaces.ListFeishuBoundWorkspaceIDs(ctx)
	if err != nil {
		log.Printf("identityorg worker: scheduled 09:00 pass workspace enumeration failed: %v", err)
		return
	}
	if len(workspaceIDs) == 0 {
		log.Printf("identityorg worker: scheduled 09:00 pass skipped: no Feishu-bound workspaces")
		return
	}
	for _, workspaceID := range workspaceIDs {
		worker.syncWorkspace(ctx, workspaceID)
	}
}

// syncWorkspace runs one bounded sync pass for one workspace. The in-progress
// conflict stays a skip, not an error: the manual endpoint or the identity
// background refresh owns that pass and the next occurrence retries.
func (worker *Worker) syncWorkspace(ctx context.Context, workspaceID uint) {
	syncCtx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()
	stats, err := worker.sync.SyncOnce(syncCtx)
	if err != nil {
		if errors.Is(err, identity.ErrFeishuOrgSyncInProgress) {
			log.Printf("identityorg worker: scheduled 09:00 sync for workspace %d skipped: %v", workspaceID, err)
			return
		}
		log.Printf("identityorg worker: scheduled 09:00 sync for workspace %d failed: %v", workspaceID, err)
		return
	}
	log.Printf("identityorg worker: scheduled 09:00 sync for workspace %d completed: departments=%d contacts=%d matched_users=%d",
		workspaceID, stats.Departments, stats.Contacts, stats.MatchedUsers)
}

// GormWorkspaceSource enumerates enabled workspaces that own at least one
// Feishu-bound active member (users.feishu_open_id set). It is a read-only
// SELECT on identity-owned tables: the identity module stays the sole writer
// of its schema.
type GormWorkspaceSource struct {
	db *gorm.DB
}

// NewGormWorkspaceSource wires the workspace enumeration to the PM database.
func NewGormWorkspaceSource(db *gorm.DB) *GormWorkspaceSource {
	return &GormWorkspaceSource{db: db}
}

// ListFeishuBoundWorkspaceIDs returns the distinct enabled workspaces with at
// least one active Feishu-bound member, in stable workspace-id order.
// feishu_tenant_id is deliberately NOT a binding marker: a tenant backfill in
// the login repository assigns it to every user once a Feishu login tenant
// exists, so it does not select Feishu-bound members.
func (source *GormWorkspaceSource) ListFeishuBoundWorkspaceIDs(ctx context.Context) ([]uint, error) {
	var ids []uint
	err := source.db.WithContext(ctx).
		Table("workspace_members AS wm").
		Joins("JOIN users AS u ON u.id = wm.user_id").
		Where("wm.enabled = ?", true).
		Where("u.active = ? AND u.status = ?", true, string(identity.AccountActive)).
		Where("u.feishu_open_id IS NOT NULL AND u.feishu_open_id <> ''").
		Distinct().
		Order("wm.workspace_id ASC").
		Pluck("wm.workspace_id", &ids).Error
	return ids, err
}

// feishuOrgSource adapts the Feishu SDK reader's contact methods to the
// identity-owned org sync port, mirroring the manual admin handler assembly.
type feishuOrgSource struct {
	reader *feishu.SDKReader
}

func (source feishuOrgSource) ListFeishuDepartments(
	ctx context.Context,
) ([]identity.FeishuDepartment, error) {
	departments, err := source.reader.ListFeishuDepartments(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]identity.FeishuDepartment, 0, len(departments))
	for _, department := range departments {
		result = append(result, identity.FeishuDepartment{
			OpenID: department.OpenID, ParentID: department.ParentID,
			Name: department.Name, Order: department.Order, MemberCount: department.MemberCount,
		})
	}
	return result, nil
}

func (source feishuOrgSource) ListFeishuDepartmentUsers(
	ctx context.Context,
	departmentID string,
) ([]identity.FeishuDepartmentUser, error) {
	users, err := source.reader.ListFeishuDepartmentUsers(ctx, departmentID)
	if err != nil {
		return nil, err
	}
	result := make([]identity.FeishuDepartmentUser, 0, len(users))
	for _, user := range users {
		result = append(result, identity.FeishuDepartmentUser{
			OpenID: user.OpenID, Name: user.Name, DepartmentID: user.DepartmentID,
			JobTitle: user.JobTitle,
		})
	}
	return result, nil
}

// NewSyncService assembles the identity Feishu org sync service exactly like
// the manual admin endpoint wiring: SDK contact reader plus identity-owned
// store repository.
func NewSyncService(reader *feishu.SDKReader, db *gorm.DB) (*identity.FeishuOrgSyncService, error) {
	return identity.NewFeishuOrgSyncService(
		feishuOrgSource{reader: reader},
		identitygorm.NewFeishuOrgStoreRepository(db),
		true,
	)
}
