package workflowtestcase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
)

type memoryStore struct {
	mu        sync.Mutex
	objects   map[string][]byte
	keys      []string
	failWrite atomic.Bool
}

type objectStoreFunc func(context.Context, string, io.Reader, int64, string) error

func (put objectStoreFunc) PutObject(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	return put(ctx, key, body, size, contentType)
}

func (store *memoryStore) PutObject(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
	raw, _ := io.ReadAll(body)
	store.mu.Lock()
	defer store.mu.Unlock()
	store.keys = append(store.keys, key)
	if store.failWrite.Load() {
		return errors.New("temporary COS failure")
	}
	if store.objects == nil {
		store.objects = map[string][]byte{}
	}
	store.objects[key] = raw
	return nil
}

func TestMaterializerRetriesStorageFailureWithDeterministicKey(t *testing.T) {
	db, cfg, run := materializerFixture(t, "storage_retry")
	store := &memoryStore{}
	store.failWrite.Store(true)
	materializer := NewMaterializer(db, cfg, store, nil)
	if _, _, err := materializer.Materialize(context.Background(), run); !errors.Is(err, ErrStorageUpload) {
		t.Fatalf("first error=%v", err)
	}
	store.failWrite.Store(false)
	artifact, created, err := materializer.Materialize(context.Background(), run)
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	wantKey := "pm/workspace/1/team/10/project/11/workflow_test_cases/instance/40/test-cases/run/60.csv"
	if artifact.ObjectKey != wantKey || len(store.keys) != 2 || store.keys[0] != wantKey || store.keys[1] != wantKey {
		t.Fatalf("artifact key=%q attempts=%v", artifact.ObjectKey, store.keys)
	}
	duplicate, created, err := materializer.Materialize(context.Background(), run)
	if err != nil || created || duplicate.ID != artifact.ID || len(store.keys) != 2 {
		t.Fatalf("duplicate=%+v created=%v err=%v attempts=%v", duplicate, created, err, store.keys)
	}
}

func TestMaterializerKeepsCanonicalArtifactIdempotentAfterPublicURLChange(t *testing.T) {
	db, cfg, run := materializerFixture(t, "public_url_change")
	firstStore := &memoryStore{}
	first, created, err := NewMaterializer(db, cfg, firstStore, nil).Materialize(context.Background(), run)
	if err != nil || !created {
		t.Fatalf("first created=%v err=%v artifact=%+v", created, err, first)
	}

	changed := cfg
	changed.PublicBaseURL = "https://new-cdn.example.test/materials"
	secondStore := &memoryStore{}
	second, created, err := NewMaterializer(db, changed, secondStore, nil).Materialize(context.Background(), run)
	if err != nil || created {
		t.Fatalf("second created=%v err=%v artifact=%+v", created, err, second)
	}
	if second.ID != first.ID || second.ObjectKey != first.ObjectKey || second.URL != first.URL {
		t.Fatalf("second artifact=%+v, want existing=%+v", second, first)
	}
	if len(secondStore.keys) != 0 {
		t.Fatalf("URL-only config change uploaded objects: %v", secondStore.keys)
	}

	unavailableURL := changed
	unavailableURL.PublicBaseURL = ""
	unavailableURL.Endpoint = ""
	unavailableURL.Bucket = ""
	thirdStore := &memoryStore{}
	third, created, err := NewMaterializer(db, unavailableURL, thirdStore, nil).Materialize(context.Background(), run)
	if err != nil || created || third.ID != first.ID || len(thirdStore.keys) != 0 {
		t.Fatalf("existing artifact with unavailable URL derivation=%+v created=%v err=%v uploads=%v", third, created, err, thirdStore.keys)
	}
}

func TestMaterializerRecoversCanonicalArtifactAfterObjectKeyUniqueConflict(t *testing.T) {
	db, cfg, run := materializerFixture(t, "object_key_conflict")
	key := "pm/workspace/1/team/10/project/11/workflow_test_cases/instance/40/test-cases/run/60.csv"
	runID := run.ID
	competing := workflowgorm.WorkflowNodeArtifact{
		WorkspaceID: run.WorkspaceID, InstanceID: 40, NodeID: "test-cases", Kind: "ai_generated",
		Name: "legacy-generated-name.csv", ContentType: CSVContentType, Size: 128, ObjectKey: key,
		URL: "https://old-cdn.example.test/generated.csv", AIRunID: &runID, UploaderUserID: 77,
	}
	uploads := 0
	store := objectStoreFunc(func(_ context.Context, uploadedKey string, body io.Reader, _ int64, _ string) error {
		uploads++
		if uploadedKey != key {
			t.Fatalf("uploaded key=%q, want %q", uploadedKey, key)
		}
		if _, err := io.Copy(io.Discard, body); err != nil {
			return err
		}
		return db.Create(&competing).Error
	})

	artifact, created, err := NewMaterializer(db, cfg, store, nil).Materialize(context.Background(), run)
	if err != nil || created {
		t.Fatalf("created=%v err=%v artifact=%+v", created, err, artifact)
	}
	if artifact.ID != competing.ID || artifact.ObjectKey != key || artifact.Name != competing.Name || artifact.URL != competing.URL {
		t.Fatalf("artifact=%+v, want concurrently persisted artifact=%+v", artifact, competing)
	}

	retried, created, err := NewMaterializer(db, cfg, store, nil).Materialize(context.Background(), run)
	if err != nil || created || retried.ID != competing.ID || uploads != 1 {
		t.Fatalf("retry=%+v created=%v err=%v uploads=%d", retried, created, err, uploads)
	}
}

func TestMaterializerFailsClosedWhenObjectKeyBelongsToNoncanonicalArtifact(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*workflowgorm.WorkflowNodeArtifact)
	}{
		{name: "workspace", mutate: func(row *workflowgorm.WorkflowNodeArtifact) { row.WorkspaceID++ }},
		{name: "instance", mutate: func(row *workflowgorm.WorkflowNodeArtifact) { row.InstanceID++ }},
		{name: "node", mutate: func(row *workflowgorm.WorkflowNodeArtifact) { row.NodeID = "other-test-cases" }},
		{name: "run", mutate: func(row *workflowgorm.WorkflowNodeArtifact) {
			otherRunID := uint(61)
			row.AIRunID = &otherRunID
		}},
		{name: "missing_run", mutate: func(row *workflowgorm.WorkflowNodeArtifact) { row.AIRunID = nil }},
		{name: "kind", mutate: func(row *workflowgorm.WorkflowNodeArtifact) { row.Kind = "uploaded" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, cfg, run := materializerFixture(t, "object_key_noncanonical_"+test.name)
			key := "pm/workspace/1/team/10/project/11/workflow_test_cases/instance/40/test-cases/run/60.csv"
			runID := run.ID
			conflict := workflowgorm.WorkflowNodeArtifact{
				WorkspaceID: run.WorkspaceID, InstanceID: 40, NodeID: "test-cases", Kind: "ai_generated",
				Name: "generated.csv", ContentType: CSVContentType, Size: 128, ObjectKey: key,
				URL: "https://cdn.example.test/generated.csv", AIRunID: &runID, UploaderUserID: run.RequestedByUser,
			}
			test.mutate(&conflict)
			if err := db.Create(&conflict).Error; err != nil {
				t.Fatal(err)
			}
			store := &memoryStore{}
			artifact, created, err := NewMaterializer(db, cfg, store, nil).Materialize(context.Background(), run)
			if !errors.Is(err, ErrRunNotEligible) || created || artifact.ID != 0 {
				t.Fatalf("artifact=%+v created=%v err=%v, want fail-closed identity conflict", artifact, created, err)
			}
			if len(store.keys) != 0 {
				t.Fatalf("noncanonical artifact key was overwritten: uploads=%v", store.keys)
			}
		})
	}
}

func TestMaterializerRetriesAfterObjectWriteAndDatabaseCreateFailure(t *testing.T) {
	db, cfg, run := materializerFixture(t, "db_retry")
	store := &memoryStore{}
	materializer := NewMaterializer(db, cfg, store, nil)
	var fail atomic.Bool
	fail.Store(true)
	callbackName := "qatestcase:fail_first_artifact_create"
	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "WorkflowNodeArtifact" && fail.CompareAndSwap(true, false) {
			tx.AddError(errors.New("temporary database failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := materializer.Materialize(context.Background(), run); err == nil {
		t.Fatal("expected first database create failure")
	}
	if err := db.Callback().Create().Remove(callbackName); err != nil {
		t.Fatal(err)
	}
	artifact, created, err := materializer.Materialize(context.Background(), run)
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if len(store.keys) != 2 || store.keys[0] != store.keys[1] || len(store.objects) != 1 {
		t.Fatalf("keys=%v objects=%d artifact=%+v", store.keys, len(store.objects), artifact)
	}
}

func TestMaterializerDoesNotTreatWrongKeyArtifactAsCanonical(t *testing.T) {
	db, cfg, run := materializerFixture(t, "wrong_key")
	runID := run.ID
	legacy := workflowgorm.WorkflowNodeArtifact{
		WorkspaceID: run.WorkspaceID, InstanceID: 40, NodeID: "test-cases", Kind: "ai_generated",
		Name: canonicalArtifactName(40), ContentType: CSVContentType, Size: 128,
		ObjectKey: "pm/workspace/1/team/10/project/11/workflow_test_cases/legacy-upload.csv",
		URL:       "https://cdn.example.test/legacy-upload.csv", AIRunID: &runID, UploaderUserID: run.RequestedByUser,
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	store := &memoryStore{}
	artifact, created, err := NewMaterializer(db, cfg, store, nil).Materialize(context.Background(), run)
	if err != nil || !created {
		t.Fatalf("created=%v err=%v artifact=%+v", created, err, artifact)
	}
	wantKey := "pm/workspace/1/team/10/project/11/workflow_test_cases/instance/40/test-cases/run/60.csv"
	if artifact.ObjectKey != wantKey || len(store.keys) != 1 || store.keys[0] != wantKey {
		t.Fatalf("artifact=%+v uploads=%v", artifact, store.keys)
	}
	var count int64
	if err := db.Model(&workflowgorm.WorkflowNodeArtifact{}).Where("ai_run_id = ?", run.ID).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("artifact count=%d err=%v", count, err)
	}
}

func TestMaterializerRejectsStaleRequirementBeforeUpload(t *testing.T) {
	db, cfg, run := materializerFixture(t, "stale_requirement")
	requirement := requirementgorm.Requirement{
		ID: 21, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, Title: "结算", Version: 2,
	}
	if err := db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&workflowgorm.WorkflowInstance{}).Where("id = ?", 40).Updates(map[string]any{
		"subject_type": "requirement", "subject_id": requirement.ID,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&aigorm.AIRun{}).Where("id = ?", run.ID).Updates(map[string]any{
		"subject_type": "requirement", "subject_id": requirement.ID, "subject_version": 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	run.SubjectType, run.SubjectID, run.SubjectVersion = "requirement", requirement.ID, 1
	store := &memoryStore{}
	if _, _, err := NewMaterializer(db, cfg, store, nil).Materialize(context.Background(), run); !errors.Is(err, ErrSubjectStale) {
		t.Fatalf("error=%v, want ErrSubjectStale", err)
	}
	if len(store.keys) != 0 {
		t.Fatalf("stale run uploaded objects: %v", store.keys)
	}
}

func TestMaterializerIgnoresRequirementTasksForSingleRequirementDelivery(t *testing.T) {
	db, cfg, run := materializerFixture(t, "requirement_ignores_tasks")
	requirement := requirementgorm.Requirement{
		ID: 21, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, Title: "结算", Version: 12,
	}
	if err := db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	task := taskgorm.WorkTask{
		ID: 22, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 20,
		RequirementID: requirement.ID, Kind: taskgorm.WorkTaskKindTask, TaskKey: "PAY-1", Title: "拆解任务",
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&workflowgorm.WorkflowInstance{}).Where("id = ?", 40).Updates(map[string]any{
		"subject_type": "requirement", "subject_id": requirement.ID,
	}).Error; err != nil {
		t.Fatal(err)
	}
	run.SubjectType, run.SubjectID, run.SubjectVersion = "requirement", requirement.ID, requirement.Version
	run.InputJSON = fmt.Sprintf(
		`{"subject":{"id":%d,"version":%d},"test_case_context":{"mode":"single_requirement_document","requirements":[{"id":%d,"version":%d}],"tasks":[]},"test_case_delivery":{"instance_id":40,"node_id":"test-cases"}}`,
		requirement.ID, requirement.Version, requirement.ID, requirement.Version,
	)
	if err := db.Model(&aigorm.AIRun{}).Where("id = ?", run.ID).Updates(map[string]any{
		"subject_type": run.SubjectType, "subject_id": run.SubjectID, "subject_version": run.SubjectVersion, "input_json": run.InputJSON,
	}).Error; err != nil {
		t.Fatal(err)
	}
	store := &memoryStore{}
	artifact, created, err := NewMaterializer(db, cfg, store, nil).Materialize(context.Background(), run)
	if err != nil || !created || artifact.ID == 0 {
		t.Fatalf("created=%v err=%v artifact=%+v", created, err, artifact)
	}
	if len(store.keys) != 1 {
		t.Fatalf("uploaded objects=%v, want one requirement delivery", store.keys)
	}
}

func TestMaterializerRejectsChangedPlanUnitTaskSnapshotBeforeUpload(t *testing.T) {
	db, cfg, run := materializerFixture(t, "stale_plan_context")
	var unit projectgorm.PlanUnit
	if err := db.First(&unit, 20).Error; err != nil {
		t.Fatal(err)
	}
	requirement := requirementgorm.Requirement{
		ID: 21, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, PlanUnitID: unit.ID,
		Title: "结算", Version: 3,
	}
	if err := db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	task := taskgorm.WorkTask{
		ID: 22, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: unit.ID,
		RequirementID: requirement.ID, Kind: taskgorm.WorkTaskKindTask, TaskKey: "PAY-1", Title: "校验结算",
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	run.InputJSON = fmt.Sprintf(
		`{"subject":{"updated_at":%q},"test_case_context":{"requirements":[{"id":%d,"version":%d}],"tasks":[{"id":%d,"updated_at":%q}]},"test_case_delivery":{"instance_id":40,"node_id":"test-cases"}}`,
		unit.UpdatedAt.UTC().Format(time.RFC3339Nano), requirement.ID, requirement.Version,
		task.ID, task.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err := db.Model(&aigorm.AIRun{}).Where("id = ?", run.ID).Update("input_json", run.InputJSON).Error; err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := db.Model(&taskgorm.WorkTask{}).Where("id = ?", task.ID).Update("title", "结算规则已变更").Error; err != nil {
		t.Fatal(err)
	}
	store := &memoryStore{}
	if _, _, err := NewMaterializer(db, cfg, store, nil).Materialize(context.Background(), run); !errors.Is(err, ErrSubjectStale) {
		t.Fatalf("error=%v, want ErrSubjectStale", err)
	}
	if len(store.keys) != 0 {
		t.Fatalf("stale plan-unit run uploaded objects: %v", store.keys)
	}
}

func TestMaterializerRejectsSubjectChangeAfterPostUploadCheckBeforeArtifactCreate(t *testing.T) {
	db, cfg, run := materializerFixture(t, "stale_before_artifact_create")
	var unit projectgorm.PlanUnit
	if err := db.First(&unit, 20).Error; err != nil {
		t.Fatal(err)
	}
	requirement := requirementgorm.Requirement{
		ID: 21, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, PlanUnitID: unit.ID,
		Title: "结算", Version: 3,
	}
	if err := db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	task := taskgorm.WorkTask{
		ID: 22, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: unit.ID,
		RequirementID: requirement.ID, Kind: taskgorm.WorkTaskKindTask, TaskKey: "PAY-1", Title: "校验结算",
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	run.InputJSON = fmt.Sprintf(
		`{"subject":{"updated_at":%q},"test_case_context":{"requirements":[{"id":%d,"version":%d}],"tasks":[{"id":%d,"updated_at":%q}]},"test_case_delivery":{"instance_id":40,"node_id":"test-cases"}}`,
		unit.UpdatedAt.UTC().Format(time.RFC3339Nano), requirement.ID, requirement.Version,
		task.ID, task.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err := db.Model(&aigorm.AIRun{}).Where("id = ?", run.ID).Update("input_json", run.InputJSON).Error; err != nil {
		t.Fatal(err)
	}

	var taskReads atomic.Int32
	var mutationErr error
	callbackName := "qatestcase:mutate_after_post_upload_freshness"
	if err := db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "WorkTask" || taskReads.Add(1) != 2 {
			return
		}
		time.Sleep(2 * time.Millisecond)
		mutationErr = db.Session(&gorm.Session{SkipHooks: true}).Model(&taskgorm.WorkTask{}).
			Where("id = ? AND workspace_id = ?", task.ID, task.WorkspaceID).
			Updates(map[string]any{
				"title":      "结算规则在持久化前变更",
				"updated_at": time.Now().UTC(),
			}).Error
		if mutationErr != nil {
			tx.AddError(mutationErr)
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Callback().Query().Remove(callbackName); err != nil {
			t.Errorf("remove query callback: %v", err)
		}
	}()

	store := &memoryStore{}
	if _, _, err := NewMaterializer(db, cfg, store, nil).Materialize(context.Background(), run); !errors.Is(err, ErrSubjectStale) {
		t.Fatalf("error=%v, want ErrSubjectStale", err)
	}
	if mutationErr != nil {
		t.Fatalf("mutate task after post-upload freshness check: %v", mutationErr)
	}
	if taskReads.Load() < 3 {
		t.Fatalf("task freshness reads=%d, want initial, post-upload, and transactional checks", taskReads.Load())
	}
	if len(store.keys) != 1 {
		t.Fatalf("uploaded objects=%v, want one deterministic unreferenced object", store.keys)
	}
	var count int64
	if err := db.Model(&workflowgorm.WorkflowNodeArtifact{}).Where("ai_run_id = ?", run.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("artifact count=%d err=%v, want no stale artifact", count, err)
	}
}

func TestMaterializerConcurrentRequestsPersistOneArtifact(t *testing.T) {
	db, cfg, run := materializerFixture(t, "concurrent")
	store := &memoryStore{}
	materializer := NewMaterializer(db, cfg, store, nil)
	const callers = 8
	results := make(chan error, callers)
	var wait sync.WaitGroup
	for index := 0; index < callers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, _, err := materializer.Materialize(context.Background(), run)
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent materialize error=%v", err)
		}
	}
	var count int64
	if err := db.Model(&workflowgorm.WorkflowNodeArtifact{}).Where("ai_run_id = ?", run.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("artifact count=%d err=%v", count, err)
	}
	if len(store.objects) != 1 {
		t.Fatalf("stored objects=%d keys=%v", len(store.objects), store.keys)
	}
}

func TestMaterializerHonorsTargetBoundBeforeLaterBranchDecision(t *testing.T) {
	db, cfg, run := materializerFixture(t, "branch_changed")
	definition := `{"nodes":[{"id":"start","type":"start","name":"开始"},{"id":"choice","type":"condition","name":"是否需要测试"},{"id":"test-cases","type":"human_task","name":"测试用例","config":{"test_case_artifact":true}},{"id":"manual","type":"human_task","name":"人工处理"},{"id":"end","type":"end","name":"结束"}],"edges":[{"id":"e1","source":"start","target":"choice"},{"id":"e2","source":"choice","target":"test-cases","kind":"yes"},{"id":"e3","source":"choice","target":"manual","kind":"no"},{"id":"e4","source":"test-cases","target":"end"},{"id":"e5","source":"manual","target":"end"}]}`
	if err := db.Model(&workflowgorm.WorkflowTemplateVersion{}).Where("id = ? AND workspace_id = ?", 30, 1).Update("definition", definition).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&workflowgorm.WorkflowNodeRun{
		WorkspaceID: 1, InstanceID: 40, NodeID: "choice", NodeType: "condition", Status: "completed",
		OutputJSON: `{"decision":false}`, StartedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	store := &memoryStore{}
	artifact, created, err := NewMaterializer(db, cfg, store, nil).Materialize(context.Background(), run)
	if err != nil || !created || artifact.NodeID != "test-cases" || len(store.keys) != 1 {
		t.Fatalf("artifact=%+v created=%v error=%v keys=%v", artifact, created, err, store.keys)
	}
}

func materializerFixture(t *testing.T, suffix string) (*gorm.DB, config.ObjectStorage, aigorm.AIRun) {
	t.Helper()
	db, err := database.Open(config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN:    fmt.Sprintf("file:qatestcase_%s_%d?mode=memory&cache=shared", suffix, time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	definition := `{"nodes":[{"id":"start","type":"start","name":"开始"},{"id":"test-cases","type":"human_task","name":"测试用例","config":{"test_case_artifact":true}},{"id":"end","type":"end","name":"结束"}],"edges":[{"id":"e1","source":"start","target":"test-cases"},{"id":"e2","source":"test-cases","target":"end"}]}`
	project := projectgorm.ProjectSet{ID: 11, WorkspaceID: 1, TeamID: 10, Name: "PM"}
	unit := projectgorm.PlanUnit{ID: 20, WorkspaceID: 1, TeamID: 10, ProjectSetID: project.ID, Title: "V1"}
	version := workflowgorm.WorkflowTemplateVersion{ID: 30, WorkspaceID: 1, TemplateID: 31, Version: 1, Definition: definition, DefinitionHash: strings.Repeat("a", 64), PublishedByUser: 1, PublishedAt: time.Now().UTC()}
	instance := workflowgorm.WorkflowInstance{ID: 40, WorkspaceID: 1, TemplateID: 31, TemplateVersionID: version.ID, SubjectType: "plan_unit", SubjectID: unit.ID, Status: "running", CurrentNodeIDs: `["test-cases"]`, NodePlansJSON: `{}`, StartedByUserID: 9, StartedAt: time.Now().UTC()}
	for _, row := range []any{&project, &unit, &version, &instance} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	output := validOutputJSON(`{"caseId":"TC-MAIN-001","title":"主流程","category":"positive","priority":"P0","preconditions":[],"steps":["提交"],"expectedResults":["显示成功"],"evidence":["subject.title"]}`, `{"smokeCaseIds":["TC-MAIN-001"],"executionOrder":["TC-MAIN-001"],"automationCandidateCaseIds":[],"releaseBlockingCaseIds":["TC-MAIN-001"]}`)
	run := aigorm.AIRun{ID: 60, WorkspaceID: 1, Capability: "qa.test_cases", SubjectType: "plan_unit", SubjectID: unit.ID, SubjectVersion: 1, Status: "completed", Model: "test", InputJSON: `{"subject":{},"test_case_delivery":{"instance_id":40,"node_id":"test-cases"}}`, OutputJSON: output, RequestedByUser: 9}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	cfg := config.ObjectStorage{Enabled: true, Endpoint: "cos.example.test", Bucket: "pm", AccessKey: "key", SecretKey: "secret", UseSSL: true, GlobalPrefix: "pm", TeamPrefix: true, PublicBaseURL: "https://cdn.example.test"}
	return db, cfg, run
}
