package main

import (
	"fmt"
	"log"
	"student-management/config"
	"student-management/controllers"
	"student-management/middleware"
	"student-management/seed"

	"github.com/gin-gonic/gin"
)

func main() {

	config.ConnectDB()

	seed.SeedData()

	fmt.Println("Seeding data completed successfully!")
	//Khởi chạy gin và các route
	r := gin.Default()

	r.POST("/login", controllers.Login)
	r.POST("/forgot-password", controllers.ForgotPassword)
	r.POST("/change-password",
		middleware.AuthRequired(),
		controllers.ChangePassword,
	)
	log.Println("Server running at http://localhost:8080")
	r.Run(":8080")
}
