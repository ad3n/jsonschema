package jsonschema

import (
	stdjson "encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPropertyPathsRemainIndependent(t *testing.T) {
	schema, err := NewCompiler().Compile([]byte(`{"$id":"https://example.com/object","type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`))
	require.NoError(t, err)
	input := map[string]any{"name": "Alice"}
	first := schema.ValidateMap(input)
	require.True(t, first.IsValid())
	require.Len(t, first.Details, 1)
	detail := first.Details[0]
	require.Equal(t, "/properties/name", detail.EvaluationPath)
	require.Equal(t, "https://example.com/object#/properties/name", detail.SchemaLocation)
	require.Equal(t, "/name", detail.InstanceLocation)
	input["name"] = 42
	second := schema.Validate(input)
	require.False(t, second.IsValid())
	require.Len(t, second.Details, 1)
	second.Details[0].EvaluationPath = "changed"
	second.Details[0].InstanceLocation = "changed"
	require.True(t, first.IsValid())
	require.Equal(t, "/properties/name", detail.EvaluationPath)
	require.Equal(t, "/name", detail.InstanceLocation)
	for _, result := range []*EvaluationResult{
		schema.ValidateJSON([]byte(`{"name":"Alice"}`)),
		schema.Validate(map[string]any{"name": "Alice"}),
	} {
		require.True(t, result.IsValid())
		require.Len(t, result.Details, 1)
		require.Equal(t, detail.EvaluationPath, result.Details[0].EvaluationPath)
		require.Equal(t, detail.SchemaLocation, result.Details[0].SchemaLocation)
		require.Equal(t, detail.InstanceLocation, result.Details[0].InstanceLocation)
	}
}

func TestUniqueNumberScratchIsolation(t *testing.T) {
	type namedUint uint64
	schema, err := NewCompiler().Compile([]byte(`{"type":"array","uniqueItems":true}`))
	require.NoError(t, err)
	pairs := [][2]any{
		{int64(-9223372036854775808), stdjson.Number("-9223372036854775808")},
		{namedUint(18446744073709551615), stdjson.Number("18446744073709551615")},
		{float32(1.25), stdjson.Number("1.250")},
		{math.Copysign(0, -1), stdjson.Number("0e100")},
		{stdjson.Number("1e100"), stdjson.Number("10e99")},
		{stdjson.Number("1e-100"), stdjson.Number("10e-101")},
	}

	for i, pair := range pairs {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			for range 20 {
				values := []any{pair[0], stdjson.Number("1e-80"), stdjson.Number("invalid"), math.NaN(), stdjson.Number("-7.125"), pair[1]}
				result := schema.Validate(values)
				require.False(t, result.IsValid())
				require.Contains(t, result.Errors, "uniqueItems")
				require.Equal(t, 0, result.Errors["uniqueItems"].Params["index1"])
				require.Equal(t, 5, result.Errors["uniqueItems"].Params["index2"])
				values[5] = "different"
				require.True(t, schema.Validate(values).IsValid())
				require.False(t, result.IsValid())
			}
		})
	}

	first, second := NewRat(stdjson.Number("1.25")), NewRat(stdjson.Number("1.25"))
	first.SetInt64(9)
	require.Equal(t, "5/4", second.RatString())
}

func TestUniqueObjectScratchBoundaries(t *testing.T) {
	schema, err := NewCompiler().Compile([]byte(`{"type":"object","properties":{"items":{"type":"array","uniqueItems":true}}}`))
	require.NoError(t, err)
	for _, size := range []int{0, 1, 16, 17, 32} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			t.Parallel()
			left := make(map[string]any, size)
			right := make(map[string]stdjson.Number, size)
			for i := range size {
				key := fmt.Sprintf("key%02d", i)
				left[key] = i
				right[key] = stdjson.Number(strconv.Itoa(i) + ".0")
			}

			first := map[string]any{"outer": left, "next": []any{1.25, 0, -1}}
			second := map[string]any{"next": []any{stdjson.Number("1.25"), stdjson.Number("0"), stdjson.Number("-1")}, "outer": right}
			input := map[string]any{"items": []any{first, second}}
			data, err := marshalJSON(input)
			require.NoError(t, err)
			for _, result := range []*EvaluationResult{schema.Validate(input), schema.ValidateMap(input), schema.ValidateStruct(input), schema.ValidateJSON(data)} {
				require.False(t, result.IsValid())
			}

			first["outer"] = map[string]any{"changed": true}
			require.True(t, schema.ValidateMap(input).IsValid())
			first["outer"] = left
			require.False(t, schema.ValidateMap(input).IsValid())
			require.Len(t, left, size)
			require.Len(t, right, size)
		})
	}
}

func FuzzUniqueNumericScratch(f *testing.F) {
	f.Add([]byte{0, 1, 255, 3}, uint8(17))
	f.Add([]byte{128, 0}, uint8(32))
	schema, err := NewCompiler().Compile([]byte(`{"type":"array","uniqueItems":true}`))
	if err != nil {
		f.Fatal(err)
	}

	f.Fuzz(func(t *testing.T, data []byte, width uint8) {
		size := min(int(width), 32)
		data = data[:min(len(data), 64)]
		left, right := make(map[string]any, size), make(map[string]any, size)
		for i := range size {
			value := i
			if len(data) > 0 {
				value = int(data[i%len(data)]) - 128
			}

			key := strconv.Itoa(i)
			left[key] = float64(value) / 4
			right[key] = stdjson.Number(strconv.FormatFloat(float64(value)/4, 'f', 2, 64))
		}

		items := []any{left, stdjson.Number("1e-" + strconv.Itoa(len(data)+1)), strings.Repeat("x", len(data)), right}
		require.False(t, schema.Validate(items).IsValid())
		right["different"] = true
		require.True(t, schema.Validate(items).IsValid())
	})
}
