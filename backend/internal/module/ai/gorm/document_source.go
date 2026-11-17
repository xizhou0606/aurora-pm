package aigorm

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai/docsource"
	"gorm.io/gorm"
)

// SaveDocumentSources persists identities only. Bodies never enter source rows.
func SaveDocumentSources(tx *gorm.DB, run AIRun, m *docsource.Manifest) error {
	for _, ref := range m.Sources {
		row := AIRunSource{WorkspaceID: run.WorkspaceID, RunID: run.ID, SourceType: ref.Kind, SourceURL: ref.URL}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
