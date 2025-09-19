package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/spf13/viper"
	"gitlab.com/nbyl/metio/views"
)

func serverHandler(w http.ResponseWriter, r *http.Request) {
	serverStatus, err := getServerStatus()
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	component := views.ServerPage(*serverStatus)
	component.Render(r.Context(), w)
}

func startServerHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer c.Close()

	req := &computepb.StartInstanceRequest{
		Instance: viper.GetString("INSTANCE_NAME"),
		Project:  viper.GetString("GCP_PROJECT"),
		Zone:     viper.GetString("GCP_ZONE"),
	}

	_, err = c.Start(ctx, req)
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/server", http.StatusSeeOther)
}

func stopServerHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer c.Close()

	req := &computepb.StopInstanceRequest{
		Instance: viper.GetString("INSTANCE_NAME"),
		Project:  viper.GetString("GCP_PROJECT"),
		Zone:     viper.GetString("GCP_ZONE"),
	}

	_, err = c.Stop(ctx, req)
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/server", http.StatusSeeOther)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	serverStatus, err := getServerStatus()
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
		Instance: viper.GetString("INSTANCE_NAME"),
		Project:  viper.GetString("GCP_PROJECT"),
		Zone:     viper.GetString("GCP_ZONE"),
	}

	resp, err := c.Get(ctx, req)
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
