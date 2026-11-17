package bitable

import (
	"encoding/json"
	"math"
	"time"
)

// ValidateValue is shared by commands and imports. The provider repeats these
// checks against the schema locked in its write transaction.
func ValidateValue(f Field, v any) error {
	if f.ReadOnly || f.Type == FieldAttachment || f.Type == FieldCreatedBy || f.Type == FieldLastModifiedBy || f.Type == FieldCreatedTime || f.Type == FieldLastModifiedTime {
		return ErrForbidden
	}
	if v == nil {
		return nil
	}
	switch f.Type {
	case FieldText, FieldLongText:
		s, ok := v.(string)
		if !ok || len(s) > 32768 {
			return ErrInvalidInput
		}
	case FieldNumber:
		n, ok := v.(float64)
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > float64(MaxSafeRevision) {
			return ErrInvalidInput
		}
	case FieldCheckbox:
		if _, ok := v.(bool); !ok {
			return ErrInvalidInput
		}
	case FieldDate:
		s, ok := v.(string)
		if !ok {
			return ErrInvalidInput
		}
		if _, e := time.Parse(time.RFC3339Nano, s); e != nil {
			return ErrInvalidInput
		}
	case FieldSingleSelect:
		s, ok := v.(string)
		if !ok || !knownChoice(f, s) {
			return ErrInvalidInput
		}
	case FieldMultipleSelect:
		data, e := json.Marshal(v)
		if e != nil {
			return ErrInvalidInput
		}
		var values []string
		if json.Unmarshal(data, &values) != nil || values == nil {
			return ErrInvalidInput
		}
		seen := map[string]bool{}
		for _, s := range values {
			if seen[s] || !knownChoice(f, s) {
				return ErrInvalidInput
			}
			seen[s] = true
		}
	case FieldUser:
		data, e := json.Marshal(v)
		if e != nil {
			return ErrInvalidInput
		}
		var users []UserRef
		if json.Unmarshal(data, &users) != nil || users == nil || len(users) > 100 || !f.Options.Multiple && len(users) > 1 {
			return ErrInvalidInput
		}
		seen := map[uint]bool{}
		for _, u := range users {
			if u.ID == 0 || uint64(u.ID) > MaxSafeRevision || seen[u.ID] {
				return ErrInvalidInput
			}
			seen[u.ID] = true
		}
	default:
		return ErrInvalidInput
	}
	return nil
}
func knownChoice(f Field, id string) bool {
	for _, c := range f.Options.Choices {
		if c.ID == id {
			return true
		}
	}
	return false
}

func ValidateBatchFields(b CommandBatch, fields []Field) error {
	byID := map[string]Field{}
	for _, f := range fields {
		byID[f.ID] = f
	}
	for _, c := range b.Commands {
		for id, v := range c.Values {
			f, ok := byID[id]
			if !ok {
				return ErrInvalidInput
			}
			if e := ValidateValue(f, v); e != nil {
				return e
			}
		}
	}
	return nil
}
