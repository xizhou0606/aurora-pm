package resourceaccessgorm

import (
	"context"
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// Storage is shared, while authorization and membership effects stay with project.
type ProjectRequestHooks struct {
	Scope  func(*gorm.DB, project.JoinActor, project.JoinTarget, bool) (project.JoinScope, error)
	Grant  func(*gorm.DB, project.JoinScope, uint, uint, string) error
	Notify func(*gorm.DB, project.JoinScope, project.JoinMutation, project.JoinRequest, uint) error
	Names  func(*gorm.DB, []uint) (map[uint]string, error)
}
type ProjectRequestRepository struct {
	db    *gorm.DB
	hooks ProjectRequestHooks
}

func NewProjectRequestRepository(db *gorm.DB, h ProjectRequestHooks) *ProjectRequestRepository {
	return &ProjectRequestRepository{db: db, hooks: h}
}
func projectRequestQuery(db *gorm.DB, s project.JoinScope) *gorm.DB {
	q := db.Model(&AccessRequestRecord{}).Where("target_kind = ? AND workspace_id = ?", s.TargetKind, s.WorkspaceID)
	if s.TargetKind == "project_set" {
		return q.Where("project_set_id = ?", s.ProjectSetID)
	}
	if s.TargetKind == "workspace" {
		return q.Where("plan_unit_id IS NULL")
	}
	return q.Where("plan_unit_id = ?", s.PlanUnitID)
}
func (r *ProjectRequestRepository) Context(ctx context.Context, a project.JoinActor, t project.JoinTarget) (project.JoinContext, error) {
	s, err := r.hooks.Scope(r.db.WithContext(ctx), a, t, false)
	if err != nil {
		return project.JoinContext{}, err
	}
	q := projectRequestQuery(r.db.WithContext(ctx), s)
	if !s.CanReview {
		q = q.Where("applicant_user_id = ?", a.FactID)
	}
	var rows []AccessRequestRecord
	if err := q.Order("id DESC").Limit(101).Find(&rows).Error; err != nil {
		return project.JoinContext{}, err
	}
	out := project.JoinContext{JoinScope: s, Items: []project.JoinRequest{}, CanRequest: project.ValidateJoinScope(s, false) == nil, HasMore: len(rows) > 100}
	if len(rows) > 100 {
		rows = rows[:100]
	}
	ids := []uint{}
	for _, row := range rows {
		ids = append(ids, row.ApplicantUserID)
	}
	names, err := r.hooks.Names(r.db.WithContext(ctx), ids)
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		v := projectRequestView(row)
		v.ApplicantName = names[row.ApplicantUserID]
		if resourceaccess.RequestHasExpired(resourceaccess.RequestStatus(v.Status), v.CreatedAt, time.Now().UTC()) {
			v.Status = string(resourceaccess.RequestExpired)
		}
		if v.ApplicantUserID == a.FactID && v.Status == string(resourceaccess.RequestPending) {
			out.CanRequest = false
		}
		out.Items = append(out.Items, v)
	}
	return out, nil
}
func (r *ProjectRequestRepository) Mutate(ctx context.Context, m project.JoinMutation) (project.JoinRequest, error) {
	var result AccessRequestRecord
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		s, err := r.hooks.Scope(tx, m.Actor, m.Target, true)
		if err != nil {
			return err
		}
		if err := project.ValidateJoinScope(s, m.RequestID != 0); err != nil {
			return err
		}
		now := time.Now().UTC()
		q := projectRequestQuery(tx, s).Session(&gorm.Session{})
		if m.RequestID == 0 {
			if err := q.Where("applicant_user_id = ? AND status = ? AND created_at <= ?", m.Actor.FactID, resourceaccess.RequestPending, now.Add(-resourceaccess.RequestLifetime)).Update("status", resourceaccess.RequestExpired).Error; err != nil {
				return err
			}
			err = q.Where("applicant_user_id = ? AND status = ?", m.Actor.FactID, resourceaccess.RequestPending).First(&result).Error
			if err == nil {
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			// Project rows keep resource_id NULL, preserving the original document FK.
			data := map[string]any{"target_kind": s.TargetKind, "workspace_id": s.WorkspaceID, "plan_unit_id": s.PlanUnitID, "resource_id": nil, "applicant_user_id": m.Actor.FactID, "reason": m.Reason, "notify_owner": true, "status": resourceaccess.RequestPending, "read": false, "created_at": now, "grant_level": resourceaccess.LevelViewer}
			if s.TargetKind != "plan_unit" {
				data["plan_unit_id"] = nil
			}
			if s.TargetKind == "project_set" {
				data["project_set_id"] = s.ProjectSetID
			}
			if err := tx.Model(&AccessRequestRecord{}).Create(data).Error; err != nil {
				return err
			}
			if err := q.Where("applicant_user_id = ? AND status = ?", m.Actor.FactID, resourceaccess.RequestPending).First(&result).Error; err != nil {
				return err
			}
			return r.hooks.Notify(tx, s, m, projectRequestView(result), 0)
		}
		if err := q.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", m.RequestID).First(&result).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return project.ErrGuideUnavailable
			}
			return err
		}
		if string(result.Status) == m.Decision {
			return nil
		}
		if result.Status != resourceaccess.RequestPending || resourceaccess.RequestHasExpired(result.Status, result.CreatedAt, now) {
			return project.ErrJoinConflict
		}
		if m.Decision == string(resourceaccess.RequestApproved) {
			if err := r.hooks.Grant(tx, s, result.ApplicantUserID, m.Actor.FactID, result.Reason); err != nil {
				return err
			}
		}
		result.Status = resourceaccess.RequestStatus(m.Decision)
		result.Read = true
		result.ReviewedAt = &now
		result.ReviewedBy = &m.Actor.FactID
		// Updating explicit columns avoids writing a zero string into resource_id.
		if err := q.Where("id = ?", result.ID).Updates(map[string]any{"status": result.Status, "read": true, "reviewed_at": now, "reviewed_by": m.Actor.FactID}).Error; err != nil {
			return err
		}
		return r.hooks.Notify(tx, s, m, projectRequestView(result), result.ApplicantUserID)
	})
	return projectRequestView(result), err
}

func projectRequestView(row AccessRequestRecord) project.JoinRequest {
	return project.JoinRequest{ID: row.ID, ApplicantUserID: row.ApplicantUserID, Reason: row.Reason, Status: string(row.Status), CreatedAt: row.CreatedAt, ExpiresAt: resourceaccess.RequestExpiresAt(row.CreatedAt)}
}

// Read all entry states in one applicant-scoped query, without loading history or names.
func (r *ProjectRequestRepository) PendingMembershipKinds(ctx context.Context, a project.JoinActor, g project.AccessGuide) (map[string]bool, error) {
	var kinds []string
	err := r.db.WithContext(ctx).Model(&AccessRequestRecord{}).Where("workspace_id = ? AND applicant_user_id = ? AND status = ? AND created_at > ?", g.WorkspaceID, a.FactID, resourceaccess.RequestPending, time.Now().UTC().Add(-resourceaccess.RequestLifetime)).Where("target_kind = ? OR (target_kind = ? AND plan_unit_id = ?)", "workspace", "plan_unit", g.PlanUnitID).Distinct("target_kind").Pluck("target_kind", &kinds).Error
	out := map[string]bool{}
	for _, kind := range kinds {
		out[kind] = true
	}
	return out, err
}
