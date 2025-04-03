package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"nwr/middleware"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Health check handler.
func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprint(w, "200")
}

// Middleware for security headers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "max-age=0, no-cache, no-store, must-revalidate, private")
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Initialize MongoDB.
	var err error
	mongoClient, err = mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	// Choose your database and collections.
	db = mongoClient.Database("chatxapp")
	chatsCollection = db.Collection("chats")
	messagesCollection = db.Collection("messages")

	// Initialize Redis.
	redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	if err = redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Start the background flushing process.
	go flushRedisMessages()

	router := httprouter.New()

	// Health check.
	router.GET("/health", Index)

	// Existing endpoints.
	router.GET("/api/contacts", middleware.Authenticate(contactsHandler))
	router.GET("/api/chats", middleware.Authenticate(chatsHandler))
	router.GET("/api/messages", middleware.Authenticate(messagesHandler))
	router.POST("/api/messages/send", middleware.Authenticate(sendMessageHandler))
	router.PUT("/api/messages/edit", middleware.Authenticate(editMessageHandler))
	router.DELETE("/api/messages/delete", middleware.Authenticate(deleteMessageHandler))
	router.DELETE("/api/chats/:chatid", middleware.Authenticate(deleteChatHandler))
	router.GET("/ws", wsHandler)

	// Register the new create chat endpoint.
	router.POST("/api/chats/create", createChatHandler)

	// CORS setup.
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})
	handler := securityHeaders(c.Handler(router))

	// Serve uploaded files.
	router.ServeFiles("/uploads/*filepath", http.Dir("uploads"))

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	// Start the server in a goroutine.
	go func() {
		log.Println("Server started on port 8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on port 8080: %v", err)
		}
	}()

	// Graceful shutdown.
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)
	<-shutdownChan
	log.Println("Shutting down gracefully...")
	if err := server.Shutdown(context.Background()); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
	log.Println("Server stopped")
}
