package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"amanah/libs/auth"
	"amanah/services/authentication/handlers"
	"amanah/services/authentication/models"
	"amanah/services/authentication/repositories"
	svc "amanah/services/authentication/services"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "OK")
	})

	// simple in-memory setup with a single user
	repo := repositories.NewInMemoryUserRepo()
	repo.AddUser(models.User{ID: "1", Username: "admin", Password: "password"})

	tokenMgr := auth.NewManager()
	service := svc.NewAuthService(repo, tokenMgr)

	mux.Handle("/login", handlers.LoginHandler(service))

	addr := ":8080"
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to signal server errors
	serverErrors := make(chan error, 1)

	// Start server in goroutine
	go func() {
		log.Printf("authentication service listening on %s", addr)
		serverErrors <- server.ListenAndServe()
	}()

	// Channel to listen for shutdown signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a signal or server error
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	case sig := <-shutdown:
		log.Printf("received signal %v, initiating graceful shutdown", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown failed: %v, forcing close", err)
			if closeErr := server.Close(); closeErr != nil {
				log.Fatalf("forced close failed: %v", closeErr)
			}
		}

		log.Println("server shutdown complete")
	}
}
