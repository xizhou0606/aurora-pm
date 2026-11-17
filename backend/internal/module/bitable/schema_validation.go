package bitable

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

func ValidOperationID(id string) bool {
	v, e := uuid.Parse(id)
	return e == nil && v != uuid.Nil && v.String() == id
}
func ValidateFieldMutation(in FieldMutation) error {
	if !ValidOperationID(in.OperationID) || strings.TrimSpace(in.Name) == "" || !utf8.ValidString(in.Name) || utf8.RuneCountInString(in.Name) > 200 || in.ExpectedSchemaRevision > MaxSafeRevision {
		return ErrInvalidInput
	}
	switch in.Type {
	case FieldText, FieldLongText, FieldNumber, FieldCheckbox, FieldSingleSelect, FieldMultipleSelect, FieldDate, FieldUser, FieldAttachment, FieldCreatedBy, FieldLastModifiedBy, FieldCreatedTime, FieldLastModifiedTime:
	default:
		return ErrInvalidInput
	}
	if in.Options.Multiple && in.Type != FieldUser {
		return ErrInvalidInput
	}
	if len(in.Options.Choices) > 100 || len(in.Options.Choices) > 0 && in.Type != FieldSingleSelect && in.Type != FieldMultipleSelect {
		return ErrInvalidInput
	}
	ids, names := map[string]bool{}, map[string]bool{}
	for _, c := range in.Options.Choices {
		if !validChoiceColor(c.Color) {
			return ErrInvalidInput
		}
		name := strings.TrimSpace(c.Name)
		if !validID(c.ID) || ids[c.ID] || name == "" || names[name] || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 100 {
			return ErrInvalidInput
		}
		ids[c.ID] = true
		names[name] = true
	}
	if in.Options.DefaultValue != nil {
		if in.Type != FieldSingleSelect && in.Type != FieldMultipleSelect {
			return ErrInvalidInput
		}
		if e := ValidateValue(Field{Type: in.Type, Options: in.Options}, in.Options.DefaultValue); e != nil {
			return e
		}
	}
	seenCells := map[string]bool{}
	for _, contract := range in.CellContracts {
		if ValidateCellContract(contract) != nil {
			return ErrInvalidInput
		}
		key := contract.RecordID + "\x00" + contract.FieldID
		if seenCells[key] {
			return ErrInvalidInput
		}
		seenCells[key] = true
	}
	return nil
}
func ValidateViewMutation(in ViewMutation, updating bool) error {
	if !ValidOperationID(in.OperationID) || strings.TrimSpace(in.Name) == "" || !utf8.ValidString(in.Name) || utf8.RuneCountInString(in.Name) > 200 || in.Kind != "grid" && in.Kind != "kanban" || updating && in.ExpectedRevision == nil || in.ExpectedRevision != nil && *in.ExpectedRevision > MaxSafeRevision {
		return ErrInvalidInput
	}
	c := in.Config
	if c.RowHeight < 24 || c.RowHeight > 240 || len(c.HiddenFieldIDs) > 100 || len(c.ColumnOrder) > 100 || len(c.ColumnWidths) > 100 {
		return ErrInvalidInput
	}
	for _, ids := range [][]string{c.HiddenFieldIDs, c.ColumnOrder} {
		seen := map[string]bool{}
		for _, id := range ids {
			if !validID(id) || seen[id] {
				return ErrInvalidInput
			}
			seen[id] = true
		}
	}
	for id, width := range c.ColumnWidths {
		if !validID(id) || width < 64 || width > 1200 {
			return ErrInvalidInput
		}
	}
	if c.GroupFieldID != "" && !validID(c.GroupFieldID) {
		return ErrInvalidInput
	}
	return ValidateQuery(Query{ViewID: "validation", Limit: 1, Filters: c.Filters, Sorts: c.Sorts})
}
func AuditField(t FieldType) bool {
	return t == FieldCreatedBy || t == FieldLastModifiedBy || t == FieldCreatedTime || t == FieldLastModifiedTime
}
