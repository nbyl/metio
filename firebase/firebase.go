package firebase

import (
	"context"
	"sync"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

var (
	firebaseApp  *firebase.App
	firebaseAuth *auth.Client
	initOnce     sync.Once
	initErr      error
)

// Initialize sets up the Firebase Admin SDK
// Uses Application Default Credentials (ADC) which works automatically in GCP
func Initialize(ctx context.Context) error {
	initOnce.Do(func() {
		firebaseApp, initErr = firebase.NewApp(ctx, nil)
		if initErr != nil {
			return
		}

		firebaseAuth, initErr = firebaseApp.Auth(ctx)
	})

	return initErr
}

// GetAuth returns the Firebase Auth client
func GetAuth() *auth.Client {
	return firebaseAuth
}

// CreateCustomToken generates a Firebase custom token for the given user ID
// This token can be used by the frontend to sign in with Firebase
func CreateCustomToken(ctx context.Context, userID string) (string, error) {
	if firebaseAuth == nil {
		if err := Initialize(ctx); err != nil {
			return "", err
		}
	}

	return firebaseAuth.CustomToken(ctx, userID)
}
