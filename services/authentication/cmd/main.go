package main

import (
	"fmt"
	"log"
	"net/http"

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
	log.Printf("authentication service listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
