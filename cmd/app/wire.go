//go:build wireinject

package main

//go:generate go run github.com/google/wire/cmd/wire@latest

import (
	"github.com/google/wire"
	"github.com/motixo/goat-api/internal/cron"
	"github.com/motixo/goat-api/internal/delivery/http"
	"github.com/motixo/goat-api/internal/di"
	"github.com/motixo/goat-api/internal/infra/event"
)

type AppContext struct {
	Server   *http.Server
	EventBus *event.InMemoryPublisher
	Cleaner  *cron.SessionCleaner
}

func InitializeApp() (*AppContext, error) {
	panic(wire.Build(
		di.ProviderSet,
		wire.Struct(new(AppContext), "Server", "EventBus", "Cleaner"),
	))
}
