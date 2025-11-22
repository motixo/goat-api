package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/mtextr/gopi/internal/adapter/postgres"
	"github.com/mtextr/gopi/internal/delivery/gin"
	"github.com/mtextr/gopi/internal/usecase/user"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using default environment variables")
	}

	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	serverPort := ":" + os.Getenv("SERVER_PORT")
	jwtSecret := os.Getenv("JWT_SECRET")

	db, err := postgres.NewDatabase(dbHost, dbPort, dbUser, dbPassword, dbName)
	if err != nil {
		log.Fatal(err)
	}

	userRepo := postgres.NewUserRepo(db.DB)
	usersUC := user.NewUserUsecase(userRepo, jwtSecret)
	server := gin.NewServer(usersUC)

	server.Run(serverPort)
}
