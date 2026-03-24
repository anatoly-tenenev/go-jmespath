package jmespath

import "testing"

func BenchmarkCompileSchema_Small(b *testing.B) {
	schema := compileTestSchema()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CompileSchema(schema)
		if err != nil {
			b.Fatalf("CompileSchema failed: %v", err)
		}
	}
}

func BenchmarkCompileWithCompiledSchema_SimplePath(b *testing.B) {
	schema := compileTestSchema()
	cs, err := CompileSchema(schema)
	if err != nil {
		b.Fatalf("CompileSchema failed: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CompileWithCompiledSchema("foo.bar.baz[2]", cs)
		if err != nil {
			b.Fatalf("CompileWithCompiledSchema failed: %v", err)
		}
	}
}

func BenchmarkCompileWithCompiledSchema_Filter(b *testing.B) {
	schema := compileTestSchema()
	cs, err := CompileSchema(schema)
	if err != nil {
		b.Fatalf("CompileSchema failed: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CompileWithCompiledSchema("items[?price > `10`].price", cs)
		if err != nil {
			b.Fatalf("CompileWithCompiledSchema failed: %v", err)
		}
	}
}

func BenchmarkCompileWithSchema_ReparsePenalty(b *testing.B) {
	schema := compileTestSchema()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CompileWithSchema("foo.bar.baz[2]", schema)
		if err != nil {
			b.Fatalf("CompileWithSchema failed: %v", err)
		}
	}
}
