package resourceaccessgorm

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PolicyRevisionEventAppenderFactory func(*gorm.DB) (resourceaccess.PolicyRevisionEventAppender, error)

type Repository struct {
	requestClock                resourceaccess.Clock
	requestNotificationAppender func(*gorm.DB, resourceaccess.RequestMutation, uint, uint) error
	requestOwnerGuard           func(*gorm.DB, resourceaccess.RequestMutation) error
	db                          *gorm.DB
	policyEventAppenderFactory  PolicyRevisionEventAppenderFactory
}

func (repository *Repository) ConfigureRequestNotificationAppender(appender func(*gorm.DB, resourceaccess.RequestMutation, uint, uint) error) {
	repository.requestNotificationAppender = appender
}

func (repository *Repository) ConfigureRequestOwnerGuard(guard func(*gorm.DB, resourceaccess.RequestMutation) error) {
	repository.requestOwnerGuard = guard
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db, requestClock: resourceaccess.SystemClock{}}
}

func (repository *Repository) ConfigureRequestClock(clock resourceaccess.Clock) {
	repository.requestClock = clock
}

func (repository *Repository) ConfigurePolicyRevisionEventAppenderFactory(factory PolicyRevisionEventAppenderFactory) {
	repository.policyEventAppenderFactory = factory
}

// CreatePrivatePolicy is called only from the owning resource-creation
// transaction. The caller must supply a transaction-bound Repository.
func (repository *Repository) CreatePrivatePolicy(
	ctx context.Context,
	principal resourceaccess.Principal,
	resourceID documentlibrary.ResourceID,
	now time.Time,
) (resourceaccess.Policy, error) {
	if repository == nil || repository.db == nil || principal.WorkspaceID == 0 || principal.UserID == 0 || resourceID == "" {
		return resourceaccess.Policy{}, resourceaccess.ErrInvalidInput
	}
	now = now.UTC()
	record := PolicyRecord{
		WorkspaceID: principal.WorkspaceID, ResourceID: string(resourceID), Mode: string(resourceaccess.ModePrivate),
		Revision: 1, CreatedBy: principal.UserID, UpdatedBy: principal.UserID, CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.db.WithContext(ctx).Create(&record).Error; err != nil {
		return resourceaccess.Policy{}, err
	}
	created := policyFromRecord(record, []resourceaccess.Rule{})
	if err := appendAuditAndOutbox(
		repository.db.WithContext(ctx), principal, resourceID, "policy.created",
		resourceaccess.Policy{}, created, now,
	); err != nil {
		return resourceaccess.Policy{}, err
	}
	return created, nil
}

func (repository *Repository) GetPolicy(
	ctx context.Context, workspaceID uint, resourceID documentlibrary.ResourceID,
) (resourceaccess.Policy, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || resourceID == "" {
		return resourceaccess.Policy{}, resourceaccess.ErrInvalidInput
	}
	return loadPolicy(ctx, repository.db, workspaceID, resourceID)
}

// LockPolicyRevision is used by a caller-owned content mutation transaction.
// Holding this row lock through commit prevents a concurrent policy
// replacement from committing between final authorization and the content
// write. The repository never opens or commits a transaction here.
func (repository *Repository) LockPolicyRevision(
	ctx context.Context,
	workspaceID uint,
	resourceID documentlibrary.ResourceID,
	expectedRevision uint64,
) (resourceaccess.Policy, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || resourceID == "" || expectedRevision == 0 {
		return resourceaccess.Policy{}, resourceaccess.ErrInvalidInput
	}
	var row PolicyRecord
	if err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND resource_id = ?", workspaceID, string(resourceID),
	).First(&row).Error; err != nil {
		return resourceaccess.Policy{}, mapNotFound(err)
	}
	if row.Revision != expectedRevision {
		return resourceaccess.Policy{}, resourceaccess.ErrConflict
	}
	return loadPolicyWithRecord(repository.db.WithContext(ctx), row)
}

// LockCurrentPolicy acquires the policy row in a caller-owned transaction and
// returns its current revision. It is used when a transaction must perform a
// fresh authorization decision before it has an expected revision to compare.
// The lock is held by the caller's transaction through the content commit.
func (repository *Repository) LockCurrentPolicy(
	ctx context.Context,
	workspaceID uint,
	resourceID documentlibrary.ResourceID,
) (resourceaccess.Policy, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || resourceID == "" {
		return resourceaccess.Policy{}, resourceaccess.ErrInvalidInput
	}
	var row PolicyRecord
	if err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND resource_id = ?", workspaceID, string(resourceID),
	).First(&row).Error; err != nil {
		return resourceaccess.Policy{}, mapNotFound(err)
	}
	return loadPolicyWithRecord(repository.db.WithContext(ctx), row)
}

func (repository *Repository) ListPolicies(
	ctx context.Context, workspaceID uint, resourceIDs []documentlibrary.ResourceID,
) (map[documentlibrary.ResourceID]resourceaccess.Policy, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || len(resourceIDs) == 0 {
		return nil, resourceaccess.ErrInvalidInput
	}
	ids := stringResourceIDs(resourceIDs)
	var rows []PolicyRecord
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND resource_id IN ?", workspaceID, ids).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	var ruleRows []RuleRecord
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND resource_id IN ?", workspaceID, ids).
		Order("resource_id ASC, subject_kind ASC, subject_id ASC").Find(&ruleRows).Error; err != nil {
		return nil, err
	}
	rules := make(map[string][]resourceaccess.Rule)
	for _, row := range ruleRows {
		rules[row.ResourceID] = append(rules[row.ResourceID], ruleFromRecord(row))
	}
	result := make(map[documentlibrary.ResourceID]resourceaccess.Policy, len(rows))
	for _, row := range rows {
		policy := policyFromRecord(row, rules[row.ResourceID])
		result[policy.ResourceID] = policy
	}
	return result, nil
}

func (repository *Repository) ReplacePolicy(
	ctx context.Context, mutation resourceaccess.PolicyMutation,
) (resourceaccess.Policy, error) {
	if repository == nil || repository.db == nil {
		return resourceaccess.Policy{}, resourceaccess.ErrUnavailable
	}
	var result resourceaccess.Policy
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if replay, found, err := replayPolicyMutation(tx, mutation); found || err != nil {
			result = replay
			return err
		}
		var current PolicyRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("workspace_id = ? AND resource_id = ?", mutation.Principal.WorkspaceID, string(mutation.ResourceID)).
			First(&current).Error; err != nil {
			return mapNotFound(err)
		}
		if mutation.ExpectedRevision == 0 || current.Revision != mutation.ExpectedRevision {
			return resourceaccess.ErrConflict
		}
		before, err := loadPolicyWithRecord(tx, current)
		if err != nil {
			return err
		}
		workspaceLevel := levelString(mutation.WorkspaceLevel)
		nextRevision := current.Revision + 1
		updates := map[string]any{
			"mode": mutation.Mode, "workspace_level": workspaceLevel, "revision": nextRevision,
			"updated_by": mutation.Principal.UserID, "updated_at": mutation.Now,
		}
		if err := tx.Model(&current).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ? AND resource_id = ?", mutation.Principal.WorkspaceID, mutation.ResourceID).
			Delete(&RuleRecord{}).Error; err != nil {
			return err
		}
		if len(mutation.Rules) > 0 {
			rows := make([]RuleRecord, 0, len(mutation.Rules))
			for _, rule := range mutation.Rules {
				rows = append(rows, RuleRecord{
					WorkspaceID: mutation.Principal.WorkspaceID, ResourceID: string(mutation.ResourceID),
					SubjectKind: string(rule.SubjectKind), SubjectID: rule.SubjectID, Level: string(rule.Level),
					IncludeDescendants: rule.IncludeDescendants, CreatedAt: mutation.Now,
				})
			}
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		current.Mode, current.WorkspaceLevel = string(mutation.Mode), workspaceLevel
		current.Revision, current.UpdatedBy, current.UpdatedAt = nextRevision, mutation.Principal.UserID, mutation.Now
		result = policyFromRecord(current, mutation.Rules)
		if err := appendAuditAndOutbox(tx, mutation.Principal, mutation.ResourceID, "policy.replaced", before, result, mutation.Now); err != nil {
			return err
		}
		if repository.policyEventAppenderFactory == nil {
			return resourceaccess.ErrUnavailable
		}
		appender, err := repository.policyEventAppenderFactory(tx)
		if err != nil {
			return err
		}
		if appender == nil {
			return resourceaccess.ErrUnavailable
		}
		if err := appender.AppendPolicyRevisionEvent(ctx, resourceaccess.PolicyRevisionEvent{
			WorkspaceID: mutation.Principal.WorkspaceID, ResourceID: mutation.ResourceID,
			ActorUserID: mutation.Principal.UserID, IdempotencyKey: mutation.IdempotencyKey,
			PreviousRevision: before.Revision, PolicyRevision: result.Revision, Mode: result.Mode,
		}); err != nil {
			return err
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return tx.Create(&MutationReceiptRecord{
			WorkspaceID: mutation.Principal.WorkspaceID, ResourceID: string(mutation.ResourceID), Operation: "policy.replace",
			IdempotencyKey: mutation.IdempotencyKey, InputDigest: mutation.InputDigest,
			ResultJSON: string(encoded), CreatedAt: mutation.Now,
		}).Error
	})
	return result, err
}

func loadPolicy(
	ctx context.Context, db *gorm.DB, workspaceID uint, resourceID documentlibrary.ResourceID,
) (resourceaccess.Policy, error) {
	var row PolicyRecord
	if err := db.WithContext(ctx).Where("workspace_id = ? AND resource_id = ?", workspaceID, string(resourceID)).
		First(&row).Error; err != nil {
		return resourceaccess.Policy{}, mapNotFound(err)
	}
	return loadPolicyWithRecord(db.WithContext(ctx), row)
}

func loadPolicyWithRecord(db *gorm.DB, row PolicyRecord) (resourceaccess.Policy, error) {
	var rules []RuleRecord
	if err := db.Where("workspace_id = ? AND resource_id = ?", row.WorkspaceID, row.ResourceID).
		Order("subject_kind ASC, subject_id ASC").Find(&rules).Error; err != nil {
		return resourceaccess.Policy{}, err
	}
	items := make([]resourceaccess.Rule, 0, len(rules))
	for _, rule := range rules {
		items = append(items, ruleFromRecord(rule))
	}
	return policyFromRecord(row, items), nil
}

func replayPolicyMutation(tx *gorm.DB, mutation resourceaccess.PolicyMutation) (resourceaccess.Policy, bool, error) {
	var receipt MutationReceiptRecord
	err := tx.Where(
		"workspace_id = ? AND resource_id = ? AND operation = ? AND idempotency_key = ?",
		mutation.Principal.WorkspaceID, mutation.ResourceID, "policy.replace", mutation.IdempotencyKey,
	).First(&receipt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return resourceaccess.Policy{}, false, nil
	}
	if err != nil {
		return resourceaccess.Policy{}, false, err
	}
	if receipt.InputDigest != mutation.InputDigest {
		return resourceaccess.Policy{}, true, resourceaccess.ErrIdempotencyConflict
	}
	var result resourceaccess.Policy
	if err := json.Unmarshal([]byte(receipt.ResultJSON), &result); err != nil {
		return resourceaccess.Policy{}, true, err
	}
	return result, true, nil
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return resourceaccess.ErrNotFound
	}
	return err
}

func stringResourceIDs(ids []documentlibrary.ResourceID) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		result = append(result, string(id))
	}
	return result
}

func levelString(level *resourceaccess.Level) *string {
	if level == nil {
		return nil
	}
	value := string(*level)
	return &value
}

func policyFromRecord(row PolicyRecord, rules []resourceaccess.Rule) resourceaccess.Policy {
	var workspaceLevel *resourceaccess.Level
	if row.WorkspaceLevel != nil {
		value := resourceaccess.Level(*row.WorkspaceLevel)
		workspaceLevel = &value
	}
	if rules == nil {
		rules = []resourceaccess.Rule{}
	}
	return resourceaccess.Policy{
		WorkspaceID: row.WorkspaceID, ResourceID: documentlibrary.ResourceID(row.ResourceID),
		Mode: resourceaccess.Mode(row.Mode), WorkspaceLevel: workspaceLevel, Rules: rules,
		Revision: row.Revision, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func ruleFromRecord(row RuleRecord) resourceaccess.Rule {
	return resourceaccess.Rule{
		SubjectKind: identity.SubjectKind(row.SubjectKind), SubjectID: row.SubjectID,
		Level: resourceaccess.Level(row.Level), IncludeDescendants: row.IncludeDescendants,
	}
}

var _ resourceaccess.PolicyStore = (*Repository)(nil)
