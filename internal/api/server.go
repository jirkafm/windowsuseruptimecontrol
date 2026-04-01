package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"wincontrol/internal/model"
)

type AdminController interface {
	State() model.StateFile
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
		users := make([]model.UserDayState, 0, len(s.admin.State().Users))
		for _, user := range s.admin.State().Users {
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
			user, ok := s.admin.State().Users[userID]
			if !ok {
				s.logRequest(r, http.StatusNotFound)
				http.NotFound(w, r)
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
