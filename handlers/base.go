package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
)

func New() *mux.Router {
	r := mux.NewRouter()

	// Add tracing middleware to all routes
	r.Use(TracingMiddleware)

	// Static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// Routes
	r.HandleFunc("/", homeHandler).Methods("GET")

	r.HandleFunc("/auth/login", loginHandler).Methods("GET")
	r.HandleFunc("/auth/callback", callbackHandler).Methods("GET")

	// Events endpoint for Pub/Sub push notifications (no auth required)
	r.HandleFunc("/events", eventsHandler).Methods("POST")

	serverRouter := r.PathPrefix("/server").Subrouter()
	serverRouter.Use(authMiddleware)

	serverRouter.HandleFunc("/", serverHandler).Methods("GET")
	serverRouter.HandleFunc("/start", startServerHandler).Methods("POST")
	serverRouter.HandleFunc("/stop", stopServerHandler).Methods("POST")
	serverRouter.HandleFunc("/status", statusHandler).Methods("GET")

	return r
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if !isUserAuthenticated(r) {
		http.Redirect(w, r, "/auth/login", http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, "/server/", http.StatusTemporaryRedirect)
}
