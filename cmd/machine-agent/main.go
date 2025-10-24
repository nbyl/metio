package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/spf13/viper"
	"gitlab.com/nbyl/metio/db"
)

var execCommand = exec.Command
var getMinecraftPlayerCountFunc = getMinecraftPlayerCount

func main() {
	viper.SetDefault("MINECRAFT_CHECK_INTERVAL", "30s")
	viper.AutomaticEnv()

	intervalStr := viper.GetString("MINECRAFT_CHECK_INTERVAL")
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		log.Fatalf("Invalid interval format: %v", err)
	}

	environment := viper.GetString("ENVIRONMENT")
	if environment == "" {
		environment = "dev"
	}
	region := viper.GetString("REGION")
	if region == "" {
		region = "us-central1"
	}
	instanceName := viper.GetString("INSTANCE_NAME")
	if instanceName == "" {
		instanceName = "minecraft-server"
	}

	projectID, err := metadata.ProjectID()
	if err != nil {
		log.Fatalf("Error getting project ID: %v", err)
	}

	ctx := context.Background()
	databaseID := fmt.Sprintf("%s-%s-metio-db", environment, region)
	dbConn, err := db.NewConnection(ctx, projectID, databaseID)
	if err != nil {
		log.Fatalf("Error creating Firestore client: %v", err)
	}

	fmt.Printf("Machine agent started with check interval: %v\n", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			if err := runStatusUpdate(ctx, dbConn, instanceName); err != nil {
				log.Printf("Error in status update: %v", err)
			}
		}
	}()

	// Keep the program running
	select {}
}

func runStatusUpdate(ctx context.Context, dbConn db.DB, instanceName string) error {
	current, max, err := getMinecraftPlayerCountFunc()
	if err != nil {
		return err
	}
	return dbConn.UpdateStatus(ctx, instanceName, db.Status{
		Players:   db.Players{Current: current, Max: max},
		Timestamp: time.Now(),
	})
}

func getMinecraftPlayerCount() (int, int, error) {
	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "rcon-cli", "list")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	// Parse output like "There are 2 of a max of 20 players online: Steve, Alex"
	re := regexp.MustCompile(`There are (\d+) of a max of (\d+) players online`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 3 {
		return 0, 0, fmt.Errorf("could not parse player count from output: %s", string(output))
	}

	current, _ := strconv.Atoi(matches[1])
	max, _ := strconv.Atoi(matches[2])
	return current, max, nil
}
