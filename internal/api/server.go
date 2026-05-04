package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"windowsuseruptimecontrol/internal/model"
)

type AdminController interface {
	State() model.StateFile
	LookupUser(user string) (model.UserDayState, error)
	ConfigView() map[string]any
	AdjustUser(user string, delta int64) (model.UserDayState, error)
	SetAllowance(user string, sec int64) (model.UserDayState, error)
	ResetToday(user string) (model.UserDayState, error)
	Announce(msg string) error
	HibernateNow() error
}

type Logger interface {
	APIf(format string, args ...any)
	Recent(limit int) ([]string, error)
}

type Server struct {
	token string
	admin AdminController
	log   Logger
	mux   *http.ServeMux
}

type endpointInfo struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Auth        string `json:"auth"`
	Description string `json:"description"`
	Example     any    `json:"example"`
}

func New(token string, admin AdminController, logger Logger) *Server {
	s := &Server{
		token: token,
		admin: admin,
		log:   logger,
		mux:   http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		s.logRequest(r, http.StatusOK)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	s.mux.HandleFunc("/v1/config", func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			s.logRequest(r, http.StatusUnauthorized)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		s.logRequest(r, http.StatusOK)
		writeJSON(w, http.StatusOK, s.admin.ConfigView())
	})

	s.mux.HandleFunc("/v1/info", func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			s.logRequest(r, http.StatusUnauthorized)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			s.logRequest(r, http.StatusMethodNotAllowed)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.logRequest(r, http.StatusOK)
		writeJSON(w, http.StatusOK, map[string]any{
			"service":     "windowsuseruptimecontrol",
			"api_version": "v1",
			"note":        "Use the Authorization header with your bearer token. The /v1/config response is sanitized and does not expose the raw bearer token.",
			"endpoints":   infoEndpoints(),
		})
	})

	s.mux.HandleFunc("/v1/users", func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			s.logRequest(r, http.StatusUnauthorized)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			s.logRequest(r, http.StatusMethodNotAllowed)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		state := s.admin.State()
		users := make([]model.UserDayState, 0, len(state.Users))
		for _, user := range state.Users {
			users = append(users, user)
		}
		s.logRequest(r, http.StatusOK)
		writeJSON(w, http.StatusOK, users)
	})

	s.mux.HandleFunc("/v1/users/", func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			s.logRequest(r, http.StatusUnauthorized)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/v1/users/")
		parts := strings.Split(path, "/")
		if len(parts) != 2 {
			s.logRequest(r, http.StatusNotFound)
			http.NotFound(w, r)
			return
		}

		userID := parts[0]
		action := parts[1]

		switch {
		case r.Method == http.MethodPost && action == "adjust":
			var req struct {
				DeltaSec int64 `json:"delta_sec"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				s.logRequest(r, http.StatusBadRequest)
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			user, err := s.admin.AdjustUser(userID, req.DeltaSec)
			if err != nil {
				s.logRequest(r, http.StatusBadRequest)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			s.logRequest(r, http.StatusOK)
			writeJSON(w, http.StatusOK, user)
		case r.Method == http.MethodPost && action == "allowance":
			var req struct {
				DailyAllowanceSec int64 `json:"daily_allowance_sec"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				s.logRequest(r, http.StatusBadRequest)
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			user, err := s.admin.SetAllowance(userID, req.DailyAllowanceSec)
			if err != nil {
				s.logRequest(r, http.StatusBadRequest)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			s.logRequest(r, http.StatusOK)
			writeJSON(w, http.StatusOK, user)
		case r.Method == http.MethodPost && action == "reset-today":
			user, err := s.admin.ResetToday(userID)
			if err != nil {
				s.logRequest(r, http.StatusBadRequest)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			s.logRequest(r, http.StatusOK)
			writeJSON(w, http.StatusOK, user)
		case r.Method == http.MethodGet && action == "status":
			user, err := s.admin.LookupUser(userID)
			if err != nil {
				s.logRequest(r, http.StatusNotFound)
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			s.logRequest(r, http.StatusOK)
			writeJSON(w, http.StatusOK, user)
		default:
			s.logRequest(r, http.StatusNotFound)
			http.NotFound(w, r)
		}
	})

	s.mux.HandleFunc("/v1/enforcement/hibernate-now", func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			s.logRequest(r, http.StatusUnauthorized)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			s.logRequest(r, http.StatusMethodNotAllowed)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := s.admin.HibernateNow(); err != nil {
			s.logRequest(r, http.StatusBadRequest)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.logRequest(r, http.StatusOK)
		writeJSON(w, http.StatusOK, map[string]string{"status": "hibernation triggered"})
	})

	s.mux.HandleFunc("/v1/announce", func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			s.logRequest(r, http.StatusUnauthorized)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			s.logRequest(r, http.StatusMethodNotAllowed)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Message) == "" {
			s.logRequest(r, http.StatusBadRequest)
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.admin.Announce(req.Message); err != nil {
			s.logRequest(r, http.StatusBadRequest)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.logRequest(r, http.StatusOK)
		writeJSON(w, http.StatusOK, map[string]string{"status": "announced"})
	})

	s.mux.HandleFunc("/v1/logs/recent", func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			s.logRequest(r, http.StatusUnauthorized)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		lines, err := s.log.Recent(100)
		if err != nil {
			s.logRequest(r, http.StatusInternalServerError)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.logRequest(r, http.StatusOK)
		writeJSON(w, http.StatusOK, map[string]any{"lines": lines})
	})
}

func (s *Server) authorized(r *http.Request) bool {
	return r.Header.Get("Authorization") == "Bearer "+s.token
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Server) logRequest(r *http.Request, status int) {
	if s.log != nil {
		s.log.APIf("%s %s remote=%s status=%d", r.Method, r.URL.Path, r.RemoteAddr, status)
	}
}

func infoEndpoints() []endpointInfo {
	return []endpointInfo{
		{
			Method:      http.MethodGet,
			Path:        "/v1/health",
			Auth:        "none",
			Description: "Service health check.",
			Example: map[string]any{
				"request":  "curl http://localhost:8111/v1/health",
				"response": map[string]string{"status": "ok"},
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/v1/info",
			Auth:        "bearer",
			Description: "Lists the available API endpoints and example usage.",
			Example: map[string]any{
				"request": "curl -H 'Authorization: Bearer token-123' http://localhost:8111/v1/info",
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/v1/config",
			Auth:        "bearer",
			Description: "Returns the sanitized runtime configuration without the raw bearer token.",
			Example: map[string]any{
				"request": "curl -H 'Authorization: Bearer token-123' http://localhost:8111/v1/config",
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/v1/users",
			Auth:        "bearer",
			Description: "Lists tracked per-user quota state.",
			Example: map[string]any{
				"request": "curl -H 'Authorization: Bearer token-123' http://localhost:8111/v1/users",
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/v1/users/{userId}/status",
			Auth:        "bearer",
			Description: "Returns quota state for a specific user SID or username.",
			Example: map[string]any{
				"request": "curl -H 'Authorization: Bearer token-123' http://localhost:8111/v1/users/john/status",
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/v1/users/{userId}/adjust",
			Auth:        "bearer",
			Description: "Adds or removes consumed time by delta in seconds for a SID or username.",
			Example: map[string]any{
				"request": "curl -X POST -H 'Authorization: Bearer token-123' -H 'Content-Type: application/json' http://localhost:8111/v1/users/john/adjust",
				"body":    map[string]int64{"delta_sec": 300},
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/v1/users/{userId}/allowance",
			Auth:        "bearer",
			Description: "Sets the daily allowance in seconds for a SID or username.",
			Example: map[string]any{
				"request": "curl -X POST -H 'Authorization: Bearer token-123' -H 'Content-Type: application/json' http://localhost:8111/v1/users/john/allowance",
				"body":    map[string]int64{"daily_allowance_sec": 1800},
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/v1/users/{userId}/reset-today",
			Auth:        "bearer",
			Description: "Resets today's consumed time and warning flags for a SID or username.",
			Example: map[string]any{
				"request": "curl -X POST -H 'Authorization: Bearer token-123' http://localhost:8111/v1/users/john/reset-today",
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/v1/announce",
			Auth:        "bearer",
			Description: "Speaks a message in the active user's session.",
			Example: map[string]any{
				"request": "curl -X POST -H 'Authorization: Bearer token-123' -H 'Content-Type: application/json' http://localhost:8111/v1/announce",
				"body":    map[string]string{"message": "WindowsUserUptimeControl test announcement"},
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/v1/enforcement/hibernate-now",
			Auth:        "bearer",
			Description: "Runs the countdown immediately and triggers hibernation or shutdown fallback.",
			Example: map[string]any{
				"request": "curl -X POST -H 'Authorization: Bearer token-123' http://localhost:8111/v1/enforcement/hibernate-now",
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/v1/logs/recent",
			Auth:        "bearer",
			Description: "Returns recent API and service log lines.",
			Example: map[string]any{
				"request": "curl -H 'Authorization: Bearer token-123' http://localhost:8111/v1/logs/recent",
			},
		},
	}
}
