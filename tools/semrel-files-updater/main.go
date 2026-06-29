package main

import (
	tfvarsUpdater "github.com/nbyl/metio/tools/semrel-files-updater/pkg/tfvars"
	"github.com/go-semantic-release/semantic-release/v2/pkg/plugin"
	"github.com/go-semantic-release/semantic-release/v2/pkg/updater"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		FilesUpdater: func() updater.FilesUpdater {
			return &tfvarsUpdater.Updater{}
		},
	})
}
