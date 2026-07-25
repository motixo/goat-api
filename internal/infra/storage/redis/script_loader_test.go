package redis

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestEmbeddedScriptRegistryIsValid(t *testing.T) {
	t.Parallel()

	registry, err := loadScriptRegistry(luaScripts, requiredScriptNames)
	if err != nil {
		t.Fatalf("loadScriptRegistry() error = %v", err)
	}
	for _, name := range requiredScriptNames {
		if _, err := registry.script(name); err != nil {
			t.Errorf("required script %q: %v", name, err)
		}
	}
}

func TestScriptRegistryRejectsMissingRequiredScript(t *testing.T) {
	t.Parallel()

	_, err := newScriptRegistry(
		[]scriptAsset{{name: ScriptCreateSession, source: "return 1"}},
		[]ScriptName{ScriptCreateSession, ScriptRateLimit},
	)
	if !errors.Is(err, errInvalidScriptRegistry) {
		t.Fatalf("newScriptRegistry() error = %v, want invalid-registry identity", err)
	}
}

func TestScriptRegistryRejectsDuplicateScriptName(t *testing.T) {
	t.Parallel()

	_, err := newScriptRegistry(
		[]scriptAsset{
			{name: ScriptCreateSession, source: "return 1"},
			{name: ScriptCreateSession, source: "return 2"},
		},
		[]ScriptName{ScriptCreateSession},
	)
	if !errors.Is(err, errInvalidScriptRegistry) {
		t.Fatalf("newScriptRegistry() error = %v, want invalid-registry identity", err)
	}
}

func TestScriptRegistryRejectsEmptyScript(t *testing.T) {
	t.Parallel()

	_, err := newScriptRegistry(
		[]scriptAsset{{name: ScriptCreateSession, source: " \n\t"}},
		[]ScriptName{ScriptCreateSession},
	)
	if !errors.Is(err, errInvalidScriptRegistry) {
		t.Fatalf("newScriptRegistry() error = %v, want invalid-registry identity", err)
	}
}

func TestScriptRegistryLookupReturnsErrorInsteadOfPanicking(t *testing.T) {
	t.Parallel()

	registry, err := newScriptRegistry(
		[]scriptAsset{{name: ScriptCreateSession, source: "return 1"}},
		[]ScriptName{ScriptCreateSession},
	)
	if err != nil {
		t.Fatalf("newScriptRegistry() error = %v", err)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("script lookup panicked: %v", recovered)
		}
	}()
	if _, err := registry.script(ScriptName("not_registered")); !errors.Is(err, errScriptNotRegistered) {
		t.Fatalf("script() error = %v, want not-registered identity", err)
	}
}

func TestRuntimeScriptValidationRejectsMissingClientWithoutPanicking(t *testing.T) {
	t.Parallel()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("runtime script validation panicked: %v", recovered)
		}
	}()
	if err := ValidateRuntimeScripts(context.Background(), nil); !errors.Is(err, errRuntimeScriptValidation) {
		t.Fatalf("ValidateRuntimeScripts() error = %v, want runtime-validation identity", err)
	}
}

func TestProductionScriptOperationsUseRegisteredNames(t *testing.T) {
	t.Parallel()

	constantNames := map[string]ScriptName{
		"ScriptCleanOrphans":        ScriptCleanOrphans,
		"ScriptCreateSession":       ScriptCreateSession,
		"ScriptDeleteOtherSessions": ScriptDeleteOtherSessions,
		"ScriptDeleteOwnedSessions": ScriptDeleteOwnedSessions,
		"ScriptDeleteSession":       ScriptDeleteSession,
		"ScriptGetSessionByJTI":     ScriptGetSessionByJTI,
		"ScriptListSessions":        ScriptListSessions,
		"ScriptRateLimit":           ScriptRateLimit,
		"ScriptRotateJTI":           ScriptRotateJTI,
		"ScriptUnblockUser":         ScriptUnblockUser,
	}
	files := []string{
		"session/repository.go",
		"../../ratelimiter/redis_limiter.go",
	}

	lookups := 0
	for _, path := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			getScript, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || getScript.Sel.Name != "GetScript" {
				return true
			}
			lookups++
			if len(call.Args) != 1 {
				t.Errorf("%s GetScript argument count = %d, want 1", path, len(call.Args))
				return true
			}
			nameSelector, ok := call.Args[0].(*ast.SelectorExpr)
			if !ok {
				t.Errorf("%s GetScript uses a non-constant script name", path)
				return true
			}
			name, exists := constantNames[nameSelector.Sel.Name]
			if !exists {
				t.Errorf("%s GetScript references unknown constant %s", path, nameSelector.Sel.Name)
				return true
			}
			if _, err := embeddedScriptRegistry.script(name); err != nil {
				t.Errorf("%s GetScript references unregistered %q: %v", path, name, err)
			}
			return true
		})
	}
	if lookups == 0 {
		t.Fatal("no production Redis script lookups found")
	}
}

func TestRedisScriptAssetLoaderDoesNotPanic(t *testing.T) {
	t.Parallel()

	parsed, err := parser.ParseFile(token.NewFileSet(), "script_loader.go", nil, 0)
	if err != nil {
		t.Fatalf("parse script_loader.go: %v", err)
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, ok := call.Fun.(*ast.Ident)
		if ok && identifier.Name == "panic" {
			t.Error("script_loader.go contains a production panic call")
		}
		return true
	})
}
