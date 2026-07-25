package redis

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/redis/go-redis/v9"
)

// ScriptName identifies one embedded Redis script registered by this adapter.
type ScriptName string

const (
	ScriptCleanOrphans        ScriptName = "clean_orphans"
	ScriptCreateSession       ScriptName = "create_session"
	ScriptDeleteOtherSessions ScriptName = "delete_other_sessions"
	ScriptDeleteOwnedSessions ScriptName = "delete_owned_sessions"
	ScriptDeleteSession       ScriptName = "delete_session"
	ScriptGetSessionByJTI     ScriptName = "get_session_by_jti"
	ScriptListSessions        ScriptName = "list_sessions"
	ScriptRateLimit           ScriptName = "rate_limit"
	ScriptRotateJTI           ScriptName = "rotate_jti"
	ScriptUnblockUser         ScriptName = "unblock_user"
)

var (
	errInvalidScriptRegistry   = errors.New("invalid embedded Redis script registry")
	errScriptNotRegistered     = errors.New("redis script is not registered")
	errRuntimeScriptValidation = errors.New("runtime Redis script validation failed")
	requiredScriptNames        = []ScriptName{
		ScriptCleanOrphans,
		ScriptCreateSession,
		ScriptDeleteOtherSessions,
		ScriptDeleteOwnedSessions,
		ScriptDeleteSession,
		ScriptGetSessionByJTI,
		ScriptListSessions,
		ScriptRateLimit,
		ScriptRotateJTI,
		ScriptUnblockUser,
	}
)

//go:embed scripts/*.lua
var luaScripts embed.FS

type scriptAsset struct {
	name   ScriptName
	source string
}

type scriptRegistry struct {
	scripts map[ScriptName]*redis.Script
}

var embeddedScriptRegistry, embeddedScriptRegistryErr = loadScriptRegistry(luaScripts, requiredScriptNames)

func loadScriptRegistry(fsys fs.FS, required []ScriptName) (*scriptRegistry, error) {
	assets := make([]scriptAsset, 0, len(required))
	err := fs.WalkDir(fsys, "scripts", func(scriptPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(scriptPath, ".lua") {
			return nil
		}

		source, err := fs.ReadFile(fsys, scriptPath)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(strings.TrimPrefix(scriptPath, "scripts/"), ".lua")
		assets = append(assets, scriptAsset{
			name:   ScriptName(name),
			source: string(source),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: read embedded scripts: %w", errInvalidScriptRegistry, err)
	}
	return newScriptRegistry(assets, required)
}

func newScriptRegistry(assets []scriptAsset, required []ScriptName) (*scriptRegistry, error) {
	registry := &scriptRegistry{
		scripts: make(map[ScriptName]*redis.Script, len(assets)),
	}
	for _, asset := range assets {
		if strings.TrimSpace(string(asset.name)) == "" {
			return nil, fmt.Errorf("%w: script name is empty", errInvalidScriptRegistry)
		}
		if strings.TrimSpace(asset.source) == "" {
			return nil, fmt.Errorf("%w: script %q is empty", errInvalidScriptRegistry, asset.name)
		}
		if _, duplicate := registry.scripts[asset.name]; duplicate {
			return nil, fmt.Errorf("%w: script %q is registered more than once", errInvalidScriptRegistry, asset.name)
		}
		registry.scripts[asset.name] = redis.NewScript(asset.source)
	}

	seenRequired := make(map[ScriptName]struct{}, len(required))
	for _, name := range required {
		if strings.TrimSpace(string(name)) == "" {
			return nil, fmt.Errorf("%w: required script name is empty", errInvalidScriptRegistry)
		}
		if _, duplicate := seenRequired[name]; duplicate {
			return nil, fmt.Errorf("%w: required script %q is declared more than once", errInvalidScriptRegistry, name)
		}
		seenRequired[name] = struct{}{}
		if _, exists := registry.scripts[name]; !exists {
			return nil, fmt.Errorf(
				"%w: required script %q: %w",
				errInvalidScriptRegistry,
				name,
				errScriptNotRegistered,
			)
		}
	}
	return registry, nil
}

func (r *scriptRegistry) script(name ScriptName) (*redis.Script, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: registry is unavailable", errInvalidScriptRegistry)
	}
	script, exists := r.scripts[name]
	if !exists {
		return nil, fmt.Errorf("%w: %q", errScriptNotRegistered, name)
	}
	return script, nil
}

// GetScript returns a registered embedded script without panicking on invalid
// registry state or unknown names.
func GetScript(name ScriptName) (*redis.Script, error) {
	if embeddedScriptRegistryErr != nil {
		return nil, fmt.Errorf("load embedded Redis scripts: %w", embeddedScriptRegistryErr)
	}
	return embeddedScriptRegistry.script(name)
}

// ValidateRuntimeScripts validates the embedded registry and asks the mandatory
// Redis dependency to compile every script before runtime construction.
func ValidateRuntimeScripts(ctx context.Context, client *redis.Client) error {
	if embeddedScriptRegistryErr != nil {
		return fmt.Errorf("%w: %w", errRuntimeScriptValidation, embeddedScriptRegistryErr)
	}
	return validateScriptRegistry(ctx, client, embeddedScriptRegistry, requiredScriptNames)
}

func validateScriptRegistry(
	ctx context.Context,
	client *redis.Client,
	registry *scriptRegistry,
	required []ScriptName,
) error {
	if ctx == nil {
		return fmt.Errorf("%w: context is required", errRuntimeScriptValidation)
	}
	if client == nil {
		return fmt.Errorf("%w: Redis client is required", errRuntimeScriptValidation)
	}

	for _, name := range required {
		script, err := registry.script(name)
		if err != nil {
			return fmt.Errorf("%w: resolve script %q: %w", errRuntimeScriptValidation, name, err)
		}
		if err := script.Load(ctx, client).Err(); err != nil {
			return fmt.Errorf("%w: compile script %q: %w", errRuntimeScriptValidation, name, err)
		}
	}
	return nil
}
