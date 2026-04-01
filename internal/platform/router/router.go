// Package router defines shared routing types for the platform layer.
package router

import (
	chi "github.com/go-chi/chi/v5"
)

// RouteRegistrar is a function that registers routes on a chi.Router.
// Each module exposes a Routes() function returning this type so that
// route paths are owned by the module rather than the central wiring layer.
//
// Type alias (not a new type) so callers can return func(chi.Router)
// without importing this package.
type RouteRegistrar = func(chi.Router)
