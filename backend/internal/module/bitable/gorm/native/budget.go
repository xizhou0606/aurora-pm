package native

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
)

func tableBudget(tx *gorm.DB, s bitable.Scope, fields int) error {
	stats := struct {
		Rows  int64
		Bytes int64
	}{}
	length := `LENGTH(CAST("values" AS BLOB))`
	if tx.Dialector.Name() == "postgres" {
		length = `OCTET_LENGTH(CAST("values" AS TEXT))`
	}
	if e := scopeDB(tx.Model(&RowRecord{}), s).Where("deleted = ?", false).Select(`COUNT(*) AS rows, COALESCE(SUM(` + length + `),0) AS bytes`).Scan(&stats).Error; e != nil {
		return e
	}
	if stats.Rows > bitable.MaxExportRows || stats.Rows*int64(fields) > bitable.MaxExportCells || stats.Bytes > bitable.MaxTableValueBytes {
		return bitable.ErrBudgetExceeded
	}
	return nil
}
