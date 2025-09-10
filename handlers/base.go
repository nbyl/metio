package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
)

func New() *mux.Router {
	r := mux.NewRouter()

	// Static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// Routes
	r.HandleFunc("/", homeHandler).Methods("GET")

	r.HandleFunc("/auth/login", loginHandler).Methods("GET")
	r.HandleFunc("/auth/callback", callbackHandler).Methods("GET")

	serverRouter := r.PathPrefix("/server").Subrouter()
	serverRouter.Use(authMiddleware)

	serverRouter.HandleFunc("/start", startServerHandler).Methods("POST")
	serverRouter.HandleFunc("/stop", stopServerHandler).Methods("POST")
	serverRouter.HandleFunc("/status", statusHandler).Methods("GET")

	return r
}
