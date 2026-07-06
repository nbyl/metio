package agent

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

func AgentAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "missing authorization header"})
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid authorization format"})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		instanceName, err := VerifyToken(tokenStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid or expired token"})
			return
		}

		vars := mux.Vars(r)
		pathInstance := vars["instance"]
		if instanceName != pathInstance {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "token does not match instance"})
			return
		}

		next.ServeHTTP(w, r)
	})
}
