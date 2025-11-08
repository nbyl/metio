package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	gorillahandlers "github.com/gorilla/handlers"
	"github.com/spf13/viper"
	"gitlab.com/nbyl/metio/handlers"
	"gitlab.com/nbyl/metio/tracing"
)

func main() {
	// Initialize OpenTelemetry
	if err := tracing.InitTracerWithDetails("metio-controller", "1.0.0"); err != nil {
		log.Printf("Failed to initialize tracer: %v", err)
	}
	if err := tracing.InitMetrics(); err != nil {
		log.Printf("Failed to initialize metrics: %v", err)
	}
	defer tracing.ShutdownTracer()

	viper.AutomaticEnv()
	viper.SetDefault("PORT", "8080")

	r := handlers.New()

	port := viper.GetString("PORT")
	log.Printf("Server starting on :%s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), gorillahandlers.LoggingHandler(os.Stdout, r)))
}
