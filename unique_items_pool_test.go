package jsonschema

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReleaseUniqueItems(t *testing.T) {
	hashes := map[uint64][]int{1: {0, 2}, 2: {1}}
	releaseUniqueItems(&hashes)
	require.Empty(t, hashes)
}

func TestUniqueItemsPoolSizes(t *testing.T) {
	schema := Array(UniqueItems(true))
	for _, size := range []int{maxPooledUniqueItems, 2, maxPooledUniqueItems + 1, 2} {
		items := make([]any, size)
		for i := range items {
			items[i] = fmt.Sprint(i)
		}

		require.True(t, schema.Validate(items).IsValid())
		items[len(items)-1] = items[0]
		require.False(t, schema.Validate(items).IsValid())
	}
}

func TestUniqueObjectsConcurrent(t *testing.T) {
	schema, err := NewCompiler().Compile([]byte(`{"type":"object","properties":{"items":{"type":"array","uniqueItems":true}}}`))
	require.NoError(t, err)

	for worker := range 16 {
		t.Run(fmt.Sprint(worker), func(t *testing.T) {
			t.Parallel()

			for range 50 {
				first := map[string]any{"outer": map[string]any{"value": "a"}, "tag": "same"}
				second := map[string]any{"outer": map[string]string{"value": "b"}, "tag": "same"}
				input := map[string]any{"items": []any{first, second}}
				valid := schema.ValidateMap(input)
				require.True(t, valid.IsValid())

				second["outer"] = map[string]string{"value": "a"}
				invalid := schema.Validate(input)
				require.False(t, invalid.IsValid())
				require.False(t, schema.ValidateStruct(input).IsValid())
				require.False(t, schema.ValidateJSON([]byte(`{"items":[{"outer":{"value":"a"},"tag":"same"},{"tag":"same","outer":{"value":"a"}}]}`)).IsValid())

				delete(second, "tag")
				require.True(t, schema.ValidateMap(input).IsValid())
				require.True(t, valid.IsValid())
				require.False(t, invalid.IsValid())
				require.Equal(t, "same", first["tag"])
				require.Equal(t, map[string]any{"value": "a"}, first["outer"])
			}
		})
	}
}

func FuzzUniqueObjectsPoolReuse(f *testing.F) {
	f.Add(`{"outer":{"value":1},"tag":true}`)
	f.Add(`{}`)
	f.Add(`{"outer":[{"value":null}]}`)
	schema := Array(UniqueItems(true))
	f.Fuzz(func(t *testing.T, input string) {
		var value map[string]any
		if unmarshalJSON([]byte(input), &value) != nil || value == nil {
			return
		}

		require.False(t, schema.Validate([]any{value, value}).IsValid())
		require.True(t, schema.Validate([]any{value, "different kind"}).IsValid())
		require.False(t, schema.Validate([]any{value, value}).IsValid())
	})
}
