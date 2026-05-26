package handlers

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"gitlab.com/nbyl/metio/config"
	"gitlab.com/nbyl/metio/db"
	"gitlab.com/nbyl/metio/pulumi/programs"
	"gitlab.com/nbyl/metio/static"
)

// UpdateType classifies what kind of update is being performed, which determines
// the provisioning workflow steps (e.g., stop VM, save world, etc.).
type UpdateType int

const (
	UpdateTypeInPlace   UpdateType = iota // Fields that can be updated without VM disruption
	UpdateTypeResize                      // Machine type change (stop -> up -> start)
	UpdateTypeRecreate                    // Minecraft version change (backup -> up -> start)
)

// ProvisioningServiceInterface defines the methods used by handlers from the provisioning service.
type ProvisioningServiceInterface interface {
	CreateServer(ctx context.Context, serverID string, config *programs.ServerConfig) error
	UpdateServer(ctx context.Context, serverID string, config *programs.ServerConfig, updateType int) error
	DestroyServer(ctx context.Context, serverID string) error
	GetProvisioningStatus(ctx context.Context, serverID string) (*db.ProvisioningStatus, error)
	RevertServerConfig(ctx context.Context, serverID string) error
}

var provisioningService ProvisioningServiceInterface

var validationService ValidationServiceInterface

// getDBConnection is a function variable that returns a DB connection.
// Override in tests to inject a mock DB.
var getDBConnection = func(ctx context.Context) (db.DB, config.Config, error) {
	cfg := config.Load()
	dbConn, err := cfg.NewDBConnection(ctx)
	return dbConn, cfg, err
}

func New(ps ProvisioningServiceInterface, vs ValidationServiceInterface) *mux.Router {
	provisioningService = ps
	validationService = vs
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

	apiRouter.HandleFunc("/servers", listServers).Methods("GET")
	apiRouter.HandleFunc("/servers", createServer).Methods("POST")
	apiRouter.HandleFunc("/servers/{id}", getServer).Methods("GET")
	apiRouter.HandleFunc("/servers/{id}", updateServer).Methods("PUT")
	apiRouter.HandleFunc("/servers/{id}", deleteServer).Methods("DELETE")
	apiRouter.HandleFunc("/servers/{id}/provisioning", getServerProvisioningStatus).Methods("GET")

	apiRouter.HandleFunc("/setup/validate", validateSetupHandler).Methods("GET")

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
