package db

import "github.com/nbyl/metio/internal/dbtypes"

type ServerState = dbtypes.ServerState

const (
	ServerStateStopped  = dbtypes.ServerStateStopped
	ServerStateStarting = dbtypes.ServerStateStarting
	ServerStateRunning  = dbtypes.ServerStateRunning
	ServerStateStopping = dbtypes.ServerStateStopping
)

type Players = dbtypes.Players
type Status = dbtypes.Status
type WhitelistEntry = dbtypes.WhitelistEntry
type WhitelistConfig = dbtypes.WhitelistConfig
