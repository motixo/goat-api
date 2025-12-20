// Copyright (c) 2025 MOTIXO. All rights reserved.
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/motixo/goat-api/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	app, err := InitializeApp()
	if err != nil {
		panic("failed to initialize app: " + err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := app.Server.Run(cfg.ServerPort); err != nil {
			if err.Error() != "http: Server closed" {
				panic("Server failed to run: " + err.Error())
			}
		}
	}()

	app.Cleaner.Start(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// Shut down HTTP server (stop accepting new requests)
	app.Server.Shutdown(shutdownCtx)

	// Wait for background Event Handlers to complete
	app.EventBus.Wait()
}
