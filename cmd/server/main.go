package main

import (
	"context"
	"log"
	"net/http"

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

	// Initialize authentication handler
	authHandler := handlers.NewAuthHandler(userService)

	// Initialize API routes
	router := routes.NewRouter(authHandler)

	// Start HTTP server
	port := ":8080"

	log.Println("DouzTach API starting on http://localhost" + port)

	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatal(err)
	}
}
