// internal/ghmcp/auth.go
package ghmcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

// UserContext holds user information extracted from gateway headers
type UserContext struct {
	UserID    string
	Email     string
	Name      string
	SessionID string
	Token     string
	RequestID string
}

// contextKey is a type for context keys to avoid collisions
type contextKey string

const (
	userContextKey contextKey = "user_context"
)

// extractUserContext extracts user information from HTTP headers
func extractUserContext(r *http.Request) (*UserContext, error) {
	// Extract Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing Authorization header")
	}

	// Extract Bearer token
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("invalid Authorization header format")
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Extract user context headers
	userID := r.Header.Get("X-User-ID")
	email := r.Header.Get("X-User-Email")
	name := r.Header.Get("X-User-Name")
	sessionID := r.Header.Get("X-Session-ID")
	requestID := r.Header.Get("X-Gateway-Request-ID")

	if userID == "" || email == "" {
		return nil, fmt.Errorf("missing required user context headers (X-User-ID or X-User-Email)")
	}

	return &UserContext{
		UserID:    userID,
		Email:     email,
		Name:      name,
		SessionID: sessionID,
		Token:     token,
		RequestID: requestID,
	}, nil
}

// GetUserContext retrieves user context from the request context
func GetUserContext(ctx context.Context) (*UserContext, bool) {
	userCtx, ok := ctx.Value(userContextKey).(*UserContext)
	return userCtx, ok
}

// WithUserContext adds user context to the request context
func WithUserContext(ctx context.Context, userCtx *UserContext) context.Context {
	return context.WithValue(ctx, userContextKey, userCtx)
}

// AuthenticationMiddleware extracts user context from headers and adds to request context
func AuthenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log incoming request headers for debugging
		logrus.WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
			"remote": r.RemoteAddr,
		}).Debug("Incoming request")

		// Extract user context from headers
		userCtx, err := extractUserContext(r)
		if err != nil {
			// Log the error with request details
			logrus.WithFields(logrus.Fields{
				"error":      err.Error(),
				"path":       r.URL.Path,
				"user_agent": r.Header.Get("User-Agent"),
			}).Warn("Authentication extraction failed")

			// Return 401 Unauthorized for missing or invalid authentication
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"authentication required","message":"` + err.Error() + `"}`))
			return
		}

		// Log successful authentication
		logrus.WithFields(logrus.Fields{
			"user_id":    userCtx.UserID,
			"user_email": userCtx.Email,
			"session_id": userCtx.SessionID,
			"request_id": userCtx.RequestID,
		}).Info("Authenticated request")

		// Add user context to request context
		ctx := WithUserContext(r.Context(), userCtx)
		r = r.WithContext(ctx)

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}

// OptionalAuthenticationMiddleware extracts user context but allows unauthenticated requests
func OptionalAuthenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract user context from headers
		userCtx, err := extractUserContext(r)
		if err != nil {
			// Log the warning but continue without user context
			logrus.WithFields(logrus.Fields{
				"error": err.Error(),
				"path":  r.URL.Path,
			}).Debug("No authentication context, continuing without user context")

			// Continue without user context
			next.ServeHTTP(w, r)
			return
		}

		// Log successful authentication
		logrus.WithFields(logrus.Fields{
			"user_id":    userCtx.UserID,
			"user_email": userCtx.Email,
		}).Debug("Authenticated request")

		// Add user context to request context
		ctx := WithUserContext(r.Context(), userCtx)
		r = r.WithContext(ctx)

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}
