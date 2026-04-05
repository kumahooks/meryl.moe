// Package auth defines the authenticated user type and context accessors.
package auth

import (
	"context"
)

type authContextKey struct{}

// User holds the authenticated user's identity injected into the request context.
type User struct {
	ID       string
	Username string
}

// WithUser returns a new context with the given User stored under the auth key.
func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, authContextKey{}, user)
}

// AuthUser returns the authenticated User from the request context.
// The second return value is false if no authenticated user is present.
func AuthUser(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(authContextKey{}).(User)

	return user, ok
}
