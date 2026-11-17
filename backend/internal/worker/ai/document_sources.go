package aiworker

import (
	"context"
	"encoding/json"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai/docsource"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	"gorm.io/gorm"
)

func (w *Worker) ConfigureDocumentSources(factory func(*gorm.DB) (*docsource.Service, error)) {
	w.documentSources = factory
}
func (w *Worker) attachDocumentSources(ctx context.Context, run *aigorm.AIRun, input map[string]any, m *docsource.Manifest) error {
	if w.documentSources == nil || m.Context.WorkspaceID != run.WorkspaceID {
		return docsource.ErrUnavailable
	}
	service, err := w.documentSources(w.db.WithContext(ctx))
	if err != nil {
		return err
	}
	snapshots, err := service.Read(ctx, m)
	if err != nil {
		return err
	}
	var rows []aigorm.AIRunSource
	if err = w.db.WithContext(ctx).Where("workspace_id = ? AND run_id = ?", run.WorkspaceID, run.ID).Order("id").Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) != len(snapshots) {
		return docsource.ErrUnavailable
	}
	for i, snapshot := range snapshots {
		if rows[i].SourceType != m.Sources[i].Kind || rows[i].SourceURL != m.Sources[i].URL {
			return docsource.ErrUnavailable
		}
		refs, _ := json.Marshal(snapshot.BlockRefs)
		if err = w.db.WithContext(ctx).Model(&aigorm.AIRunSource{}).Where("workspace_id = ? AND run_id = ? AND id = ?", run.WorkspaceID, run.ID, rows[i].ID).Updates(map[string]any{"revision": snapshot.Revision, "content_hash": snapshot.ContentHash, "block_refs": string(refs)}).Error; err != nil {
			return err
		}
	}
	input["sources"] = snapshots
	// This metadata is trusted server state and must not enter the model request.
	delete(input, docsource.ManifestKey)
	return nil
}
func (w *Worker) verifyDocumentSources(ctx context.Context, db *gorm.DB, run *aigorm.AIRun, input map[string]any, internalOnly bool) error {
	var original map[string]any
	if json.Unmarshal([]byte(run.InputJSON), &original) != nil {
		return docsource.ErrUnavailable
	}
	m, err := docsource.DecodeManifest(original)
	if err != nil {
		return err
	}
	if m == nil {
		return nil
	}
	if w.documentSources == nil || m.Context.WorkspaceID != run.WorkspaceID {
		return docsource.ErrUnavailable
	}
	service, err := w.documentSources(db.WithContext(ctx))
	if err != nil {
		return err
	}
	raw, err := json.Marshal(input["sources"])
	if err != nil {
		return err
	}
	var snapshots []docsource.Snapshot
	if json.Unmarshal(raw, &snapshots) != nil {
		return docsource.ErrUnavailable
	}
	return service.Verify(ctx, m, snapshots, internalOnly)
}

// Legacy runs never infer a principal from RequestedByUser, which stores fact IDs.
func (w *Worker) rejectUnregisteredInternalSources(ctx context.Context, run *aigorm.AIRun, input map[string]any) error {
	if w.documentSources == nil || !docsource.Supported(run.Capability) {
		return nil
	}
	service, err := w.documentSources(w.db.WithContext(ctx))
	if err != nil {
		return err
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	var value struct {
		Subject         struct{ ID uint }
		TestCaseContext struct{ Requirements []struct{ ID uint } } `json:"test_case_context"`
	}
	if json.Unmarshal(raw, &value) != nil {
		return docsource.ErrUnavailable
	}
	ids := []uint{}
	if run.Capability == "qa.test_cases" {
		for _, r := range value.TestCaseContext.Requirements {
			ids = append(ids, r.ID)
		}
	} else {
		ids = append(ids, run.SubjectID)
	}
	for _, id := range ids {
		source, err := service.Requirements.ReadDocumentSource(ctx, run.WorkspaceID, id)
		if err != nil {
			return docsource.ErrUnavailable
		}
		if len(source.ResourceIDs) > 0 {
			return docsource.ErrUnavailable
		}
	}
	return nil
}
