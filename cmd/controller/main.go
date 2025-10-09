package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	gorillahandlers "github.com/gorilla/handlers"
	"github.com/spf13/viper"
	"gitlab.com/nbyl/metio/handlers"
)

func main() {
	viper.AutomaticEnv()
	viper.SetDefault("PORT", "8080")

	r := handlers.New()

	port := viper.GetString("PORT")
	log.Printf("Server starting on :%s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), gorillahandlers.LoggingHandler(os.Stdout, r)))
}
