package jsonschema

import (
	stdjson "encoding/json"
	"fmt"
	"testing"
)

func BenchmarkProfileEntryPoints(b *testing.B) {
	schema, err := NewCompiler().Compile([]byte(`{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer","minimum":0},"active":{"type":"boolean"}},"required":["name","age"]}`))
	if err != nil {
		b.Fatal(err)
	}

	data := []byte(`{"name":"Alice","age":30,"active":true}`)
	object := map[string]any{"name": "Alice", "age": 30, "active": true}
	person := struct {
		Name   string `json:"name"`
		Age    int    `json:"age"`
		Active bool   `json:"active"`
	}{Name: "Alice", Age: 30, Active: true}

	for name, validate := range map[string]func() *EvaluationResult{
		"JSON":   func() *EvaluationResult { return schema.ValidateJSON(data) },
		"Map":    func() *EvaluationResult { return schema.ValidateMap(object) },
		"Struct": func() *EvaluationResult { return schema.ValidateStruct(person) },
	} {
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				if !validate().IsValid() {
					b.Fatal("valid instance rejected")
				}
			}
		})
	}
}

func BenchmarkProfileUniqueNumbers(b *testing.B) {
	schema, err := NewCompiler().Compile([]byte(`{"type":"array","uniqueItems":true}`))
	if err != nil {
		b.Fatal(err)
	}

	if schema.Validate([]any{1, 1}).IsValid() {
		b.Fatal("duplicate numbers accepted")
	}

	for _, size := range []int{10, 100, 1000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			values := make([]any, size)
			for i := range values {
				values[i] = i
			}

			for b.Loop() {
				if !schema.Validate(values).IsValid() {
					b.Fatal("unique numbers rejected")
				}
			}
		})
	}
}

func BenchmarkProfileUniqueRepresentations(b *testing.B) {
	schema, err := NewCompiler().Compile([]byte(`{"type":"array","uniqueItems":true}`))
	if err != nil {
		b.Fatal(err)
	}

	for _, kind := range []string{"float", "json", "large", "wide-object"} {
		b.Run(kind, func(b *testing.B) {
			values := make([]any, 100)
			for i := range values {
				switch kind {
				case "float":
					values[i] = float64(i) + 0.25
				case "json":
					values[i] = stdjson.Number(fmt.Sprintf("%d.25", i))
				case "large":
					values[i] = stdjson.Number(fmt.Sprintf("%d123456789012345678901234567890123456789012345678901234567890123456789.25", i+1))
				case "wide-object":
					object := make(map[string]any, 32)
					for j := range 32 {
						object[fmt.Sprint(j)] = i*32 + j
					}

					values[i] = object
				}
			}

			for b.Loop() {
				if !schema.Validate(values).IsValid() {
					b.Fatal("unique values rejected")
				}
			}
		})
	}
}
