package jsonschema_test

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
	"weak"

	"github.com/stretchr/testify/require"

	"github.com/ad3n/jsonschema"
)

type scopePayload [65536]byte

func poppedInput(scope *jsonschema.DynamicScope, schema *jsonschema.Schema) weak.Pointer[scopePayload] {
	payload := new(scopePayload)
	pointer := weak.Make(payload)
	scope.Push(schema)
	scope.Push(schema, struct{ Payload *scopePayload }{payload})
	scope.Pop()
	runtime.KeepAlive(payload)
	return pointer
}

func validatedInput(schema *jsonschema.Schema) weak.Pointer[scopePayload] {
	payload := new(scopePayload)
	pointer := weak.Make(payload)
	schema.ValidateStruct(struct{ Payload *scopePayload }{payload})
	runtime.KeepAlive(payload)
	return pointer
}

func TestScopeReleasesCallerInput(t *testing.T) {
	if os.Getenv("JSONSCHEMA_SCOPE_OWNERSHIP_HELPER") == "1" {
		schema, err := jsonschema.NewCompiler().Compile([]byte(`{"type":"object"}`))
		require.NoError(t, err)
		scope := jsonschema.NewDynamicScope()
		pointer := poppedInput(scope, schema)
		runtime.GC()
		require.True(t, pointer.Value() == nil, "Pop must release the removed input while the scope remains live")
		runtime.KeepAlive(scope)
		scope.Pop()
		for range 16 {
			pointer = validatedInput(schema)
			runtime.GC()
			require.True(t, pointer.Value() == nil, "pooled validation scope must not retain caller input")
		}

		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestScopeReleasesCallerInput$")
	cmd.Env = append(os.Environ(), "JSONSCHEMA_SCOPE_OWNERSHIP_HELPER=1", "GOGC=off", "GOMAXPROCS=1")
	output, err := cmd.CombinedOutput()
	require.NoError(t, ctx.Err(), "ownership probe must terminate")
	require.NoError(t, err, "%s", output)
}
