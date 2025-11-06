package db

import "time"

type ServerState string

const (
	ServerStateStopped  ServerState = "STOPPED"
	ServerStateStarting ServerState = "STARTING"
	ServerStateRunning  ServerState = "RUNNING"
	ServerStateStopping ServerState = "STOPPING"
)

func (s ServerState) String() string {
	return string(s)
}

func (s ServerState) IsRunning() bool {
	return s == ServerStateRunning
}

func (s ServerState) IsStopped() bool {
	return s == ServerStateStopped
}

func (s ServerState) IsTransitioning() bool {
	return s == ServerStateStarting || s == ServerStateStopping
}

type Players struct {
	Current int `firestore:"current"`
	Max     int `firestore:"max"`
}

type Status struct {
	Players     Players     `firestore:"players"`
	Timestamp   time.Time   `firestore:"timestamp"`
	Uptime      string      `firestore:"uptime"`
	ServerState ServerState `firestore:"server_state"`
	InstanceIP  string      `firestore:"instance_ip"`
}
