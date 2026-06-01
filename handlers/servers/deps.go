package servers

import (
	"context"
	"net/http"

	"gitlab.com/nbyl/metio/config"
	"gitlab.com/nbyl/metio/db"
	"gitlab.com/nbyl/metio/services"
)

var GetDBConnection func(ctx context.Context) (db.DB, config.Config, error)

var ProvisioningService ProvisioningServiceInterface

var LookupMinecraftUser func(ctx context.Context, username string) (*services.MojangProfile, error)

var GetUserEmail func(r *http.Request) string

var WriteJSONError func(w http.ResponseWriter, message string, statusCode int)
