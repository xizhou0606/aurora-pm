package native

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db                       *gorm.DB
	mutationRecorderFactory  func(*gorm.DB) (bitable.MutationRecorder, error)
	cellGuardFactory         func(*gorm.DB) (bitable.CellMutationGuard, error)
	cellConflictGuardFactory func(*gorm.DB) (bitable.CellLeaseConflictGuard, error)
	eventAppenderFactory     func(*gorm.DB) (bitable.CommittedEventAppender, error)
	valueNormalizer          func(context.Context, bitable.Principal, []bitable.Field, map[string]any) (map[string]any, error)
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) ConfigureValueNormalizer(fn func(context.Context, bitable.Principal, []bitable.Field, map[string]any) (map[string]any, error)) {
	r.valueNormalizer = fn
}
func (r *Repository) normalizeValues(ctx context.Context, p bitable.Principal, fields []bitable.Field, values map[string]any) (map[string]any, error) {
	if r.valueNormalizer != nil {
		return r.valueNormalizer(ctx, p, fields, values)
	}
	for _, f := range fields {
		if f.Type == bitable.FieldUser && values[f.ID] != nil {
			return nil, bitable.ErrUnavailable
		}
	}
	return values, nil
}
func (r *Repository) ConfigureMutationRecorderFactory(factory func(*gorm.DB) (bitable.MutationRecorder, error)) {
	r.mutationRecorderFactory = factory
}
func (r *Repository) ConfigureCellMutationGuardFactory(factory func(*gorm.DB) (bitable.CellMutationGuard, error)) {
	r.cellGuardFactory = factory
}
func (r *Repository) ConfigureCellLeaseConflictGuardFactory(factory func(*gorm.DB) (bitable.CellLeaseConflictGuard, error)) {
	r.cellConflictGuardFactory = factory
}
func (r *Repository) ConfigureCommittedEventAppenderFactory(factory func(*gorm.DB) (bitable.CommittedEventAppender, error)) {
	r.eventAppenderFactory = factory
}

func (r *Repository) cellGuard(tx *gorm.DB) (bitable.CellMutationGuard, error) {
	if r.cellGuardFactory == nil {
		return nil, bitable.ErrUnavailable
	}
	guard, err := r.cellGuardFactory(tx)
	if err != nil {
		return nil, err
	}
	if guard == nil {
		return nil, bitable.ErrUnavailable
	}
	return guard, nil
}

func (r *Repository) cellConflictGuard(tx *gorm.DB) (bitable.CellLeaseConflictGuard, error) {
	if r.cellConflictGuardFactory == nil {
		return nil, bitable.ErrUnavailable
	}
	guard, err := r.cellConflictGuardFactory(tx)
	if err != nil {
		return nil, err
	}
	if guard == nil {
		return nil, bitable.ErrUnavailable
	}
	return guard, nil
}

func (r *Repository) appendCommittedEvent(ctx context.Context, tx *gorm.DB, s bitable.Scope, operationID, eventType string, payload any) error {
	if r.eventAppenderFactory == nil {
		return bitable.ErrUnavailable
	}
	if s.DocumentResourceID == "" || s.PolicyRevision == 0 {
		return bitable.ErrUnavailable
	}
	appender, err := r.eventAppenderFactory(tx)
	if err != nil {
		return err
	}
	if appender == nil {
		return bitable.ErrUnavailable
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return appender.AppendBitableCommittedEvent(ctx, bitable.CommittedEvent{
		WorkspaceID: s.Principal.WorkspaceID, ResourceID: s.DocumentResourceID, TableID: s.TableID,
		OperationID: operationID, ActorUserID: s.Principal.UserID, Type: eventType,
		Payload: data, PolicyRevision: s.PolicyRevision,
	})
}
func (r *Repository) recordMutation(ctx context.Context, tx *gorm.DB, s bitable.Scope, key, kind string, ids []string) error {
	if r.mutationRecorderFactory == nil {
		return bitable.ErrUnavailable
	}
	recorder, e := r.mutationRecorderFactory(tx)
	if e != nil {
		return e
	}
	if recorder == nil {
		return bitable.ErrUnavailable
	}
	return recorder.RecordBitableMutation(ctx, bitable.MutationEffect{Principal: s.Principal, ResourceID: s.ResourceID, TableID: s.TableID, OperationID: key, Kind: kind, RecordIDs: ids})
}
func dbError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return bitable.ErrNotFound
	}
	return err
}
func encode(value any) (string, error) { data, e := json.Marshal(value); return string(data), e }
func hash(value any) (string, error) {
	data, e := json.Marshal(value)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
func scopeDB(db *gorm.DB, s bitable.Scope) *gorm.DB {
	return db.Where("workspace_id = ? AND resource_id = ? AND table_id = ?", s.Principal.WorkspaceID, s.ResourceID, s.TableID)
}
func tableDB(db *gorm.DB, s bitable.Scope) *gorm.DB {
	return db.Where("workspace_id = ? AND resource_id = ? AND id = ?", s.Principal.WorkspaceID, s.ResourceID, s.TableID)
}
func loadTable(db *gorm.DB, s bitable.Scope, lock bool) (TableRecord, []bitable.Field, []bitable.View, error) {
	var table TableRecord
	var resource ResourceRecord
	resources := db.Where("workspace_id = ? AND id = ? AND deleted = ?", s.Principal.WorkspaceID, s.ResourceID, false)
	if lock {
		resources = resources.Clauses(clause.Locking{Strength: "SHARE"})
	}
	if e := resources.First(&resource).Error; e != nil {
		return table, nil, nil, dbError(e)
	}
	q := tableDB(db, s)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if e := q.First(&table).Error; e != nil {
		return table, nil, nil, dbError(e)
	}
	fields := []bitable.Field{}
	views := []bitable.View{}
	if e := json.Unmarshal([]byte(table.Fields), &fields); e != nil {
		return table, nil, nil, e
	}
	if e := json.Unmarshal([]byte(table.Views), &views); e != nil {
		return table, nil, nil, e
	}
	return table, fields, views, nil
}
func (r *Repository) transaction(ctx context.Context, read bool, fn func(*gorm.DB) error) error {
	opts := &sql.TxOptions{}
	if read && r.db.Dialector.Name() == "postgres" {
		opts.Isolation = sql.LevelRepeatableRead
		opts.ReadOnly = true
	}
	return r.db.WithContext(ctx).Transaction(fn, opts)
}
func receipt(db *gorm.DB, s bitable.Scope, key, kind, requestHash string, result any) (bool, error) {
	var item ReceiptRecord
	e := db.Where("scope_key = ? AND operation_id = ?", s.TableID, key).First(&item).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if item.WorkspaceID != s.Principal.WorkspaceID || item.ActorID != s.Principal.UserID || item.ResourceID != s.ResourceID || item.Kind != kind || item.RequestHash != requestHash {
		return false, bitable.ErrConflict
	}
	return true, json.Unmarshal([]byte(item.Result), result)
}
func saveReceipt(db *gorm.DB, s bitable.Scope, key, kind, requestHash string, result any) error {
	data, e := encode(result)
	if e != nil {
		return e
	}
	return db.Create(&ReceiptRecord{ScopeKey: s.TableID, OperationID: key, WorkspaceID: s.Principal.WorkspaceID, ActorID: s.Principal.UserID, ResourceID: s.ResourceID, TableID: s.TableID, Kind: kind, RequestHash: requestHash, Result: data, CreatedAt: time.Now().UTC()}).Error
}
func (r *Repository) Get(ctx context.Context, workspace uint, id string) (bitable.Resource, error) {
	var resource ResourceRecord
	if e := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ? AND deleted = ?", workspace, id, false).First(&resource).Error; e != nil {
		return bitable.Resource{}, dbError(e)
	}
	var tables []TableRecord
	if e := r.db.WithContext(ctx).Select("id", "name").Where("workspace_id = ? AND resource_id = ?", workspace, id).Order("created_at ASC, id ASC").Find(&tables).Error; e != nil {
		return bitable.Resource{}, e
	}
	result := bitable.Resource{ID: id, Name: resource.Name, WorkspaceID: workspace, Tables: []bitable.Table{}, AllowedActions: []string{}}
	result.Revision = resource.Revision
	result.OwnerUserID = resource.OwnerUserID
	result.ParentFolderID = resource.ParentFolderID
	for _, table := range tables {
		result.Tables = append(result.Tables, bitable.Table{ID: table.ID, Name: table.Name})
	}
	return result, nil
}
func (r *Repository) List(ctx context.Context, workspace uint) ([]bitable.Resource, error) {
	var records []ResourceRecord
	if e := r.db.WithContext(ctx).Where("workspace_id = ? AND deleted = ?", workspace, false).Order("created_at DESC, id ASC").Limit(bitable.MaxExportRows + 1).Find(&records).Error; e != nil {
		return nil, e
	}
	if len(records) > bitable.MaxExportRows {
		return nil, bitable.ErrBudgetExceeded
	}
	result := []bitable.Resource{}
	for _, record := range records {
		item, e := r.Get(ctx, workspace, record.ID)
		if e != nil {
			return nil, e
		}
		result = append(result, item)
	}
	return result, nil
}
func defaultView() bitable.View {
	return bitable.View{ID: uuid.NewString(), Name: "表格视图", Kind: "grid", Revision: 1, Config: bitable.ViewConfig{HiddenFieldIDs: []string{}, ColumnOrder: []string{}, ColumnWidths: map[string]int{}, RowHeight: 36, Filters: []bitable.Filter{}, Sorts: []bitable.Sort{}}}
}
func (r *Repository) Create(ctx context.Context, p bitable.Principal, name, key string) (bitable.Resource, error) {
	result := bitable.Resource{}
	h, e := hash(name)
	if e != nil {
		return result, e
	}
	e = r.transaction(ctx, false, func(tx *gorm.DB) error {
		// PostgreSQL advisory locks serialize creation keys even before a resource row exists.
		scopeKey := fmt.Sprintf("create:%d:%d", p.WorkspaceID, p.UserID)
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", scopeKey+":"+key).Error; err != nil {
				return err
			}
		}
		var prior ReceiptRecord
		err := tx.Where("scope_key = ? AND operation_id = ?", scopeKey, key).First(&prior).Error
		if err == nil {
			if prior.RequestHash != h {
				return bitable.ErrConflict
			}
			return json.Unmarshal([]byte(prior.Result), &result)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		now := time.Now().UTC()
		id, tableID := uuid.NewString(), uuid.NewString()
		fields, err := encode([]bitable.Field{{ID: uuid.NewString(), Name: "名称", Type: bitable.FieldText}})
		if err != nil {
			return err
		}
		views, err := encode([]bitable.View{defaultView()})
		if err != nil {
			return err
		}
		if err = tx.Create(&ResourceRecord{ID: id, WorkspaceID: p.WorkspaceID, Name: name, Revision: 1, OwnerUserID: p.UserID, CreatedBy: p.UserID, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			return err
		}
		if err = tx.Create(&TableRecord{ID: tableID, ResourceID: id, WorkspaceID: p.WorkspaceID, Name: name, SchemaRevision: 1, DataRevision: 1, Fields: fields, Views: views, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			return err
		}
		result = bitable.Resource{ID: id, Name: name, WorkspaceID: p.WorkspaceID, Tables: []bitable.Table{{ID: tableID, Name: name}}, AllowedActions: []string{}}
		result.Revision = 1
		result.OwnerUserID = p.UserID
		if err = r.recordMutation(ctx, tx, bitable.Scope{Principal: p, ResourceID: id, TableID: tableID}, key, "resource.create", []string{}); err != nil {
			return err
		}
		payload, err := encode(result)
		if err != nil {
			return err
		}
		return tx.Create(&ReceiptRecord{ScopeKey: scopeKey, OperationID: key, WorkspaceID: p.WorkspaceID, ActorID: p.UserID, ResourceID: id, TableID: tableID, Kind: "create", RequestHash: h, Result: payload, CreatedAt: now}).Error
	})
	return result, e
}
func (r *Repository) Bootstrap(ctx context.Context, s bitable.Scope) (bitable.Bootstrap, error) {
	table, fields, views, e := loadTable(r.db.WithContext(ctx), s, false)
	return bitable.Bootstrap{TableID: s.TableID, SchemaRevision: table.SchemaRevision, Fields: fields, Views: views}, e
}
