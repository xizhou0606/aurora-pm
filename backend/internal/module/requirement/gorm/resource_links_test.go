package requirementgorm

import (
	"context"
	"errors"
	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"gorm.io/gorm"
	"testing"
)

type linkTestAccess struct {
	tx   *gorm.DB
	fail bool
}

func (a linkTestAccess) LockResource(context.Context, uint, string) error { return nil }

func (a linkTestAccess) RequireRequirement(_ context.Context, w, u, p uint, _ bool) error {
	if w != 7 || u != 3 || p == 0 {
		return requirement.ErrResourceLinkNotFound
	}
	return nil
}
func (a linkTestAccess) Resource(_ context.Context, w, u uint, id string, _ bool) (requirement.ResourceLink, error) {
	return requirement.ResourceLink{ResourceID: id, CanManage: true, PolicyRevision: 1}, nil
}
func (a linkTestAccess) Candidates(context.Context, uint, uint) ([]requirement.ResourceLink, error) {
	return []requirement.ResourceLink{}, nil
}
func (a linkTestAccess) ApplyResourcePolicy(_ context.Context, c requirement.ResourceLinkCommand) error {
	if err := a.tx.Create(&mutationTestEffect{WorkspaceID: c.WorkspaceID, Kind: "policy"}).Error; err != nil {
		return err
	}
	if a.fail {
		return requirement.ErrResourceLinkConflict
	}
	return nil
}
func TestResourceLinkTransactionReplayIsolationAndDynamicProject(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err = db.AutoMigrate(&Requirement{}, &ResourceLinkRecord{}, &ToolMutationApply{}, &mutationTestEffect{}); err != nil {
		t.Fatal(err)
	}
	row := Requirement{ID: 11, WorkspaceID: 7, ProjectSetID: 19, Version: 1, Title: "test", ProductDocURL: "https://example.feishu.cn/docx/original"}
	if err = db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	fail := true
	repo := NewResourceLinkRepository(db, func(tx *gorm.DB) (requirement.ResourceLinkAccess, error) {
		return linkTestAccess{tx: tx, fail: fail}, nil
	})
	cmd := requirement.ResourceLinkCommand{WorkspaceID: 7, UserID: 3, RequirementID: 11, ResourceID: "doc", ExpectedVersion: 1, ExpectedPolicyRevision: 1, OperationID: "attach"}
	ctx := context.Background()
	if _, err = repo.ApplyResourceLink(ctx, cmd); !errors.Is(err, requirement.ErrResourceLinkConflict) {
		t.Fatalf("rollback: %v", err)
	}
	var count int64
	db.Model(&ResourceLinkRecord{}).Count(&count)
	if count != 0 {
		t.Fatal("link escaped rollback")
	}
	db.Model(&mutationTestEffect{}).Count(&count)
	if count != 0 {
		t.Fatal("policy escaped rollback")
	}
	fail = false
	var unchanged Requirement
	db.First(&unchanged, 11)
	if unchanged.ProductDocURL != row.ProductDocURL {
		t.Fatal("failed attach cleared original source")
	}
	page, err := repo.ApplyResourceLink(ctx, cmd)
	if err != nil || page.RequirementVersion != 2 || len(page.Items) != 1 {
		t.Fatalf("attach: %+v %v", page, err)
	}
	if page.ProductDocURL != "" {
		t.Fatal("internal and external sources coexist")
	}
	second := cmd
	second.ResourceID, second.OperationID, second.ExpectedVersion = "second", "second", 2
	if _, err = repo.ApplyResourceLink(ctx, second); !errors.Is(err, requirement.ErrResourceLinkConflict) {
		t.Fatalf("second source accepted: %v", err)
	}
	if _, err = repo.ApplyResourceLink(ctx, cmd); err != nil {
		t.Fatalf("replay: %v", err)
	}
	db.Model(&mutationTestEffect{}).Count(&count)
	if count != 1 {
		t.Fatal("replay duplicated policy event")
	}
	cmd.ResourceID = "other"
	if _, err = repo.ApplyResourceLink(ctx, cmd); !errors.Is(err, requirement.ErrResourceLinkConflict) {
		t.Fatalf("changed replay: %v", err)
	}
	cmd.ResourceID = "doc"
	sets, err := repo.ResourceLinkProjectSets(ctx, 7, []string{"doc"})
	if err != nil || len(sets["doc"]) != 1 || sets["doc"][0] != 19 {
		t.Fatalf("sets: %v %v", sets, err)
	}
	if err = db.Model(&Requirement{}).Where("id = ?", 11).Update("project_new_id", 23).Error; err != nil {
		t.Fatal(err)
	}
	sets, err = repo.ResourceLinkProjectSets(ctx, 7, []string{"doc"})
	if err != nil || sets["doc"][0] != 23 {
		t.Fatalf("project not dynamic: %v %v", sets, err)
	}
	sets, err = repo.ResourceLinkProjectSets(ctx, 8, []string{"doc"})
	if err != nil || len(sets) != 0 {
		t.Fatal("workspace leak")
	}
	cmd.WorkspaceID = 8
	if _, err = repo.ApplyResourceLink(ctx, cmd); !errors.Is(err, requirement.ErrResourceLinkNotFound) {
		t.Fatalf("foreign write: %v", err)
	}
	cmd.WorkspaceID = 7
	cmd.Remove = true
	cmd.ExpectedVersion = 2
	cmd.OperationID = "detach"
	newURL := "https://example.feishu.cn/docx/new"
	cmd.ProductDocURL = &newURL
	fail = true
	if _, err = repo.ApplyResourceLink(ctx, cmd); !errors.Is(err, requirement.ErrResourceLinkConflict) {
		t.Fatalf("failed replacement: %v", err)
	}
	unchangedPage, _ := repo.ReadResourceLinks(ctx, 7, 3, 11, false)
	if len(unchangedPage.Items) != 1 || unchangedPage.ProductDocURL != "" {
		t.Fatal("failed replacement lost original source")
	}
	fail = false
	page, err = repo.ApplyResourceLink(ctx, cmd)
	if err != nil || page.RequirementVersion != 3 || len(page.Items) != 0 {
		t.Fatalf("detach: %+v %v", page, err)
	}
	if page.ProductDocURL != newURL {
		t.Fatal("external source not saved atomically")
	}
	sets, err = repo.ResourceLinkProjectSets(ctx, 7, []string{"doc"})
	if err != nil || len(sets) != 0 {
		t.Fatal("detached grant survived")
	}
}
