package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProcessPackageQuotesPattern(t *testing.T) {
	t.Chdir(t.TempDir())
	source := "package sample\ntype Sample struct { Value string `json:\"value\" jsonschema:\"pattern=^\\x60$\"` }\n"
	require.NoError(t, os.WriteFile("models.go", []byte(source), 0o600))
	generator, err := NewCodeGenerator(&GeneratorConfig{OutputSuffix: "_schema.go"})
	require.NoError(t, err)
	require.NoError(t, generator.ProcessPackage("."))
	generated, err := os.ReadFile("sample_schema.go")
	require.NoError(t, err)
	file, err := parser.ParseFile(token.NewFileSet(), "sample_schema.go", generated, 0)
	require.NoError(t, err)
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Pattern" {
			return true
		}

		require.Len(t, call.Args, 1)
		literal, ok := call.Args[0].(*ast.BasicLit)
		require.True(t, ok)
		pattern, err := strconv.Unquote(literal.Value)
		require.NoError(t, err)
		require.Equal(t, "^`$", pattern)
		found = true
		return true
	})
	require.True(t, found, "generated schema must retain the pattern")
}

func BenchmarkProcessPackagePattern(b *testing.B) {
	b.Chdir(b.TempDir())
	source := "package sample\ntype Sample struct { Value string `json:\"value\" jsonschema:\"required,pattern=^[a-z]+$\"` }\n"
	if err := os.WriteFile("models.go", []byte(source), 0o600); err != nil {
		b.Fatal(err)
	}

	generator, err := NewCodeGenerator(&GeneratorConfig{OutputSuffix: "_schema.go"})
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		if err := generator.ProcessPackage("."); err != nil {
			b.Fatal(err)
		}
	}
}

func FuzzGeneratedPatternLiteral(f *testing.F) {
	f.Add("^[a-z]+$")
	f.Add("^`$\n\r\x00")
	generator, err := NewCodeGenerator(&GeneratorConfig{DryRun: true})
	if err != nil {
		f.Fatal(err)
	}

	f.Fuzz(func(t *testing.T, pattern string) {
		if len(pattern) > 1024 {
			t.Skip()
		}

		source := generator.validatorMap["pattern"]("string", []string{pattern})
		expression, err := parser.ParseExpr(source)
		require.NoError(t, err)
		call, ok := expression.(*ast.CallExpr)
		require.True(t, ok)
		require.Len(t, call.Args, 1)
		literal, ok := call.Args[0].(*ast.BasicLit)
		require.True(t, ok)
		decoded, err := strconv.Unquote(literal.Value)
		require.NoError(t, err)
		require.Equal(t, pattern, decoded)
	})
}
