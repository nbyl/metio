package main

import (
	"log"

	"github.com/joho/godotenv"
	"gitlab.com/nbyl/metio/server"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Print("Could not load .env file")
	}
	server.RunServer()
}
