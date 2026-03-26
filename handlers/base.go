package handlers

import (
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"gitlab.com/nbyl/metio/static"
)

func New() *mux.Router {
	r := mux.NewRouter()

	// Add tracing middleware to all routes
	r.Use(TracingMiddleware)

	// Auth routes
	r.HandleFunc("/auth/login", loginHandler).Methods("GET")
	r.HandleFunc("/auth/callback", callbackHandler).Methods("GET")

	// Events endpoint for Pub/Sub push notifications (no auth required)
	r.HandleFunc("/events", eventsHandler).Methods("POST")

	// Config endpoint (no auth required - needed for frontend Firebase initialization)
	r.HandleFunc("/api/config", configHandler).Methods("GET")

	// Auth status endpoint (no auth required - used by frontend to check auth state)
	r.HandleFunc("/api/auth/me", meHandler).Methods("GET")

	// API routes (protected with JSON 401 responses)
	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.Use(apiAuthMiddleware)
	apiRouter.HandleFunc("/server/start", startServerHandler).Methods("POST")
	apiRouter.HandleFunc("/server/stop", stopServerHandler).Methods("POST")
	apiRouter.HandleFunc("/server/status", statusHandler).Methods("GET")
	apiRouter.HandleFunc("/server/whitelist", getWhitelistHandler).Methods("GET")
	apiRouter.HandleFunc("/server/whitelist", addWhitelistHandler).Methods("POST")
	apiRouter.HandleFunc("/server/whitelist/{uuid}", removeWhitelistHandler).Methods("DELETE")
	apiRouter.HandleFunc("/server/whitelist/enabled", setWhitelistEnabledHandler).Methods("PUT")
	apiRouter.HandleFunc("/server/shutdown/schedule", scheduleShutdownHandler).Methods("POST")
	apiRouter.HandleFunc("/server/shutdown/schedule", cancelScheduledShutdownHandler).Methods("DELETE")

	// SPA fallback - serve React app for all other routes
	r.PathPrefix("/").Handler(spaHandler())

	return r
}

// spaHandler serves static files from embedded FS (or filesystem in dev mode),
// falling back to index.html for client-side routing
func spaHandler() http.Handler {
	var fileSystem fs.FS
	var err error

	// Check for dev mode - serve from filesystem instead of embedded
	if os.Getenv("DEV_MODE") == "true" {
		fileSystem = os.DirFS("static/dist")
	} else {
		// Use embedded filesystem
		fileSystem, err = fs.Sub(static.DistFS, "dist")
		if err != nil {
			panic("failed to get embedded dist filesystem: " + err.Error())
		}
	}

	fileServer := http.FileServer(http.FS(fileSystem))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the path
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try to open the file to check if it exists
		f, err := fileSystem.Open(path)
		if err != nil {
			// File doesn't exist - serve index.html for SPA routing
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()

		// File exists - serve it
		fileServer.ServeHTTP(w, r)
	})
}
