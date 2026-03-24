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

var Version = "dev" // default, overridden by ldflags

func main() {
	// Initialize OpenTelemetry
	if err := tracing.InitTracerWithDetails("metio-controller", Version); err != nil {
		log.Printf("Failed to initialize tracer: %v", err)
	}
	if err := tracing.InitMetrics(); err != nil {
		log.Printf("Failed to initialize metrics: %v", err)
	}
	defer tracing.ShutdownTracer()

	viper.AutomaticEnv()
	viper.SetDefault("PORT", "8080")

	r := handlers.New()

	// Wrap router with CORS middleware (only enabled in dev mode)
	handler := handlers.CORSMiddleware(r)

	port := viper.GetString("PORT")
	log.Printf("Server starting on :%s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), gorillahandlers.LoggingHandler(os.Stdout, handler)))
}
