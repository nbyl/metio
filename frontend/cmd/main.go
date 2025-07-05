package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/nbyl/metio/frontend/views"
)

type ServerStatus struct {
	IsOnline   bool      `json:"isOnline"`
	Players    int       `json:"players"`
	MaxPlayers int       `json:"maxPlayers"`
	Uptime     string    `json:"uptime"`
	Version    string    `json:"version"`
	IP         string    `json:"ip"`
	StartTime  time.Time `json:"-"`
}

type AccessLink struct {
	URL    string    `json:"url"`
	Expiry time.Time `json:"expiry"`
}

var (
	serverStatus = ServerStatus{
		IsOnline:   false,
		Players:    0,
		MaxPlayers: 20,
		Uptime:     "00:00:00",
		Version:    "1.20.4",
		IP:         "mc.metio.server:25565",
	}
	currentAccessLink *AccessLink
)

func main() {
	r := mux.NewRouter()

	// Static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// Routes
	r.HandleFunc("/", homeHandler).Methods("GET")
	r.HandleFunc("/server/start", startServerHandler).Methods("POST")
	r.HandleFunc("/server/stop", stopServerHandler).Methods("POST")
	r.HandleFunc("/server/status", statusHandler).Methods("GET")
	r.HandleFunc("/access/generate", generateAccessHandler).Methods("POST")

	// Start uptime ticker
	go uptimeTicker()

	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func uptimeTicker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if serverStatus.IsOnline {
			duration := time.Since(serverStatus.StartTime)
			hours := int(duration.Hours())
			minutes := int(duration.Minutes()) % 60
			seconds := int(duration.Seconds()) % 60
			serverStatus.Uptime = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
		}
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	component := views.HomePage(serverStatus, currentAccessLink)
	component.Render(r.Context(), w)
}

func startServerHandler(w http.ResponseWriter, r *http.Request) {
	// Simulate startup delay
	time.Sleep(3 * time.Second)

	serverStatus.IsOnline = true
	serverStatus.StartTime = time.Now()
	serverStatus.Players = rand.Intn(5)
	serverStatus.Uptime = "00:00:00"

	component := views.ServerStatusCard(serverStatus)
	component.Render(r.Context(), w)
}

func stopServerHandler(w http.ResponseWriter, r *http.Request) {
	// Simulate shutdown delay
	time.Sleep(2 * time.Second)

	serverStatus.IsOnline = false
	serverStatus.Players = 0
	serverStatus.Uptime = "00:00:00"

	component := views.ServerStatusCard(serverStatus)
	component.Render(r.Context(), w)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	component := views.ServerStatusCard(serverStatus)
	component.Render(r.Context(), w)
}

func generateAccessHandler(w http.ResponseWriter, r *http.Request) {
	linkId := fmt.Sprintf("%d", rand.Int63())
	expiry := time.Now().Add(24 * time.Hour)

	currentAccessLink = &AccessLink{
		URL:    fmt.Sprintf("https://metio.app/join/%s", linkId),
		Expiry: expiry,
	}

	component := views.AccessLinkSection(currentAccessLink)
	component.Render(r.Context(), w)
}
