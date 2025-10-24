package db

import "time"

type Players struct {
	Current int `firestore:"current"`
	Max     int `firestore:"max"`
}

type Status struct {
	Players   Players   `firestore:"players"`
	Timestamp time.Time `firestore:"timestamp"`
}
