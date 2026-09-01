package jsonschema

import (
	stdjson "encoding/json"
	"fmt"
	"testing"
)

// BenchmarkValidateSimpleObject benchmarks validation of a simple object
func BenchmarkValidateSimpleObject(b *testing.B) {
	schema := `{"type": "object", "properties": {"name": {"type": "string"}}}`
	data := `{"name": "test"}`

	compiler := NewCompiler()
	s, err := compiler.Compile([]byte(schema))
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		_ = s.ValidateJSON([]byte(data))
	}
}

// BenchmarkValidateUniqueItems benchmarks uniqueItems validation with different array sizes
func BenchmarkValidateUniqueItems(b *testing.B) {
	schema := `{"type": "array", "uniqueItems": true}`

	for _, size := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			items := make([]int, size)
			for i := range items {
				items[i] = i
			}

			compiler := NewCompiler()
			s, err := compiler.Compile([]byte(schema))
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for b.Loop() {
				_ = s.ValidateStruct(items)
			}
		})
	}
}

// BenchmarkValidateUniqueItemsWithDuplicates benchmarks uniqueItems with duplicates
func BenchmarkValidateUniqueItemsWithDuplicates(b *testing.B) {
	schema := `{"type": "array", "uniqueItems": true}`

	for _, size := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			items := make([]int, size)
			for i := range items {
				items[i] = i % 10 // Create duplicates
			}

			compiler := NewCompiler()
			s, err := compiler.Compile([]byte(schema))
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for b.Loop() {
				_ = s.ValidateStruct(items)
			}
		})
	}
}

// BenchmarkValidateStruct benchmarks struct validation
func BenchmarkValidateStruct(b *testing.B) {
	type Person struct {
		Name  string `json:"name"`
		Age   int    `json:"age"`
		Email string `json:"email"`
	}

	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"},
			"email": {"type": "string", "format": "email"}
		}
	}`

	compiler := NewCompiler()
	s, err := compiler.Compile([]byte(schema))
	if err != nil {
		b.Fatal(err)
	}

	person := Person{Name: "John", Age: 30, Email: "john@example.com"}

	b.ResetTimer()
	for b.Loop() {
		_ = s.ValidateStruct(person)
	}
}

// BenchmarkValidateStructRepeated benchmarks repeated struct validation (tests reflection cache)
func BenchmarkValidateStructRepeated(b *testing.B) {
	type Person struct {
		Name  string `json:"name"`
		Age   int    `json:"age"`
		Email string `json:"email"`
	}

	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer", "minimum": 0},
			"email": {"type": "string", "format": "email"}
		},
		"required": ["name", "email"]
	}`

	compiler := NewCompiler()
	s, err := compiler.Compile([]byte(schema))
	if err != nil {
		b.Fatal(err)
	}

	person := Person{Name: "John", Age: 30, Email: "john@example.com"}

	b.ResetTimer()
	for b.Loop() {
		_ = s.ValidateStruct(person)
	}
}

// BenchmarkCompileSchema benchmarks schema compilation
func BenchmarkCompileSchema(b *testing.B) {
	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer", "minimum": 0},
			"email": {"type": "string", "format": "email"}
		},
		"required": ["name", "email"]
	}`

	b.ResetTimer()
	for b.Loop() {
		compiler := NewCompiler()
		_, err := compiler.Compile([]byte(schema))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidateNumberConstraints benchmarks number validation with constraints
func BenchmarkValidateNumberConstraints(b *testing.B) {
	schema := `{
		"type": "number",
		"minimum": 0,
		"maximum": 100,
		"multipleOf": 5
	}`

	compiler := NewCompiler()
	s, err := compiler.Compile([]byte(schema))
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		_ = s.ValidateStruct(50.0)
	}
}

// BenchmarkValidateNumberConstraintRepresentations measures the production
// validation path for the numeric representations accepted by the package.
func BenchmarkValidateNumberConstraintRepresentations(b *testing.B) {
	schema := `{"type":"number","minimum":0,"maximum":100,"multipleOf":5}`

	compiler := NewCompiler()
	s, err := compiler.Compile([]byte(schema))
	if err != nil {
		b.Fatal(err)
	}

	for name, value := range map[string]any{
		"int64":       int64(50),
		"float64":     float64(50),
		"json.Number": stdjson.Number("50"),
	} {
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				_ = s.ValidateStruct(value)
			}
		})
	}
}

// BenchmarkValidateNumberNoConstraints benchmarks number validation without constraints (fast path)
func BenchmarkValidateNumberNoConstraints(b *testing.B) {
	schema := `{"type": "number"}`

	compiler := NewCompiler()
	s, err := compiler.Compile([]byte(schema))
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		_ = s.ValidateStruct(50.0)
	}
}

// BenchmarkValidateAnnotationTracking measures the public validation paths
// whose evaluations need zero or one annotation-tracking map.
func BenchmarkValidateAnnotationTracking(b *testing.B) {
	compiler := NewCompiler()

	scalarSchema, err := compiler.Compile([]byte(`{"type":"string"}`))
	if err != nil {
		b.Fatal(err)
	}
	objectSchema, err := compiler.Compile([]byte(`{"type":"object","properties":{"name":{"type":"string"}}}`))
	if err != nil {
		b.Fatal(err)
	}
	arraySchema, err := compiler.Compile([]byte(`{"type":"array","items":{"type":"string"}}`))
	if err != nil {
		b.Fatal(err)
	}

	benchmarks := []struct {
		name     string
		schema   *Schema
		instance any
	}{
		{name: "scalar", schema: scalarSchema, instance: "value"},
		{name: "object", schema: objectSchema, instance: map[string]any{"name": "value"}},
		{name: "array", schema: arraySchema, instance: []any{"value"}},
	}

	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			for b.Loop() {
				_ = benchmark.schema.ValidateStruct(benchmark.instance)
			}
		})
	}
}

// BenchmarkValidateComplexObject benchmarks validation of a complex nested object
func BenchmarkValidateComplexObject(b *testing.B) {
	schema := `{
		"type": "object",
		"properties": {
			"user": {
				"type": "object",
				"properties": {
					"name": {"type": "string", "minLength": 1},
					"age": {"type": "integer", "minimum": 0, "maximum": 150},
					"email": {"type": "string", "format": "email"}
				},
				"required": ["name", "email"]
			},
			"tags": {
				"type": "array",
				"items": {"type": "string"},
				"uniqueItems": true
			}
		}
	}`

	data := `{
		"user": {
			"name": "John Doe",
			"age": 30,
			"email": "john@example.com"
		},
		"tags": ["go", "json", "schema"]
	}`

	compiler := NewCompiler()
	s, err := compiler.Compile([]byte(schema))
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		_ = s.ValidateJSON([]byte(data))
	}
}
