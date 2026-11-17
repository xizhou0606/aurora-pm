package workflowtestcase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

var (
	ErrStorageUnavailable = errors.New("test case object storage is unavailable")
	ErrStorageUpload      = errors.New("test case object upload failed")
	ErrRunNotEligible     = errors.New("AI run is not eligible for test case delivery")
	ErrSubjectStale       = errors.New("AI run subject changed before test case delivery")
)

type ObjectStore interface {
	PutObject(context.Context, string, io.Reader, int64, string) error
}

type readinessStore interface {
	CheckReady(context.Context) error
}

type Materializer struct {
	db       *gorm.DB
	cfg      config.ObjectStorage
	store    ObjectStore
	storeErr error
}

func NewMaterializer(db *gorm.DB, cfg config.ObjectStorage, store ObjectStore, storeErr error) *Materializer {
	return &Materializer{db: db, cfg: cfg, store: store, storeErr: storeErr}
}

func (m *Materializer) Materialize(ctx context.Context, run aigorm.AIRun) (workflowgorm.WorkflowNodeArtifact, bool, error) {
	if m == nil || m.db == nil {
		return workflowgorm.WorkflowNodeArtifact{}, false, ErrStorageUnavailable
	}
	var current aigorm.AIRun
	if err := m.db.Where("id = ? AND workspace_id = ?", run.ID, run.WorkspaceID).First(&current).Error; err != nil {
		return workflowgorm.WorkflowNodeArtifact{}, false, fmt.Errorf("%w: run not found", ErrRunNotEligible)
	}
	if current.Capability != "qa.test_cases" || (current.Status != "completed" && current.Status != "applied") {
		return workflowgorm.WorkflowNodeArtifact{}, false, ErrRunNotEligible
	}
	target, found, err := ParseTarget(current.InputJSON)
	if err != nil || !found {
		if err == nil {
			err = ErrTargetInvalid
		}
		return workflowgorm.WorkflowNodeArtifact{}, false, err
	}
	instance, project, err := m.deliveryHierarchy(current, target)
	if err != nil {
		return workflowgorm.WorkflowNodeArtifact{}, false, err
	}
	key := DeterministicObjectKey(m.cfg, instance, project, target, current.ID)
	if existing, ok, lookupErr := m.existing(current, target, key); lookupErr != nil {
		return workflowgorm.WorkflowNodeArtifact{}, false, lookupErr
	} else if ok {
		return existing, false, nil
	}
	publicURL, valid := objectstore.PublicURL(m.cfg, key)
	if !valid {
		return workflowgorm.WorkflowNodeArtifact{}, false, ErrStorageUnavailable
	}
	if err := m.validateSubjectFreshness(m.db, current, instance); err != nil {
		return workflowgorm.WorkflowNodeArtifact{}, false, err
	}
	if !m.cfg.Enabled || m.store == nil || m.storeErr != nil {
		return workflowgorm.WorkflowNodeArtifact{}, false, ErrStorageUnavailable
	}
	if checker, ok := m.store.(readinessStore); ok {
		if err := checker.CheckReady(ctx); err != nil {
			return workflowgorm.WorkflowNodeArtifact{}, false, fmt.Errorf("%w: readiness check failed", ErrStorageUnavailable)
		}
	}
	csv, err := BuildCSV(json.RawMessage(current.OutputJSON))
	if err != nil {
		return workflowgorm.WorkflowNodeArtifact{}, false, err
	}
	if err := m.store.PutObject(ctx, key, bytes.NewReader(csv), int64(len(csv)), CSVContentType); err != nil {
		return workflowgorm.WorkflowNodeArtifact{}, false, fmt.Errorf("%w: %v", ErrStorageUpload, err)
	}
	// Close the check/upload race: a subject edit during the COS write may leave
	// an unreferenced deterministic object, but it must never create a stale PM artifact.
	if err := m.validateSubjectFreshness(m.db, current, instance); err != nil {
		return workflowgorm.WorkflowNodeArtifact{}, false, err
	}
	row := workflowgorm.WorkflowNodeArtifact{
		WorkspaceID: current.WorkspaceID, InstanceID: target.InstanceID, NodeID: target.NodeID,
		Kind: "ai_generated", Name: canonicalArtifactName(target.InstanceID),
		ContentType: CSVContentType, Size: int64(len(csv)), ObjectKey: key, URL: publicURL,
		AIRunID: &current.ID, UploaderUserID: current.RequestedByUser,
	}
	if err := m.db.Transaction(func(tx *gorm.DB) error {
		// Project-scoped writers acquire this lock before changing the subject or
		// its requirement/task context. Recheck freshness while that gate is held
		// so no stale snapshot can be attached between validation and commit.
		var lockedProject projectgorm.ProjectSet
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where(
			"id = ? AND workspace_id = ?", project.ID, current.WorkspaceID,
		).First(&lockedProject).Error; err != nil {
			return fmt.Errorf("%w: workflow project not found", ErrTargetInvalid)
		}
		if err := m.validateSubjectFreshness(tx, current, instance); err != nil {
			return err
		}
		var lockedInstance workflowgorm.WorkflowInstance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where(
			"id = ? AND workspace_id = ? AND subject_type = ? AND subject_id = ?",
			target.InstanceID, current.WorkspaceID, current.SubjectType, current.SubjectID,
		).First(&lockedInstance).Error; err != nil {
			return fmt.Errorf("%w: workflow instance not found", ErrTargetInvalid)
		}
		var lockedRun aigorm.AIRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where(
			"id = ? AND workspace_id = ? AND subject_type = ? AND subject_id = ? AND status IN ?",
			current.ID, current.WorkspaceID, current.SubjectType, current.SubjectID, []string{"completed", "applied"},
		).First(&lockedRun).Error; err != nil {
			return fmt.Errorf("%w: run not found", ErrRunNotEligible)
		}
		return tx.Create(&row).Error
	}); err != nil {
		existing, ok, lookupErr := m.existing(current, target, key)
		if lookupErr != nil {
			return workflowgorm.WorkflowNodeArtifact{}, false, lookupErr
		}
		if ok {
			return existing, false, nil
		}
		return workflowgorm.WorkflowNodeArtifact{}, false, err
	}
	return row, true, nil
}

func (m *Materializer) existing(run aigorm.AIRun, target Target, key string) (workflowgorm.WorkflowNodeArtifact, bool, error) {
	var row workflowgorm.WorkflowNodeArtifact
	err := m.db.Where("object_key = ?", key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return workflowgorm.WorkflowNodeArtifact{}, false, nil
	}
	if err != nil {
		return workflowgorm.WorkflowNodeArtifact{}, false, err
	}
	if row.WorkspaceID != run.WorkspaceID || row.InstanceID != target.InstanceID || row.NodeID != target.NodeID ||
		row.AIRunID == nil || *row.AIRunID != run.ID || row.Kind != "ai_generated" || row.ObjectKey != key {
		return workflowgorm.WorkflowNodeArtifact{}, false, fmt.Errorf("%w: deterministic object key belongs to another artifact", ErrRunNotEligible)
	}
	return row, true, nil
}

func (m *Materializer) deliveryHierarchy(run aigorm.AIRun, target Target) (workflowgorm.WorkflowInstance, projectgorm.ProjectSet, error) {
	var instance workflowgorm.WorkflowInstance
	if err := m.db.Where("id = ? AND workspace_id = ?", target.InstanceID, run.WorkspaceID).First(&instance).Error; err != nil {
		return instance, projectgorm.ProjectSet{}, fmt.Errorf("%w: workflow instance not found", ErrTargetInvalid)
	}
	if instance.SubjectType != run.SubjectType || instance.SubjectID != run.SubjectID {
		return instance, projectgorm.ProjectSet{}, fmt.Errorf("%w: workflow subject mismatch", ErrTargetInvalid)
	}
	if err := m.validateTargetNode(instance, target.NodeID); err != nil {
		return instance, projectgorm.ProjectSet{}, err
	}
	projectID := uint(0)
	switch instance.SubjectType {
	case "requirement":
		var subject requirementgorm.Requirement
		if err := m.db.Select("project_new_id").Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).First(&subject).Error; err != nil {
			return instance, projectgorm.ProjectSet{}, fmt.Errorf("%w: workflow requirement not found", ErrTargetInvalid)
		}
		projectID = subject.ProjectSetID
	case "plan_unit":
		var subject projectgorm.PlanUnit
		if err := m.db.Select("project_new_id").Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).First(&subject).Error; err != nil {
			return instance, projectgorm.ProjectSet{}, fmt.Errorf("%w: workflow plan unit not found", ErrTargetInvalid)
		}
		projectID = subject.ProjectSetID
	default:
		return instance, projectgorm.ProjectSet{}, fmt.Errorf("%w: unsupported workflow subject", ErrTargetInvalid)
	}
	var project projectgorm.ProjectSet
	if projectID == 0 || m.db.Where("id = ? AND workspace_id = ?", projectID, instance.WorkspaceID).First(&project).Error != nil {
		return instance, project, fmt.Errorf("%w: workflow project not found", ErrTargetInvalid)
	}
	return instance, project, nil
}

func (m *Materializer) validateSubjectFreshness(db *gorm.DB, run aigorm.AIRun, instance workflowgorm.WorkflowInstance) error {
	if instance.SubjectType == "requirement" {
		var subject requirementgorm.Requirement
		if err := db.Select("version").Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).First(&subject).Error; err != nil {
			return fmt.Errorf("%w: workflow requirement not found", ErrTargetInvalid)
		}
		if subject.Version != run.SubjectVersion {
			return ErrSubjectStale
		}
	}
	return m.validateContextFreshness(db, run, instance)
}

type contextFreshnessSnapshot struct {
	Subject struct {
		UpdatedAt time.Time `json:"updated_at"`
	} `json:"subject"`
	TestCaseContext *struct {
		Requirements []struct {
			ID      uint `json:"id"`
			Version uint `json:"version"`
		} `json:"requirements"`
		Tasks []struct {
			ID        uint      `json:"id"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"tasks"`
	} `json:"test_case_context"`
}

func (m *Materializer) validateContextFreshness(db *gorm.DB, run aigorm.AIRun, instance workflowgorm.WorkflowInstance) error {
	var snapshot contextFreshnessSnapshot
	if err := json.Unmarshal([]byte(run.InputJSON), &snapshot); err != nil {
		return fmt.Errorf("%w: invalid input snapshot", ErrRunNotEligible)
	}
	if snapshot.TestCaseContext == nil {
		return nil
	}
	if instance.SubjectType == "plan_unit" && !snapshot.Subject.UpdatedAt.IsZero() {
		var unit projectgorm.PlanUnit
		if err := db.Select("updated_at").Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).First(&unit).Error; err != nil {
			return fmt.Errorf("%w: workflow plan unit not found", ErrTargetInvalid)
		}
		if !unit.UpdatedAt.Equal(snapshot.Subject.UpdatedAt) {
			return ErrSubjectStale
		}
	}

	var currentRequirements []requirementgorm.Requirement
	query := db.Select("id", "version").Where("workspace_id = ?", instance.WorkspaceID)
	if instance.SubjectType == "requirement" {
		query = query.Where("id = ?", instance.SubjectID)
	} else {
		query = query.Where("version_task_new_id = ?", instance.SubjectID)
	}
	if err := query.Order("id").Find(&currentRequirements).Error; err != nil {
		return err
	}
	storedRequirements := make(map[uint]uint, len(snapshot.TestCaseContext.Requirements))
	for _, requirement := range snapshot.TestCaseContext.Requirements {
		if requirement.ID == 0 {
			return ErrSubjectStale
		}
		if _, duplicate := storedRequirements[requirement.ID]; duplicate {
			return ErrSubjectStale
		}
		storedRequirements[requirement.ID] = requirement.Version
	}
	if len(currentRequirements) != len(storedRequirements) {
		return ErrSubjectStale
	}
	requirementIDs := make([]uint, 0, len(currentRequirements))
	for _, requirement := range currentRequirements {
		if storedRequirements[requirement.ID] != requirement.Version {
			return ErrSubjectStale
		}
		requirementIDs = append(requirementIDs, requirement.ID)
	}

	if instance.SubjectType == "requirement" {
		return nil
	}

	var currentTasks []taskgorm.WorkTask
	if len(requirementIDs) > 0 {
		if err := db.Select("id", "updated_at").Where(
			"workspace_id = ? AND requirement_id IN ?", instance.WorkspaceID, requirementIDs,
		).Order("id").Find(&currentTasks).Error; err != nil {
			return err
		}
	}
	storedTasks := make(map[uint]time.Time, len(snapshot.TestCaseContext.Tasks))
	for _, task := range snapshot.TestCaseContext.Tasks {
		if task.ID == 0 || task.UpdatedAt.IsZero() {
			return ErrSubjectStale
		}
		if _, duplicate := storedTasks[task.ID]; duplicate {
			return ErrSubjectStale
		}
		storedTasks[task.ID] = task.UpdatedAt
	}
	if len(currentTasks) != len(storedTasks) {
		return ErrSubjectStale
	}
	for _, task := range currentTasks {
		updatedAt, found := storedTasks[task.ID]
		if !found || !task.UpdatedAt.Equal(updatedAt) {
			return ErrSubjectStale
		}
	}
	return nil
}

func canonicalArtifactName(instanceID uint) string {
	return fmt.Sprintf("测试用例-流程%d.csv", instanceID)
}

func (m *Materializer) validateTargetNode(instance workflowgorm.WorkflowInstance, nodeID string) error {
	var version workflowgorm.WorkflowTemplateVersion
	if err := m.db.Select("definition").Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, instance.WorkspaceID).First(&version).Error; err != nil {
		return fmt.Errorf("%w: workflow definition not found", ErrTargetInvalid)
	}
	var definition struct {
		Nodes []struct {
			ID     string         `json:"id"`
			Type   string         `json:"type"`
			Name   string         `json:"name"`
			Config map[string]any `json:"config"`
		} `json:"nodes"`
	}
	if json.Unmarshal([]byte(version.Definition), &definition) != nil {
		return fmt.Errorf("%w: invalid workflow definition", ErrTargetInvalid)
	}
	// Selection and assignee authorization are captured when the immutable run
	// target is created. Workflow progress while the model is running must not
	// discard a legitimate result; only the exact bound node can receive it.
	for _, node := range definition.Nodes {
		configured, _ := node.Config["test_case_artifact"].(bool)
		if node.ID == nodeID && (node.Type == "human_task" || node.Type == "form") &&
			(configured || strings.Contains(node.Name, "测试用例") || strings.Contains(node.Name, "测试案例")) {
			return nil
		}
	}
	return fmt.Errorf("%w: test case workflow node not found", ErrTargetInvalid)
}

func DeterministicObjectKey(cfg config.ObjectStorage, instance workflowgorm.WorkflowInstance, project projectgorm.ProjectSet, target Target, runID uint) string {
	segments := make([]string, 0, 9)
	if prefix := strings.Trim(strings.TrimSpace(cfg.GlobalPrefix), "/"); prefix != "" {
		segments = append(segments, prefix)
	}
	segments = append(segments, fmt.Sprintf("workspace/%d", instance.WorkspaceID))
	if cfg.TeamPrefix && project.TeamID > 0 {
		segments = append(segments, fmt.Sprintf("team/%d", project.TeamID))
	}
	segments = append(segments, fmt.Sprintf("project/%d", project.ID), "workflow_test_cases",
		fmt.Sprintf("instance/%d", target.InstanceID), safePathSegment(target.NodeID), fmt.Sprintf("run/%d.csv", runID))
	return filepath.ToSlash(strings.Join(segments, "/"))
}

func safePathSegment(value string) string {
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, strings.TrimSpace(value))
	if value == "" {
		return "node"
	}
	return value
}
