package jsonschema

import (
	"fmt"
	"testing"
)

func BenchmarkValidateUniqueObjects(b *testing.B) {
	schema, err := NewCompiler().Compile([]byte(`{"type":"array","uniqueItems":true}`))
	if err != nil {
		b.Fatal(err)
	}

	for _, size := range []int{10, 1000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			items := make([]any, size)
			for i := range items {
				items[i] = map[string]any{
					"id": fmt.Sprint(i), "name": "example", "active": true,
					"metadata": map[string]any{"region": "id", "role": "reader"},
				}
			}

			b.ReportAllocs()
			for b.Loop() {
				if result := schema.Validate(items); !result.IsValid() {
					b.Fatal(result)
				}
			}
		})
	}
}
