//go:build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/motixo/goat-api/internal/delivery/http"
	appWire "github.com/motixo/goat-api/internal/wire"
)

func InitializeApp() (*http.Server, error) {
	panic(wire.Build(appWire.ProviderSet))
}
