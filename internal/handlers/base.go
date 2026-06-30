package handlers

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/config"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/handlers/servers"
	"github.com/nbyl/metio/internal/handlers/setup"
	"github.com/nbyl/metio/internal/handlers/tasks"
	"github.com/nbyl/metio/internal/services"
	"github.com/nbyl/metio/static"
)

var provisioningService servers.ProvisioningServiceInterface

var getDBConnection = func(ctx context.Context) (db.DB, config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, config.Config{}, err
	}
	dbConn, err := cfg.NewDBConnection(ctx)
	return dbConn, cfg, err
}

func WriteJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func New(ps servers.ProvisioningServiceInterface, vs setup.ValidationServiceInterface, ss setup.SetupServiceInterface, provisioningSvc *services.ProvisioningService, cfg *config.Config) *mux.Router {
	provisioningService = ps
	setup.ValidationService = vs
	setup.SetupService = ss

	servers.ProvisioningService = ps
	servers.GetDBConnection = getDBConnection
	servers.LookupMinecraftUser = services.LookupMinecraftUser
	servers.GetUserEmail = getUserEmail
	servers.WriteJSONError = WriteJSONError

	r := mux.NewRouter()

	r.Use(TracingMiddleware)

	r.HandleFunc("/auth/login", loginHandler).Methods("GET")
	r.HandleFunc("/auth/callback", callbackHandler).Methods("GET")
	r.HandleFunc("/events", eventsHandler).Methods("POST")
	r.HandleFunc("/api/auth/me", meHandler).Methods("GET")

	if provisioningSvc != nil && cfg != nil && cfg.OperationMode == "cloudtasks" {
		tasks.ProvisioningService = provisioningSvc
		tasks.RegisterTaskRoutes(r)
	}

	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.Use(apiAuthMiddleware)

	servers.RegisterRoutes(apiRouter)

	setup.RegisterRoutes(apiRouter)

	r.PathPrefix("/").Handler(spaHandler())
	return r
}

func spaHandler() http.Handler {
	var fileSystem fs.FS
	var err error

	if os.Getenv("DEV_MODE") == "true" {
		fileSystem = os.DirFS("static/dist")
	} else {
		fileSystem, err = fs.Sub(static.DistFS, "dist")
		if err != nil {
			panic("failed to get embedded dist filesystem: " + err.Error())
		}
	}

	fileServer := http.FileServer(http.FS(fileSystem))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		f, err := fileSystem.Open(path)
		if err != nil {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()

		fileServer.ServeHTTP(w, r)
	})
}
