package main

import (
	"context"
	"log"
	"net/http"

	"os"

	"github.com/ben-anass/DouzTach/internal/auth"
	"github.com/joho/godotenv"

	"github.com/ben-anass/DouzTach/internal/database"
	"github.com/ben-anass/DouzTach/internal/repository"
	"github.com/ben-anass/DouzTach/server/api/handlers"
	"github.com/ben-anass/DouzTach/server/api/routes"
	"github.com/ben-anass/DouzTach/server/users"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file: ", err)
	}

	// Connect to PostgreSQL
	ctx := context.Background()

	db, err := database.NewPool(ctx)
	if err != nil {
		log.Fatal("Database connection failed: ", err)
	}
	defer db.Close()

	log.Println("Database connected successfully!")
	// Initialize database queries
	queries := repository.New(db)

	log.Println("Database queries initialized:", queries != nil)

	// Initialize user service
	userService := users.NewService(queries)

	// Initialize JWT service
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	jwtService := auth.NewJWTService(jwtSecret)

	// Initialize authentication handler
	authHandler := handlers.NewAuthHandler(userService, jwtService)

	// Initialize API routes
	router := routes.NewRouter(authHandler, jwtService, userService)

	// Start HTTP server
	port := ":8080"

	log.Println("DouzTach API starting on http://localhost" + port)

	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatal(err)
	}
}
