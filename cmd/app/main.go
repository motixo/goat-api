package main

import (
	"github.com/mtextr/gopi/internal/delivery/gin"
	"github.com/mtextr/gopi/internal/usecase/user"
)

func main() {
	usersUC := user.NewUserUsecase()
	server := gin.NewServer(usersUC)
	server.Run(":8080")
}
