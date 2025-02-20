package main

import (
	"jnv-jit/internal/routes"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	router := gin.Default()
	routes.RegisterRoutes(router)

	port := os.Getenv("port")
	log.Printf("Starting server on port: %s ,as time: %s\n", port, time.Now())
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
