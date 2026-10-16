package main

import (
	"forum-api/internal/handlers"
	"forum-api/internal/middleware"
	"forum-api/internal/repository"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

func main() {
	// Refuse to start with missing or default JWT secret
	if sec := os.Getenv("JWT_SECRET"); sec == "" || sec == "change-this-secret" || sec == "your-secret-key-change-in-production" {
		log.Fatal("JWT_SECRET is unset or uses a default value; set a random secret in .env")
	}

	// Initialize database
	if err := repository.InitDB(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer repository.DB.Close()

	r := mux.NewRouter()

	// Middleware
	r.Use(middleware.NoIndex)
	r.Use(middleware.Logging)
	r.Use(middleware.CORS)
	r.Use(middleware.RateLimit(100))

	// API Routes
	// Registrierung ist bewusst versteckt: der Pfad steht nur in der
	// OpenAPI-Spec (siehe FORUM_OPEN_SOURCE.md). Der Standardpfad
	// /api/auth/register existiert nicht.
	r.HandleFunc("/api/auth/enlist", handlers.AuthHandler).Methods("POST")
	r.HandleFunc("/api/auth/login", handlers.AuthHandler).Methods("POST")
	r.HandleFunc("/api/auth/logout", handlers.AuthHandler).Methods("POST")

	// Protected routes require JWT authentication
	r.Handle("/api/users/me", middleware.JWTAuth(http.HandlerFunc(handlers.GetUser))).Methods("GET")

	// Forum routes - Rechte in den Handlern (admin für Hauptforen,
	// Ersteller/mod/admin für Unterforen), siehe FORUM_STRUKTUR.md
	r.Handle("/api/forums", middleware.JWTAuth(http.HandlerFunc(handlers.ForumHandler))).Methods("GET", "POST")
	r.Handle("/api/forums/{id:[0-9]+}", middleware.JWTAuth(http.HandlerFunc(handlers.ForumHandler))).Methods("GET")
	r.Handle("/api/forums/{id:[0-9]+}", middleware.JWTAuth(http.HandlerFunc(handlers.ForumHandler))).Methods("PUT", "DELETE")

	// Thread routes - Löschen: Ersteller oder mod/admin (im Handler)
	r.Handle("/api/forums/{forumID:[0-9]+}/threads", middleware.JWTAuth(http.HandlerFunc(handlers.ThreadHandler))).Methods("GET", "POST")
	r.Handle("/api/threads/{id:[0-9]+}", middleware.JWTAuth(http.HandlerFunc(handlers.ThreadHandler))).Methods("DELETE")
	r.Handle("/api/threads/{id:[0-9]+}", middleware.JWTAuth(http.HandlerFunc(handlers.ThreadHandler))).Methods("GET")

	// Post routes - Löschen: Ersteller oder mod/admin (im Handler)
	r.Handle("/api/threads/{threadID:[0-9]+}/posts", middleware.JWTAuth(http.HandlerFunc(handlers.PostHandler))).Methods("GET", "POST")
	r.Handle("/api/threads/{threadID:[0-9]+}/posts/{id:[0-9]+}", middleware.JWTAuth(http.HandlerFunc(handlers.PostHandler))).Methods("DELETE")

	r.Handle("/api/messages", middleware.JWTAuth(http.HandlerFunc(handlers.MessageHandler))).Methods("GET", "POST")
	r.Handle("/api/search", middleware.JWTAuth(http.HandlerFunc(handlers.SearchHandler))).Methods("GET")

	// Admin routes - admin only
	r.Handle("/api/admin/users/{id:[0-9]+}/suspend", middleware.RequireRole("admin")(middleware.JWTAuth(http.HandlerFunc(handlers.AdminUserHandler)))).Methods("POST")
	r.Handle("/api/admin/stats", middleware.RequireRole("admin")(middleware.JWTAuth(http.HandlerFunc(handlers.AdminStatsHandler)))).Methods("GET")

	// API documentation (machine-readable spec + human-readable page)
	r.HandleFunc("/openapi.yaml", handlers.OpenAPISpecHandler).Methods("GET")
	r.HandleFunc("/redoc.js", handlers.RedocHandler).Methods("GET")
	r.HandleFunc("/docs", handlers.DocsPage).Methods("GET")

	// Landing Page and crawler directives
	r.HandleFunc("/robots.txt", handlers.RobotsTXT)
	r.HandleFunc("/", handlers.LandingPage)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
