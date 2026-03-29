package jmespath

func compileTestSchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"foo": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"bar": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"baz": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "number",
								},
							},
						},
						"additionalProperties": false,
					},
				},
				"additionalProperties": false,
			},
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"price": map[string]interface{}{
							"type": "number",
						},
						"name": map[string]interface{}{
							"type": "string",
						},
					},
					"additionalProperties": false,
				},
			},
			"name": map[string]interface{}{
				"type": "string",
			},
		},
		"additionalProperties": false,
	}
}

func compileTestSchemaWithRequired(required ...string) JSONSchema {
	schema := compileTestSchema()
	if len(required) == 0 {
		return schema
	}
	items := make([]interface{}, len(required))
	for i, name := range required {
		items[i] = name
	}
	schema["required"] = items
	return schema
}

func functionNullableSafetySchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sections": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"summary": map[string]interface{}{
								"type": "string",
							},
						},
						"additionalProperties": false,
					},
				},
				"required":             []interface{}{"sections"},
				"additionalProperties": false,
			},
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"records": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type": "string",
						},
						"label": map[string]interface{}{
							"type": "string",
						},
						"rank": map[string]interface{}{
							"type": "number",
						},
					},
					"required":             []interface{}{"id"},
					"additionalProperties": false,
				},
			},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type": "string",
					},
					"status": map[string]interface{}{
						"type": "string",
					},
				},
				"required":             []interface{}{"status"},
				"additionalProperties": false,
			},
			"numbers": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "number",
				},
			},
			"optional_numbers": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "number",
				},
			},
			"mixed_array": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"optional_mixed_array": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"optional_path": map[string]interface{}{
				"type": "string",
			},
			"optional_number": map[string]interface{}{
				"type": "number",
			},
			"null_field": map[string]interface{}{
				"type": "string",
			},
		},
		"required":             []interface{}{"items", "records", "meta", "numbers", "mixed_array"},
		"additionalProperties": false,
	}
}
