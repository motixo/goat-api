package session

import (
	"embed"
	"io/fs"
	"sync"

	"github.com/redis/go-redis/v9"
)

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

			if !d.IsDir() {
				scriptBytes, readErr := fs.ReadFile(luaScripts, path)
				if readErr != nil {
					return readErr
				}

				scriptName := path[len("scripts/") : len(path)-4]

				scripts[scriptName] = redis.NewScript(string(scriptBytes))
			}
			return nil
		})

		if err != nil {
			panic(err)
		}
	})

	return scripts
}

func getScript(name string) *redis.Script {
	scripts := getScripts()
	script, exists := scripts[name]
	if !exists {
		panic("script not found: " + name)
	}
	return script
}
