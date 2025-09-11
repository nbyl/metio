package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	gorillahandlers "github.com/gorilla/handlers"
	"gitlab.com/nbyl/metio/handlers"
)

func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		value = fallback
	}
	return value
}

func main() {
	r := handlers.New()

	port := getEnv("PORT", "8080")
	log.Printf("Server starting on :%s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), gorillahandlers.LoggingHandler(os.Stdout, r)))
}
