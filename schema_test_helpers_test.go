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
