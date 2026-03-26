package jmespath

import "testing"

var guardBenchHitCount int

func BenchmarkProtectsWhenTrue_RepeatedChecks(b *testing.B) {
	cs, err := CompileSchema(guardBenchmarkSchema())
	if err != nil {
		b.Fatalf("CompileSchema failed: %v", err)
	}
	jp, err := CompileWithCompiledSchema("a && a.b && c != `null`", cs)
	if err != nil {
		b.Fatalf("CompileWithCompiledSchema failed: %v", err)
	}
	paths := []string{"a", "a.b", "c", "missing", "a.c"}

	b.ReportAllocs()
	b.ResetTimer()
	hits := 0
	for i := 0; i < b.N; i++ {
		if jp.ProtectsWhenTrue(paths[i%len(paths)]) {
			hits++
		}
	}
	guardBenchHitCount = hits
}

func BenchmarkCompileWithCompiledSchema_GuardAnalysis(b *testing.B) {
	cs, err := CompileSchema(guardBenchmarkSchema())
	if err != nil {
		b.Fatalf("CompileSchema failed: %v", err)
	}
	const expression = "a && a.b && (c != `null` || d)"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CompileWithCompiledSchema(expression, cs)
		if err != nil {
			b.Fatalf("CompileWithCompiledSchema failed: %v", err)
		}
	}
}

func guardBenchmarkSchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"a": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"b": map[string]interface{}{"type": "number"},
				},
				"additionalProperties": false,
			},
			"c": map[string]interface{}{"type": "number"},
			"d": map[string]interface{}{"type": "number"},
		},
		"additionalProperties": false,
	}
}
