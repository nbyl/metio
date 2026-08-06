package db

import "errors"

// ErrNotFound is returned by DB implementations when a requested record does
// not exist in the state store. Callers should use errors.Is to detect it.
var ErrNotFound = errors.New("not found")
