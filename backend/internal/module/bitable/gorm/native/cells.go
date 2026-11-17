package native

import (
	"encoding/json"
	"fmt"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"sort"
	"strconv"
	"strings"
	"time"
)

func rowDTO(row RowRecord, fields []bitable.Field) (bitable.Record, error) {
	values, revisions, e := decodeRowState(row, fields)
	if e != nil {
		return bitable.Record{}, e
	}
	for _, field := range fields {
		if _, ok := values[field.ID]; !ok {
			values[field.ID] = nil
		}
		switch field.Type {
		case bitable.FieldCreatedBy:
			values[field.ID] = []bitable.UserRef{{ID: row.CreatedBy, Name: strconv.FormatUint(uint64(row.CreatedBy), 10)}}
		case bitable.FieldLastModifiedBy:
			values[field.ID] = []bitable.UserRef{{ID: row.UpdatedBy, Name: strconv.FormatUint(uint64(row.UpdatedBy), 10)}}
		case bitable.FieldCreatedTime:
			values[field.ID] = row.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z")
		case bitable.FieldLastModifiedTime:
			values[field.ID] = row.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z")
		}
	}
	return bitable.Record{ID: row.ID, Revision: row.Revision, FieldRevisions: revisions, Values: values}, nil
}
func textValue(field bitable.Field, value any) string {
	if value == nil {
		return ""
	}
	if value, ok := value.(string); ok {
		if field.Type == bitable.FieldSingleSelect {
			for _, c := range field.Options.Choices {
				if c.ID == value {
					return c.Name
				}
			}
		}
		return value
	}
	if entries, ok := value.([]any); ok {
		result := []string{}
		for _, entry := range entries {
			if object, ok := entry.(map[string]any); ok {
				if name, ok := object["name"].(string); ok {
					result = append(result, name)
					continue
				}
			}
			if id, ok := entry.(string); ok {
				label := id
				for _, c := range field.Options.Choices {
					if c.ID == id {
						label = c.Name
					}
				}
				result = append(result, label)
				continue
			}
			result = append(result, fmt.Sprint(entry))
		}
		return strings.Join(result, ", ")
	}
	data, e := json.Marshal(value)
	if e != nil {
		return ""
	}
	return string(data)
}
func equalValue(f bitable.Field, a, b any) bool {
	if f.Type == bitable.FieldUser || f.Type == bitable.FieldCreatedBy || f.Type == bitable.FieldLastModifiedBy {
		if a == nil || b == nil {
			return a == nil && b == nil
		}
		ids := func(v any) ([]uint, error) {
			data, e := json.Marshal(v)
			if e != nil {
				return nil, e
			}
			var users []bitable.UserRef
			if e = json.Unmarshal(data, &users); e != nil {
				return nil, e
			}
			result := []uint{}
			for _, u := range users {
				result = append(result, u.ID)
			}
			sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
			return result, nil
		}
		left, le := ids(a)
		right, re := ids(b)
		if le != nil || re != nil || len(left) != len(right) {
			return false
		}
		for i := range left {
			if left[i] != right[i] {
				return false
			}
		}
		return true
	}
	left, e := json.Marshal(a)
	if e != nil {
		return false
	}
	right, e := json.Marshal(b)
	return e == nil && string(left) == string(right)
}
func compareValue(f bitable.Field, a, b any) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	if left, ok := a.(float64); ok {
		if right, ok := b.(float64); ok {
			if left < right {
				return -1
			}
			if left > right {
				return 1
			}
			return 0
		}
	}
	if f.Type == bitable.FieldDate || f.Type == bitable.FieldCreatedTime || f.Type == bitable.FieldLastModifiedTime {
		left, le := time.Parse(time.RFC3339Nano, fmt.Sprint(a))
		right, re := time.Parse(time.RFC3339Nano, fmt.Sprint(b))
		if le == nil && re == nil {
			if left.Before(right) {
				return -1
			}
			if left.After(right) {
				return 1
			}
			return 0
		}
	}
	return strings.Compare(strings.ToLower(textValue(f, a)), strings.ToLower(textValue(f, b)))
}
func emptyValue(v any) bool {
	if v == nil || v == "" {
		return true
	}
	if values, ok := v.([]any); ok {
		return len(values) == 0
	}
	return false
}
func matches(f bitable.Field, v any, filter bitable.Filter) bool {
	switch filter.Operator {
	case "empty":
		return emptyValue(v)
	case "not_empty":
		return !emptyValue(v)
	case "eq":
		return equalValue(f, v, filter.Value)
	case "neq":
		return !equalValue(f, v, filter.Value)
	case "contains":
		return strings.Contains(strings.ToLower(textValue(f, v)), strings.ToLower(textValue(f, filter.Value)))
	case "gt":
		return v != nil && filter.Value != nil && compareValue(f, v, filter.Value) > 0
	case "lt":
		return v != nil && filter.Value != nil && compareValue(f, v, filter.Value) < 0
	}
	return false
}
