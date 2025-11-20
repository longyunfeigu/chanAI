package tool

import (
	"reflect"
	"strings"
)

// GenerateSchema creates a JSON Schema from a Go struct.
// It supports "json" tag for field names and "description" tag for descriptions.
func GenerateSchema(v any) map[string]any {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return map[string]any{
			"type": "object", // Default fallback
		}
	}

	properties := make(map[string]any)
	required := []string{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		
		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		
		name := jsonTag
		if name == "" {
			name = field.Name
		}
		// Handle "name,omitempty"
		if comma := strings.Index(name, ","); comma != -1 {
			name = name[:comma]
		}

		desc := field.Tag.Get("description")
		
		propSchema := map[string]any{
			"type": getType(field.Type),
		}
		if desc != "" {
			propSchema["description"] = desc
		}
		
		// Handle nested structs if necessary, but for now keep it simple (primitives)
		// Expand as needed for complex types
		
		properties[name] = propSchema
		
		// Assume all fields without omitempty are required? 
		// Or check "required" tag? Let's check "jsonschema" or "required" tag.
		// For simplicity, let's say if no omitempty in json tag, it's required.
		if !strings.Contains(jsonTag, "omitempty") {
			required = append(required, name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

func getType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	default:
		return "string" // Default fallback
	}
}

