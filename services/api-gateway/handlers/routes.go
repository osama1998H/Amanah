package handlers

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// RegisterRoutes registers the gateway routes.
func RegisterRoutes(mux *http.ServeMux) {
	// TODO: Add authentication, rate limiting, etc.
	mux.HandleFunc("/transactions/", proxy("http://transaction:8080"))
	// Add more routes to other services here.
}

// proxy returns a handler that proxies requests to the target service.
func proxy(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url, err := url.Parse(target)
		if err != nil {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		p := httputil.NewSingleHostReverseProxy(url)
		p.ServeHTTP(w, r)
	}
}
