package bitable

import "encoding/json"

func validChoiceColor(color string) bool {
	switch color {
	case "", "neutral", "blue", "green", "yellow", "orange", "red", "purple", "pink":
		return true
	default:
		return false
	}
}

// ApplyCreateDefaults owns the only default-value behavior. Existing cells,
// including explicit null, are never overwritten. Callers only use this for
// ordinary creates, not imports, updates, restored snapshots or schema edits.
func ApplyCreateDefaults(fields []Field, values map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(values))
	for id, value := range values {
		result[id] = value
	}
	for _, field := range fields {
		if _, present := result[field.ID]; present || field.ReadOnly || field.Options.DefaultValue == nil {
			continue
		}
		if field.Type != FieldSingleSelect && field.Type != FieldMultipleSelect {
			continue
		}
		if e := ValidateValue(field, field.Options.DefaultValue); e != nil {
			return nil, e
		}
		// Decode into a fresh value so multi-select arrays never alias schema state.
		data, e := json.Marshal(field.Options.DefaultValue)
		if e != nil {
			return nil, ErrInvalidInput
		}
		var value any
		if e = json.Unmarshal(data, &value); e != nil {
			return nil, ErrInvalidInput
		}
		result[field.ID] = value
	}
	return result, nil
}
