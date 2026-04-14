// Package auth provides background work functions for auth events.
package auth

import (
	"context"
	"fmt"
	"log"
)

// LogLogin logs a login event. Intended as a test job to verify the dispatch mechanism.
// TODO: remove this job once dispatch testing is complete.
func LogLogin(ctx context.Context, payload any) error {
	username, ok := payload.(string)
	if !ok || username == "" {
		return fmt.Errorf("auth: login: missing username")
	}

	log.Printf("auth: login: %s", username)

	return nil
}
