package aigorm

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
)

// ReportExportRepository persists the Feishu document mapping of reporting
// runs on the ai_runs row itself. The claim is a conditional update on the
// still-empty mapping columns, so only the first publishing request wins and
// every retry reuses its document.
type ReportExportRepository struct {
	db *gorm.DB
}

func NewReportExportRepository(db *gorm.DB) *ReportExportRepository {
	return &ReportExportRepository{db: db}
}

func (repository *ReportExportRepository) FindReportExport(
	ctx context.Context,
	workspaceID uint,
	runID uint,
) (ai.ReportExportRecord, bool, error) {
	if repository == nil || repository.db == nil {
		return ai.ReportExportRecord{}, false, ai.ErrReportRunUnavailable
	}
	var run AIRun
	err := repository.db.WithContext(ctx).
		Select("export_document_id", "export_document_url").
		Where("workspace_id = ? AND id = ? AND capability LIKE ? AND export_document_id <> ''",
			workspaceID, runID, ai.ReportCapabilityPrefix+"%").
		Take(&run).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ai.ReportExportRecord{}, false, nil
		}
		return ai.ReportExportRecord{}, false, err
	}
	return ai.ReportExportRecord{DocumentID: run.ExportDocumentID, URL: run.ExportDocumentURL}, true, nil
}

func (repository *ReportExportRepository) ClaimReportExport(
	ctx context.Context,
	workspaceID uint,
	runID uint,
	record ai.ReportExportRecord,
) (bool, error) {
	if repository == nil || repository.db == nil {
		return false, ai.ErrReportRunUnavailable
	}
	result := repository.db.WithContext(ctx).Model(&AIRun{}).
		Where("workspace_id = ? AND id = ? AND capability LIKE ? AND export_document_id = ''",
			workspaceID, runID, ai.ReportCapabilityPrefix+"%").
		Updates(map[string]any{
			"export_document_id":  record.DocumentID,
			"export_document_url": record.URL,
			"exported_at":         time.Now(),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

var _ ai.ReportExportStore = (*ReportExportRepository)(nil)
