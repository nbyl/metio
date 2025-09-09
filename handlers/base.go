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
	r.HandleFunc("/server/start", startServerHandler).Methods("POST")
	r.HandleFunc("/server/stop", stopServerHandler).Methods("POST")
	r.HandleFunc("/server/status", statusHandler).Methods("GET")

	return r
}
