package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/auth"
	"github.com/priyanshuguptadev/job-board/internal/domain"
)

// ApiKeyAuth authenticates incoming HTTP requests using scoped API keys.
// Supported key formats:
// - Public key: jb_pub_...
// - Admin key: jb_sec_...
// Passed via 'X-API-Key' header or 'Authorization: Bearer <key>' header.
func ApiKeyAuth(repo domain.ApiKeyRepository, requiredScope domain.ApiKeyScope, l *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawKey := auth.ExtractAPIKey(r)
			if rawKey == "" {
				respondJSONError(w, http.StatusUnauthorized, "INVALID_API_KEY", "Missing API key in request headers (X-API-Key or Authorization: Bearer <key>).")
				return
			}

			// Validate key structure/prefix
			_, err := auth.ValidateKeyFormat(rawKey)
			if err != nil {
				respondJSONError(w, http.StatusUnauthorized, "INVALID_API_KEY", "Malformed or invalid API key format.")
				return
			}

			keyHash := auth.HashKey(rawKey)
			apiKey, err := repo.GetByHash(r.Context(), keyHash)
			if err != nil {
				if errors.Is(err, domain.ErrNotFound) {
					respondJSONError(w, http.StatusUnauthorized, "INVALID_API_KEY", "Invalid API key provided.")
					return
				}
				if l != nil {
					l.Error("Failed to lookup API key", "error", err)
				}
				respondJSONError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error occurred while verifying credentials.")
				return
			}

			// Check scope permissions
			if !auth.HasRequiredScope(apiKey.Scope, requiredScope) {
				respondJSONError(w, http.StatusForbidden, "FORBIDDEN", "API key lacks required scope for this resource.")
				return
			}

			// Asynchronously update last_used_at timestamp
			go func(keyID string) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if updateErr := repo.UpdateLastUsed(ctx, keyID, time.Now().UTC()); updateErr != nil && l != nil {
					l.Debug("Failed to update API key last_used_at", "key_id", keyID, "error", updateErr)
				}
			}(apiKey.ID)

			// Store authenticated key in request context
			ctx := auth.ContextWithApiKey(r.Context(), apiKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func respondJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}
