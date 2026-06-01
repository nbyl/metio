package handlers

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"gitlab.com/nbyl/metio/config"
	"gitlab.com/nbyl/metio/db"
	"gitlab.com/nbyl/metio/handlers/servers"
	"gitlab.com/nbyl/metio/handlers/setup"
	"gitlab.com/nbyl/metio/static"
)

var provisioningService servers.ProvisioningServiceInterface

var getDBConnection = func(ctx context.Context) (db.DB, config.Config, error) {
	cfg := config.Load()
	dbConn, err := cfg.NewDBConnection(ctx)
	return dbConn, cfg, err
}

func WriteJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func New(ps servers.ProvisioningServiceInterface, vs setup.ValidationServiceInterface) *mux.Router {
	provisioningService = ps
	setup.ValidationService = vs

	servers.ProvisioningService = ps
	servers.GetDBConnection = getDBConnection
	servers.LookupMinecraftUser = func(ctx context.Context, username string) (*servers.MojangProfile, error) {
		profile, err := LookupMinecraftUser(ctx, username)
		if err != nil {
			return nil, err
		}
		return &servers.MojangProfile{ID: profile.ID, Name: profile.Name}, nil
	}
	servers.GetUserEmail = getUserEmail
	servers.WriteJSONError = WriteJSONError

	r := mux.NewRouter()

	r.Use(TracingMiddleware)

	r.HandleFunc("/auth/login", loginHandler).Methods("GET")
	r.HandleFunc("/auth/callback", callbackHandler).Methods("GET")
	r.HandleFunc("/events", eventsHandler).Methods("POST")
	r.HandleFunc("/api/auth/me", meHandler).Methods("GET")

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
