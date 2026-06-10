package setup

import "github.com/gorilla/mux"

func RegisterRoutes(api *mux.Router) {
	api.HandleFunc("/setup/validate", ValidateSetupHandler).Methods("GET")
	api.HandleFunc("/setup/status", StatusHandler).Methods("GET")
	api.HandleFunc("/setup/initialize", InitializeHandler).Methods("POST")
}
