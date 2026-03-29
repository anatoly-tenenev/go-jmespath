package jmespath

import "testing"

var benchmarkDateResult interface{}

func BenchmarkSchemaAwareDateComparatorLiteralScalar(b *testing.B) {
	jp, err := CompileWithSchema("createdDate >= '2026-03-01'", schemaWithDateFields())
	if err != nil {
		b.Fatal(err)
	}
	data := map[string]interface{}{
		"createdDate": "2026-03-05",
		"otherDate":   "2026-03-06",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, searchErr := jp.Search(data)
		if searchErr != nil {
			b.Fatal(searchErr)
		}
		benchmarkDateResult = result
	}
}

func BenchmarkSchemaAwareDateComparatorLiteralFilter100(b *testing.B) {
	jp, err := CompileWithSchema("items[?createdDate >= '2026-03-01']", benchmarkLiteralDateArraySchema())
	if err != nil {
		b.Fatal(err)
	}
	data := benchmarkLiteralDateArrayData(100)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, searchErr := jp.Search(data)
		if searchErr != nil {
			b.Fatal(searchErr)
		}
		benchmarkDateResult = result
	}
}

func BenchmarkSchemaAwareDateComparatorFieldToFieldScalar(b *testing.B) {
	jp, err := CompileWithSchema("createdDate < otherDate", schemaWithDateFields())
	if err != nil {
		b.Fatal(err)
	}
	data := map[string]interface{}{
		"createdDate": "2026-03-05",
		"otherDate":   "2026-03-06",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, searchErr := jp.Search(data)
		if searchErr != nil {
			b.Fatal(searchErr)
		}
		benchmarkDateResult = result
	}
}

func BenchmarkSchemaAwareDateComparatorFieldToFieldFilter100(b *testing.B) {
	jp, err := CompileWithSchema("items[?createdDate < otherDate]", benchmarkFieldDateArraySchema())
	if err != nil {
		b.Fatal(err)
	}
	data := benchmarkFieldDateArrayData(100)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, searchErr := jp.Search(data)
		if searchErr != nil {
			b.Fatal(searchErr)
		}
		benchmarkDateResult = result
	}
}

func benchmarkLiteralDateArraySchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"createdDate": map[string]interface{}{
							"type":   "string",
							"format": "date",
						},
					},
					"required":             []interface{}{"createdDate"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []interface{}{"items"},
		"additionalProperties": false,
	}
}

func benchmarkFieldDateArraySchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"createdDate": map[string]interface{}{
							"type":   "string",
							"format": "date",
						},
						"otherDate": map[string]interface{}{
							"type":   "string",
							"format": "date",
						},
					},
					"required":             []interface{}{"createdDate", "otherDate"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []interface{}{"items"},
		"additionalProperties": false,
	}
}

func benchmarkLiteralDateArrayData(size int) map[string]interface{} {
	items := make([]interface{}, 0, size)
	for i := 0; i < size; i++ {
		date := "2026-02-27"
		if i%2 == 0 {
			date = "2026-03-05"
		}
		items = append(items, map[string]interface{}{
			"createdDate": date,
		})
	}
	return map[string]interface{}{"items": items}
}

func benchmarkFieldDateArrayData(size int) map[string]interface{} {
	items := make([]interface{}, 0, size)
	for i := 0; i < size; i++ {
		createdDate := "2026-03-05"
		otherDate := "2026-03-06"
		if i%2 != 0 {
			createdDate = "2026-03-07"
		}
		items = append(items, map[string]interface{}{
			"createdDate": createdDate,
			"otherDate":   otherDate,
		})
	}
	return map[string]interface{}{"items": items}
}
