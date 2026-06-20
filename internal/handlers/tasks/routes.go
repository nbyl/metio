package tasks

import "github.com/gorilla/mux"

func RegisterTaskRoutes(r *mux.Router) {
	s := r.PathPrefix("/tasks").Subrouter()
	s.HandleFunc("/provision/{id}", HandleProvisioningTask).Methods("POST")
}
