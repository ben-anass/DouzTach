package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/ben-anass/DouzTach/internal/auth"
	"github.com/ben-anass/DouzTach/internal/database"
	"github.com/ben-anass/DouzTach/internal/repository"
	"github.com/ben-anass/DouzTach/server/activity"
	"github.com/ben-anass/DouzTach/server/api/handlers"
	"github.com/ben-anass/DouzTach/server/api/routes"
	"github.com/ben-anass/DouzTach/server/chat"
	"github.com/ben-anass/DouzTach/server/notifications"
	"github.com/ben-anass/DouzTach/server/scrum"
	"github.com/ben-anass/DouzTach/server/users"
	"github.com/ben-anass/DouzTach/server/workspaces"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
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

	// Initialize workspace service
	workspaceService := workspaces.NewService(queries, db)

	// Initialize JWT service
	jwtSecret := os.Getenv("JWT_SECRET")
	if err := validateJWTSecret(jwtSecret); err != nil {
		log.Fatal(err)
	}

	jwtService := auth.NewJWTService(jwtSecret)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(userService, jwtService)
	workspaceHandler := handlers.NewWorkspaceHandler(workspaceService)
	scrumService := scrum.NewService(db)
	scrumHandler := handlers.NewScrumHandler(scrumService)
	activityHandler := handlers.NewActivityHandler(activity.NewService(db))
	notificationHandler := handlers.NewNotificationHandler(notifications.NewService(db))
	chatHandler := handlers.NewChatHandler(chat.NewService(db))

	// Initialize API routes
	router := routes.NewRouter(
		authHandler,
		workspaceHandler,
		scrumHandler,
		activityHandler,
		notificationHandler,
		chatHandler,
		jwtService,
		userService,
	)

	tlsCertFile := strings.TrimSpace(os.Getenv("TLS_CERT_FILE"))
	tlsKeyFile := strings.TrimSpace(os.Getenv("TLS_KEY_FILE"))
	useTLS, err := serverUsesTLS(os.Getenv("APP_ENV"), tlsCertFile, tlsKeyFile)
	if err != nil {
		log.Fatal(err)
	}
	address := serverAddress(useTLS)
	server := &http.Server{
		Addr:    address,
		Handler: router,
	}
	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-stopCtx.Done()
		chatHandler.Close()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown failed: %v", err)
		}
	}()

	var serveErr error
	if useTLS {
		log.Println("DouzTach API starting with HTTPS on", address)
		serveErr = server.ListenAndServeTLS(tlsCertFile, tlsKeyFile)
	} else {
		log.Println("WARNING: plaintext HTTP is bound to loopback for development only")
		log.Println("DouzTach API starting on http://" + address)
		serveErr = server.ListenAndServe()
	}
	chatHandler.Close()
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		log.Fatal(serveErr)
	}
}
