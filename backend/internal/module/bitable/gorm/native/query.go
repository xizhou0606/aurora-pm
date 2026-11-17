package native

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
)

func loadQuery(tx *gorm.DB, s bitable.Scope, q bitable.Query, all bool) (TableRecord, []bitable.Field, []bitable.Record, error) {
	table, fields, views, e := loadTable(tx, s, false)
	if e != nil {
		return table, nil, nil, e
	}
	var view *bitable.View
	for i := range views {
		if views[i].ID == q.ViewID {
			view = &views[i]
			break
		}
	}
	if view == nil {
		return table, nil, nil, bitable.ErrNotFound
	}
	filters, sorts := q.Filters, q.Sorts
	if filters == nil {
		filters = view.Config.Filters
	}
	if sorts == nil {
		sorts = view.Config.Sorts
	}
	if all {
		filters = []bitable.Filter{}
		sorts = []bitable.Sort{}
	}
	byID := map[string]bitable.Field{}
	for _, f := range fields {
		byID[f.ID] = f
	}
	for _, filter := range filters {
		if _, ok := byID[filter.FieldID]; !ok {
			return table, nil, nil, bitable.ErrInvalidInput
		}
	}
	for _, order := range sorts {
		if _, ok := byID[order.FieldID]; !ok {
			return table, nil, nil, bitable.ErrInvalidInput
		}
	}
	var stored []RowRecord
	if e = tableBudget(tx, s, len(fields)); e != nil {
		return table, nil, nil, e
	}
	if e = scopeDB(tx, s).Where("deleted = ?", false).Order("sort_order ASC, id ASC").Limit(bitable.MaxExportRows + 1).Find(&stored).Error; e != nil {
		return table, nil, nil, e
	}
	if len(stored) > bitable.MaxExportRows || len(stored)*len(fields) > bitable.MaxExportCells {
		return table, nil, nil, bitable.ErrBudgetExceeded
	}
	result := []bitable.Record{}
	for _, row := range stored {
		record, err := rowDTO(row, fields)
		if err != nil {
			return table, nil, nil, err
		}
		keep := true
		for _, filter := range filters {
			if !matches(byID[filter.FieldID], record.Values[filter.FieldID], filter) {
				keep = false
				break
			}
		}
		if keep {
			result = append(result, record)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		for _, order := range sorts {
			c := compareValue(byID[order.FieldID], result[i].Values[order.FieldID], result[j].Values[order.FieldID])
			if c != 0 {
				if order.Direction == "desc" {
					return c > 0
				}
				return c < 0
			}
		}
		return false
	})
	return table, fields, result, nil
}
func (r *Repository) Query(ctx context.Context, s bitable.Scope, q bitable.Query) (bitable.RecordPage, error) {
	result := bitable.RecordPage{Records: []bitable.Record{}, Offset: q.Offset}
	if e := bitable.ValidateQuery(q); e != nil {
		return result, e
	}
	e := r.transaction(ctx, true, func(tx *gorm.DB) error {
		_, _, records, err := loadQuery(tx, s, q, false)
		if err != nil {
			return err
		}
		result.Total = len(records)
		if q.LocateRecordID != "" {
			index := -1
			for i, row := range records {
				if row.ID == q.LocateRecordID {
					index = i
					break
				}
			}
			if index < 0 {
				return bitable.ErrRecordFilteredOut
			}
			q.Offset = (index / q.Limit) * q.Limit
			result.Offset = q.Offset
		}
		if q.Offset < len(records) {
			end := q.Offset + q.Limit
			if end > len(records) {
				end = len(records)
			}
			result.Records = records[q.Offset:end]
		}
		return nil
	})
	return result, e
}
func (r *Repository) Search(ctx context.Context, s bitable.Scope, q bitable.Search) (bitable.SearchPage, error) {
	result := bitable.SearchPage{Matches: []bitable.SearchMatch{}}
	if e := bitable.ValidateSearch(q); e != nil {
		return result, e
	}
	e := r.transaction(ctx, true, func(tx *gorm.DB) error {
		_, fields, records, err := loadQuery(tx, s, q.Query, false)
		if err != nil {
			return err
		}
		needle := strings.ToLower(q.Text)
		for index, row := range records {
			for _, field := range fields {
				if strings.Contains(strings.ToLower(textValue(field, row.Values[field.ID])), needle) {
					result.Matches = append(result.Matches, bitable.SearchMatch{RecordID: row.ID, FieldID: field.ID, Ordinal: index})
				}
			}
		}
		result.Total = len(result.Matches)
		return nil
	})
	return result, e
}
func (r *Repository) Export(ctx context.Context, s bitable.Scope, q bitable.ExportQuery) (bitable.ExportSnapshot, error) {
	result := bitable.ExportSnapshot{Fields: []bitable.Field{}, Records: []bitable.Record{}}
	if e := bitable.ValidateQuery(q.Query); e != nil {
		return result, e
	}
	e := r.transaction(ctx, true, func(tx *gorm.DB) error {
		table, fields, records, err := loadQuery(tx, s, q.Query, q.All)
		if err != nil {
			return err
		}
		if !q.All {
			var views []bitable.View
			if err = json.Unmarshal([]byte(table.Views), &views); err != nil {
				return err
			}
			for _, v := range views {
				if v.ID == q.ViewID {
					hidden := map[string]bool{}
					for _, id := range v.Config.HiddenFieldIDs {
						hidden[id] = true
					}
					selected := []bitable.Field{}
					byID := map[string]bitable.Field{}
					for _, f := range fields {
						byID[f.ID] = f
					}
					for _, id := range v.Config.ColumnOrder {
						if f, ok := byID[id]; ok && !hidden[id] {
							selected = append(selected, f)
							hidden[id] = true
						}
					}
					for _, f := range fields {
						if !hidden[f.ID] {
							selected = append(selected, f)
						}
					}
					fields = selected
				}
			}
		}
		visible := map[string]bool{}
		for _, f := range fields {
			visible[f.ID] = true
		}
		for i := range records {
			for id := range records[i].Values {
				if !visible[id] {
					delete(records[i].Values, id)
				}
			}
		}
		result = bitable.ExportSnapshot{SnapshotID: fmt.Sprintf("%s:%d:%d", table.ID, table.SchemaRevision, table.DataRevision), SchemaRevision: table.SchemaRevision, Fields: fields, Records: records}
		return nil
	})
	return result, e
}
