package main

import (
	"log"
	"net/http"

	"amanah/services/api-gateway/handlers"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	handlers.RegisterRoutes(mux)

	addr := ":8000"
	log.Printf("api-gateway listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
