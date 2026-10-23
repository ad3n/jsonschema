package jsonschema

import (
	stdjson "encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func BenchmarkGenerateStructSchema(b *testing.B) {
	for name, generate := range map[string]func() (*Schema, error){
		"simple": func() (*Schema, error) {
			return FromStructWithOptions[TestUser](&StructTagOptions{CacheEnabled: false})
		},
		"nested": func() (*Schema, error) {
			return FromStructWithOptions[ParentWithNested](&StructTagOptions{CacheEnabled: false})
		},
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := generate(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func FuzzCompileRegexLocations(f *testing.F) {
	f.Add("", uint8(0), false)
	f.Add("a~/b", uint8(3), true)
	f.Add("日本語", uint8(8), true)
	f.Fuzz(func(t *testing.T, key string, rawDepth uint8, invalid bool) {
		if len(key) > 64 {
			t.Skip()
		}

		key = strings.ToValidUTF8(key, "�")
		depth := int(rawDepth % 9)
		pattern := "^[a-z]+$"
		if invalid {
			pattern = "["
		}

		doc := map[string]any{"pattern": pattern}
		for range depth {
			doc = map[string]any{"properties": map[string]any{
				key: map[string]any{"allOf": []any{doc}},
			}}
		}

		data, err := stdjson.Marshal(doc)
		require.NoError(t, err)
		schema, err := NewCompiler().Compile(data)
		if !invalid {
			require.NoError(t, err)
			require.True(t, schema.Validate("abc").IsValid())
			return
		}

		require.Nil(t, schema)
		require.ErrorIs(t, err, ErrRegexValidation)
		var patternErr *RegexPatternError
		require.ErrorAs(t, err, &patternErr)
		escaped := strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
		expected := "#" + strings.Repeat("/properties/"+escaped+"/allOf/0", depth) + "/pattern"
		require.Equal(t, expected, patternErr.Location)
		require.Equal(t, pattern, patternErr.Pattern)
	})
}
