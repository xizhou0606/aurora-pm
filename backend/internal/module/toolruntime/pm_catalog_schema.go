package toolruntime

import "encoding/json"

type pmCatalogField struct {
	name, description string
	schema            map[string]any
	required          bool
}

func pmString(name, description string, required bool) pmCatalogField {
	return pmCatalogField{name: name, description: description, required: required, schema: map[string]any{"type": "string"}}
}

func pmStringMax(name, description string, max int, required bool) pmCatalogField {
	field := pmString(name, description, required)
	field.schema["maxLength"] = max
	return field
}

func pmDateTime(name, description string, required bool) pmCatalogField {
	field := pmString(name, description, required)
	field.schema["format"] = "date-time"
	return field
}

func pmInteger(name, description string, required bool) pmCatalogField {
	return pmCatalogField{name: name, description: description, required: required, schema: map[string]any{"type": "integer", "minimum": 1}}
}

func pmNonNegativeInteger(name, description string, required bool) pmCatalogField {
	return pmCatalogField{name: name, description: description, required: required, schema: map[string]any{"type": "integer", "minimum": 0}}
}

func pmBool(name, description string, required bool) pmCatalogField {
	return pmCatalogField{name: name, description: description, required: required, schema: map[string]any{"type": "boolean"}}
}

func pmEnum(name, description string, values []string, required bool) pmCatalogField {
	return pmCatalogField{name: name, description: description, required: required, schema: map[string]any{"type": "string", "enum": values}}
}

func pmArray(name, description string, items map[string]any, required bool) pmCatalogField {
	return pmCatalogField{name: name, description: description, required: required, schema: map[string]any{"type": "array", "items": items}}
}

func pmObject(name, description string, properties map[string]any, required bool) pmCatalogField {
	return pmCatalogField{name: name, description: description, required: required, schema: map[string]any{
		"type": "object", "properties": properties, "additionalProperties": false,
	}}
}

func pmNullable(field pmCatalogField) pmCatalogField {
	value := make(map[string]any, len(field.schema))
	for key, item := range field.schema {
		value[key] = item
	}
	field.schema = map[string]any{"anyOf": []any{value, map[string]any{"type": "null"}}, "nullable": true}
	return field
}

func pmRequired(fields []pmCatalogField, names ...string) []pmCatalogField {
	required := make(map[string]struct{}, len(names))
	for _, name := range names {
		required[name] = struct{}{}
	}
	result := append([]pmCatalogField(nil), fields...)
	for index := range result {
		_, result[index].required = required[result[index].name]
	}
	return result
}

func pmWithout(fields []pmCatalogField, names ...string) []pmCatalogField {
	excluded := make(map[string]struct{}, len(names))
	for _, name := range names {
		excluded[name] = struct{}{}
	}
	result := make([]pmCatalogField, 0, len(fields))
	for _, field := range fields {
		if _, skip := excluded[field.name]; !skip {
			result = append(result, field)
		}
	}
	return result
}

func pmInputSchema(name string, fields []pmCatalogField, mutation bool) json.RawMessage {
	properties := make(map[string]any, len(fields)+1)
	required := make([]string, 0, len(fields))
	for _, field := range fields {
		schema := make(map[string]any, len(field.schema)+1)
		for key, value := range field.schema {
			schema[key] = value
		}
		schema["description"] = field.description
		properties[field.name] = schema
		if field.required {
			required = append(required, field.name)
		}
	}
	if mutation {
		properties["idempotency_key"] = map[string]any{
			"type": "string", "minLength": 8, "maxLength": 128, "pattern": idempotencyKeyPattern.String(),
			"description": "Client retry key used to deduplicate a pending mutation",
		}
	}
	schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = required
	}
	switch name {
	case "task.create":
		schema["anyOf"] = []any{
			map[string]any{"required": []string{"requirement_id"}},
			map[string]any{"required": []string{"reference_task_id"}},
			map[string]any{"required": []string{"reference_task_ref"}},
		}
	case "task.update":
		updates := []string{
			"reference_task_id", "reference_task_ref", "requirement_id", "kind", "title", "description", "status", "priority",
			"assignee_id", "reporter_user_id", "parent_id", "estimated_minutes", "starts_at", "due_at",
		}
		alternatives := make([]any, 0, len(updates))
		for _, field := range updates {
			alternatives = append(alternatives, map[string]any{"required": []string{field}})
		}
		schema["anyOf"] = alternatives
	case "task.batch_delete", "task.batch_update":
		schema["anyOf"] = []any{
			map[string]any{"required": []string{"ids"}},
			map[string]any{"required": []string{"task_refs"}},
		}
	case "task.get", "task.testing.definition.get", "task.testing.definition.update", "task.testing.context.get",
		"task.testing.execution.create", "task.testing.execution_run.list", "task.testing.execution_run.update":
		schema["anyOf"] = []any{
			map[string]any{"required": []string{"task_ref"}},
			map[string]any{"required": []string{"id"}},
		}
	case "task.workflow.kind.list", "task.workflow.status.list", "task.workflow.transition.list":
		schema["dependentRequired"] = map[string]any{"project_id": []string{"team_id"}}
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}
	return encoded
}
