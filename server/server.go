package server

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"gitlab.com/nbyl/metio/views"
)

// var (
// 	serverStatus = views.ServerStatus{
// 		IsOnline:   false,
// 		Players:    0,
// 		MaxPlayers: 20,
// 		Uptime:     "00:00:00",
// 		Version:    "1.20.4",
// 		IP:         "mc.metio.server:25565",
// 	}
// )

func RunServer() {
	r := mux.NewRouter()

	// Static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// Routes
	r.HandleFunc("/", homeHandler).Methods("GET")
	r.HandleFunc("/server/start", startServerHandler).Methods("POST")
	r.HandleFunc("/server/stop", stopServerHandler).Methods("POST")
	r.HandleFunc("/server/status", statusHandler).Methods("GET")

	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", handlers.LoggingHandler(os.Stdout, r)))
}
func homeHandler(w http.ResponseWriter, r *http.Request) {
	serverStatus, err := getServerStatus()
	if err != nil {
		log.Print(err)
	}

	component := views.HomePage(*serverStatus)
	component.Render(r.Context(), w)
}

func startServerHandler(w http.ResponseWriter, r *http.Request) {
	// Simulate startup delay
	time.Sleep(3 * time.Second)

	serverStatus := views.ServerStatus{
		IsOnline:   false,
		Players:    0,
		MaxPlayers: 20,
		Uptime:     "00:00:00",
		Version:    "1.20.4",
		IP:         "mc.metio.server:25565",
	}

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

	serverStatus := views.ServerStatus{
		IsOnline:   false,
		Players:    0,
		MaxPlayers: 20,
		Uptime:     "00:00:00",
		Version:    "1.20.4",
		IP:         "mc.metio.server:25565",
	}

	serverStatus.IsOnline = false
	serverStatus.Players = 0
	serverStatus.Uptime = "00:00:00"

	component := views.ServerStatusCard(serverStatus)
	component.Render(r.Context(), w)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	serverStatus, err := getServerStatus()
	if err != nil {
		log.Print(err)
	}
	component := views.ServerStatusCard(*serverStatus)
	component.Render(r.Context(), w)
}

func getServerStatus() (*views.ServerStatus, error) {
	ctx := context.Background()
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	req := &computepb.GetInstanceRequest{
		Instance: os.Getenv("INSTANCE_NAME"),
		Project:  os.Getenv("GCP_PROJECT"),
		Zone:     os.Getenv("GCP_ZONE"),
	}

	resp, err := c.Get(ctx, req)
	log.Print(*resp.Status == "RUNNING")
	if err != nil {
		return nil, err
	}

	return &views.ServerStatus{
		Status:     *resp.Status,
		IsOnline:   *resp.Status == "RUNNING",
		Players:    0,
		MaxPlayers: 20,
		Uptime:     "TBD",
		Version:    "TBD",
		IP:         fmt.Sprintf("%s:25565", *resp.NetworkInterfaces[0].AccessConfigs[0].NatIP),
	}, nil
}
