package main

import (
	"log"

	"github.com/mot0x0/gopi/internal/adapter/postgres"
	"github.com/mot0x0/gopi/internal/adapter/redis"
	"github.com/mot0x0/gopi/internal/config"
	"github.com/mot0x0/gopi/internal/delivery/http"
	"github.com/mot0x0/gopi/internal/domain/usecase/jti"
	"github.com/mot0x0/gopi/internal/domain/usecase/user"
)

func main() {

	cfg := config.Get()

	db, err := postgres.NewDatabase(cfg.DBConnectionString())
	if err != nil {
		log.Fatal(err)
	}

	red := redis.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)

	userRepo := postgres.NewUserRepository(db.DB)
	jtiRepo := redis.NewJTIRepository(red.Client())

	jtiUC := jti.NewJTIUsecase(jtiRepo)
	usersUC := user.NewUserUsecase(userRepo, jtiUC)

	server := http.NewServer(usersUC)

	log.Printf("Server starting on port %s", cfg.ServerPort)
	if err := server.Run(cfg.ServerPort); err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}
