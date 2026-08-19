package servers

import "github.com/gorilla/mux"

func RegisterRoutes(api *mux.Router) {
	s := api.PathPrefix("/servers").Subrouter()
	s.HandleFunc("", ListServers).Methods("GET")
	s.HandleFunc("", CreateServer).Methods("POST")
	s.HandleFunc("/{id}", GetServer).Methods("GET")
	s.HandleFunc("/{id}", UpdateServer).Methods("PUT")
	s.HandleFunc("/{id}", DeleteServer).Methods("DELETE")
	s.HandleFunc("/{id}/provisioning", GetServerProvisioningStatus).Methods("GET")
	s.HandleFunc("/{id}/start", StartServerByID).Methods("POST")
	s.HandleFunc("/{id}/stop", StopServerByID).Methods("POST")
	s.HandleFunc("/{id}/status", StatusByID).Methods("GET")
	s.HandleFunc("/{id}/whitelist", GetWhitelistByID).Methods("GET")
	s.HandleFunc("/{id}/whitelist", AddWhitelistByID).Methods("POST")
	s.HandleFunc("/{id}/whitelist/{uuid}", RemoveWhitelistByID).Methods("DELETE")
	s.HandleFunc("/{id}/whitelist/enabled", SetWhitelistEnabledByID).Methods("PUT")
	s.HandleFunc("/{id}/update-agent", HandleUpdateAgent).Methods("POST")
	s.HandleFunc("/{id}/shutdown/schedule", ScheduleShutdownByID).Methods("POST")
	s.HandleFunc("/{id}/shutdown/schedule", CancelScheduledShutdownByID).Methods("DELETE")
	s.HandleFunc("/{id}/settings/backup", GetBackupSettingsByID).Methods("GET")
	s.HandleFunc("/{id}/settings/backup", UpdateBackupSettingsByID).Methods("PUT")

	api.HandleFunc("/options", ListOptions).Methods("GET")
}
