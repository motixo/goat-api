//go:build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/mot0x0/goth-api/internal/delivery/http"
	appWire "github.com/mot0x0/goth-api/internal/wire"
)

func InitializeApp() (*http.Server, error) {
	panic(wire.Build(appWire.ProviderSet))
}
