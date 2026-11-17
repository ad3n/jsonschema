package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ad3n/jsonschema/pkg/tagparser"
)

func TestReferenceAnalyzer_AnalyzePackageDependenciesFromTypeNames(t *testing.T) {
	for _, typeName := range []string{"Profile", "*Profile", "[]Profile", "[]*Profile", "map[string]Profile", "map[string]*Profile"} {
		t.Run(typeName, func(t *testing.T) {
			analyzer := NewReferenceAnalyzer()
			infos := []*GenerationInfo{
				{Name: "User", Fields: []tagparser.FieldInfo{{Name: "Value", TypeName: typeName}}},
				{Name: "Profile"},
			}
			require.NoError(t, analyzer.AnalyzePackageDependencies(infos))
			assert.Equal(t, []string{"Profile"}, analyzer.ReferencedStructs("User"))
			assert.False(t, analyzer.HasCycles())
		})
	}
}

func TestReferenceAnalyzer_DetectsCyclesFromTypeNames(t *testing.T) {
	analyzer := NewReferenceAnalyzer()
	infos := []*GenerationInfo{
		{
			Name:    "Node",
			Package: "sample",
			Fields:  []tagparser.FieldInfo{{Name: "Children", TypeName: "[]Node"}},
		},
	}

	err := analyzer.AnalyzePackageDependencies(infos)
	require.NoError(t, err)

	assert.True(t, analyzer.HasCycles())
	assert.True(t, analyzer.NeedsRefGeneration("Node"))
	assert.Contains(t, analyzer.graph.edges["Node"], "Node")
}

func TestReferenceAnalyzerDoesNotRetainCallerFields(t *testing.T) {
	analyzer := NewReferenceAnalyzer()
	fields := []tagparser.FieldInfo{{Name: "Value", TypeName: "Profile"}}
	infos := []*GenerationInfo{{Name: "User", Fields: fields}, {Name: "Profile"}}
	require.NoError(t, analyzer.AnalyzePackageDependencies(infos))
	fields[0].TypeName = "Other"
	infos[0].Name = "Changed"
	references := analyzer.ReferencedStructs("User")
	require.Equal(t, []string{"Profile"}, references)
	references[0] = "Changed"
	assert.Equal(t, []string{"Profile"}, analyzer.ReferencedStructs("User"))
}

func FuzzReferenceAnalyzerTypeNames(f *testing.F) {
	for _, name := range []string{"Node", "*Node", "[]*Node", "map[string]*Node", "", "map[", "string"} {
		f.Add(name)
	}

	f.Fuzz(func(t *testing.T, name string) {
		if len(name) > 1024 {
			t.Skip()
		}

		analyzer := NewReferenceAnalyzer()
		require.NoError(t, analyzer.AnalyzePackageDependencies([]*GenerationInfo{
			{Name: "Node", Fields: []tagparser.FieldInfo{{TypeName: name}}},
		}))
		references := analyzer.ReferencedStructs("Node")
		require.LessOrEqual(t, len(references), 1)
		assert.NotContains(t, references, "Node")
		assert.Equal(t, analyzer.HasCycles(), analyzer.NeedsRefGeneration("Node"))
	})
}

// Basic Reference Tests
func TestCodeGenerator_BasicReferences(t *testing.T) {
	generator := newTestGenerator(t)

	// Test simple struct reference
	field := tagparser.FieldInfo{
		Name:     "Profile",
		TypeName: "UserProfile",
		JSONName: "profile",
		Rules: []tagparser.TagRule{
			{Name: "required", Params: nil},
		},
		Required: true,
	}

	property, err := generator.generateFieldProperty(&field)
	require.NoError(t, err, "generateFieldProperty should succeed")

	// Should generate direct method call for simple references
	assert.Contains(t, property, "(&UserProfile{}).Schema", "should generate simple struct reference")
}

// $refs and $defs Tests
func TestCodeGenerator_RefsAndDefs(t *testing.T) {
	generator := newTestGenerator(t)

	// Initialize the analyzer for ref detection
	if generator.analyzer == nil {
		generator.analyzer, _ = NewStructAnalyzer()
	}

	tests := []struct {
		name          string
		field         tagparser.FieldInfo
		ruleName      string
		params        []string
		expectedMatch string
	}{
		{
			name: "items with custom struct",
			field: tagparser.FieldInfo{
				Name:     "Users",
				TypeName: "[]User",
				JSONName: "users",
			},
			ruleName:      "items",
			params:        []string{"User"},
			expectedMatch: "jsonschema.Items",
		},
		{
			name: "additionalProperties with struct",
			field: tagparser.FieldInfo{
				Name:     "Metadata",
				TypeName: "map[string]any",
				JSONName: "metadata",
			},
			ruleName:      "additionalProperties",
			params:        []string{"User"},
			expectedMatch: "jsonschema.AdditionalPropsSchema",
		},
		{
			name: "allOf combination",
			field: tagparser.FieldInfo{
				Name:     "Combined",
				TypeName: "any",
				JSONName: "combined",
			},
			ruleName:      "allOf",
			params:        []string{"BaseUser", "ExtendedUser"},
			expectedMatch: "jsonschema.AllOf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validatorGen, exists := generator.validatorMap[tt.ruleName]
			require.True(t, exists, "validator %s should exist", tt.ruleName)

			result := validatorGen(tt.field.TypeName, tt.params)
			assert.Contains(t, result, tt.expectedMatch, "result should contain expected match")

			// Test $ref transformation
			transformed := generator.applyRefTransformation(result, tt.ruleName, tt.params)
			t.Logf("Original: %s", result)
			t.Logf("Transformed: %s", transformed)
		})
	}
}

// Circular Reference Tests
func TestCodeGenerator_CircularReferences(t *testing.T) {
	generator := newVerboseTestGenerator(t)

	// Test circular reference detection and handling
	tests := []struct {
		name            string
		originalCode    string
		ruleName        string
		params          []string
		shouldTransform bool
		expectedPattern string
	}{
		{
			name:            "items with circular reference",
			originalCode:    "jsonschema.Items((&User{}).Schema())",
			ruleName:        "items",
			params:          []string{"User"},
			shouldTransform: false, // Would be true if circular dependency detected
			expectedPattern: "Schema()",
		},
		{
			name:            "additionalProperties transformation",
			originalCode:    "jsonschema.AdditionalPropsSchema((&User{}).Schema())",
			ruleName:        "additionalProperties",
			params:          []string{"User"},
			shouldTransform: false, // Would be true if circular dependency detected
			expectedPattern: "Schema()",
		},
		{
			name:            "allOf transformation",
			originalCode:    "jsonschema.AllOf((&BaseUser{}).Schema(), (&ExtendedUser{}).Schema())",
			ruleName:        "allOf",
			params:          []string{"BaseUser", "ExtendedUser"},
			shouldTransform: false, // Would be true if circular dependency detected
			expectedPattern: "Schema()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.applyRefTransformation(tt.originalCode, tt.ruleName, tt.params)

			if tt.shouldTransform {
				// If circular reference is detected, should use $ref
				assert.Contains(t, result, "jsonschema.Ref", "should contain $ref transformation")
			} else {
				// If no circular reference, should preserve original
				assert.Contains(t, result, tt.expectedPattern, "should preserve expected pattern")
			}
		})
	}
}

// Complex Reference Scenarios
func TestCodeGenerator_ComplexReferenceScenarios(t *testing.T) {
	generator := newVerboseTestGenerator(t)

	// Test struct with multiple reference types
	structInfo := &GenerationInfo{
		Name:    "ComplexReferenceStruct",
		Package: "main",
		Fields: []tagparser.FieldInfo{
			// Direct struct reference
			{
				Name:     "Profile",
				TypeName: "UserProfile",
				JSONName: "profile",
				Rules: []tagparser.TagRule{
					{Name: "required", Params: nil},
				},
				Required: true,
			},
			// Array with struct items
			{
				Name:     "Friends",
				TypeName: "[]User",
				JSONName: "friends",
				Rules: []tagparser.TagRule{
					{Name: "items", Params: []string{"User"}},
					{Name: "minItems", Params: []string{"0"}},
				},
			},
			// Map with struct values
			{
				Name:     "Settings",
				TypeName: "map[string]any",
				JSONName: "settings",
				Rules: []tagparser.TagRule{
					{Name: "additionalProperties", Params: []string{"Setting"}},
				},
			},
			// Logical combination with structs
			{
				Name:     "Contact",
				TypeName: "any",
				JSONName: "contact",
				Rules: []tagparser.TagRule{
					{Name: "anyOf", Params: []string{"EmailContact", "PhoneContact"}},
				},
			},
			// Conditional logic with structs
			{
				Name:     "Auth",
				TypeName: "any",
				JSONName: "auth",
				Rules: []tagparser.TagRule{
					{Name: "if", Params: []string{"AdminUser"}},
					{Name: "then", Params: []string{"AdminAuth"}},
					{Name: "else", Params: []string{"UserAuth"}},
				},
			},
		},
		FilePath: "complex_reference_struct.go",
	}

	err := generator.generateStructCode(structInfo)
	assert.NoError(t, err, "generateStructCode should succeed")

	t.Log("=== Complex Reference Scenarios Test Completed ===")
}

// $defs Generation Test
func TestCodeGenerator_DefsGeneration(t *testing.T) {
	generator := newVerboseTestGenerator(t)

	// Test generateDefinition method
	defData, err := generator.generateDefinition("TestStruct")
	require.NoError(t, err, "generateDefinition should succeed")
	require.NotNil(t, defData, "definition should not be nil")

	assert.Equal(t, "TestStruct", defData.Name, "definition name should match")

	t.Logf("Generated definition: %+v", defData)
}

// Reference Resolution Demo
func TestCodeGenerator_ReferenceResolutionDemo(t *testing.T) {
	generator := newVerboseTestGenerator(t)

	// Demo showing different reference scenarios
	structInfo := &GenerationInfo{
		Name:    "ReferenceDemo",
		Package: "main",
		Fields: []tagparser.FieldInfo{
			{
				Name:     "ID",
				TypeName: "string",
				JSONName: "id",
				Rules: []tagparser.TagRule{
					{Name: "required", Params: nil},
				},
				Required: true,
			},
			{
				Name:     "SimpleRef",
				TypeName: "Address",
				JSONName: "simple_ref",
				Rules: []tagparser.TagRule{
					{Name: "required", Params: nil},
				},
				Required: true,
			},
			{
				Name:     "ArrayRef",
				TypeName: "[]Contact",
				JSONName: "array_ref",
				Rules: []tagparser.TagRule{
					{Name: "items", Params: []string{"Contact"}},
					{Name: "minItems", Params: []string{"1"}},
				},
			},
			{
				Name:     "MapRef",
				TypeName: "map[string]any",
				JSONName: "map_ref",
				Rules: []tagparser.TagRule{
					{Name: "additionalProperties", Params: []string{"Setting"}},
				},
			},
		},
		FilePath: "reference_demo.go",
	}

	err := generator.generateStructCode(structInfo)
	assert.NoError(t, err, "generateStructCode should succeed")

	t.Log("=== Reference Resolution Demo Completed ===")
	t.Log("This demo shows how references are handled:")
	t.Log("- Simple struct references: (&Address{}).Schema()")
	t.Log("- Array items with struct types: jsonschema.Items((&Contact{}).Schema())")
	t.Log("- Map additional properties: jsonschema.AdditionalPropsSchema((&Setting{}).Schema())")
	t.Log("- Circular references would use: jsonschema.Ref(\"#/$defs/StructName\")")
}
