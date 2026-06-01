package handlers

import (
	"net/http"
	"os"

	gorillahandlers "github.com/gorilla/handlers"
)

// CORSMiddleware returns a CORS handler wrapper that is environment-aware.
// In development mode (DEV_MODE=true), it allows cross-origin requests
// from the Vite dev server (localhost:5173). In production, CORS is
// disabled and only same-origin requests are allowed.
func CORSMiddleware(h http.Handler) http.Handler {
	if os.Getenv("DEV_MODE") != "true" {
		// Production: no CORS, same-origin only
		return h
	}

	// Development: allow Vite dev server
	return gorillahandlers.CORS(
		gorillahandlers.AllowedOrigins([]string{"http://localhost:5173"}),
		gorillahandlers.AllowedMethods([]string{"GET", "POST", "OPTIONS"}),
		gorillahandlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
		gorillahandlers.AllowCredentials(),
	)(h)
}
