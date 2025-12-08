package session

import (
	"embed"
	"io/fs"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
)

//go:embed scripts/*.lua
var luaScripts embed.FS

var (
	scripts map[string]*redis.Script
	once    sync.Once
)

func getScripts() map[string]*redis.Script {
	once.Do(func() {
		scripts = make(map[string]*redis.Script)

		err := fs.WalkDir(luaScripts, "scripts", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			// Only load .lua files
			if !strings.HasSuffix(path, ".lua") {
				return nil // skip non-Lua files
			}

			scriptBytes, err := fs.ReadFile(luaScripts, path)
			if err != nil {
				return err
			}

			// Extract name: scripts/create_session.lua → create_session
			scriptName := strings.TrimSuffix(path[len("scripts/"):], ".lua")
			scripts[scriptName] = redis.NewScript(string(scriptBytes))
			return nil
		})

		if err != nil {
			panic("failed to load Redis Lua scripts: " + err.Error())
		}
	})

	return scripts
}

func getScript(name string) *redis.Script {
	scripts := getScripts()
	if script, exists := scripts[name]; exists {
		return script
	}
	panic("embedded Redis Lua script not found: " + name)
}
