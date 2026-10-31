package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/mahbodhastam/falafel/internal/room"
)

// handleHealth reports liveness and process uptime.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"uptime": int(time.Since(s.started).Seconds()),
	})
}

// handleConfig exposes ICE servers and limits for the browser client.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	servers := make([]iceServer, 0, 2)
	if len(s.cfg.STUNURLs) > 0 {
		servers = append(servers, iceServer{URLs: s.cfg.STUNURLs})
	}
	if len(s.cfg.TURNURLs) > 0 {
		username, credential := s.cfg.TURNUsername, s.cfg.TURNCredential
		if s.cfg.TURNSecret != "" {
			username, credential = turnCredentials(s.cfg.TURNSecret, time.Now().Add(s.svc.TTL()))
		}
		servers = append(servers, iceServer{URLs: s.cfg.TURNURLs, Username: username, Credential: credential})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"iceServers":        servers,
		"maxFileSize":       s.cfg.MaxFileSize,
		"minPasswordLength": s.cfg.MinPasswordLength,
		"roomTTLSeconds":    int(s.svc.TTL().Seconds()),
		"baseURL":           s.externalBaseURL(r),
	})
}

type createRoomRequest struct {
	FileName string `json:"fileName"`
	FileSize int64  `json:"fileSize"`
	MimeType string `json:"mimeType"`
	Password string `json:"password"`
}

// handleCreateRoom validates the upload metadata and returns the owner secret.
func (s *Server) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r, s.cfg.TrustProxy)
	allowed, retry := s.createLimiter.Allow(ip)
	if !allowed {
		w.Header().Set("Retry-After", secondsCeil(retry))
		writeError(w, http.StatusTooManyRequests, "too_many_rooms")
		return
	}
	s.createLimiter.Fail(ip)

	var req createRoomRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	created, err := s.svc.Create(r.Context(), room.CreateInput{
		FileName: req.FileName,
		FileSize: req.FileSize,
		MimeType: req.MimeType,
		Password: req.Password,
	})
	if err != nil {
		var invalid *room.ValidationError
		if errors.As(err, &invalid) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error": map[string]string{
					"code":   "invalid_input",
					"field":  invalid.Field,
					"reason": invalid.Reason,
				},
			})
			return
		}
		s.log.Error("create room", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"token":       created.Room.Token,
		"url":         s.externalBaseURL(r) + "/download/" + created.Room.Token,
		"ownerSecret": created.OwnerSecret,
		"expiresAt":   created.Room.ExpiresAt,
		"room":        created.Room.Public(),
	})
}

// handleGetRoom returns the public metadata needed before joining a room.
func (s *Server) handleGetRoom(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !validToken(token) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	pub, err := s.svc.Public(r.Context(), token)
	if err != nil {
		s.writeRoomError(w, err, "get room")
		return
	}
	writeJSON(w, http.StatusOK, pub)
}

type joinRequest struct {
	Password string `json:"password"`
}

// handleJoin checks the password and issues a short-lived signaling ticket.
func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !validToken(token) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}

	key := clientIP(r, s.cfg.TrustProxy) + "|" + token
	allowed, retry := s.joinLimiter.Allow(key)
	if !allowed {
		w.Header().Set("Retry-After", secondsCeil(retry))
		writeError(w, http.StatusTooManyRequests, "too_many_attempts")
		return
	}

	var req joinRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	ticket, err := s.svc.Join(r.Context(), token, req.Password)
	switch {
	case err == nil:
		s.joinLimiter.Reset(key)
		writeJSON(w, http.StatusOK, map[string]any{
			"ticket":    ticket.Token,
			"expiresAt": ticket.ExpiresAt,
		})
	case errors.Is(err, room.ErrWrongPassword):
		s.joinLimiter.Fail(key)
		s.hub.NotifyStats(token)
		writeError(w, http.StatusUnauthorized, "wrong_password")
	case errors.Is(err, room.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	default:
		s.log.Error("join room", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error")
	}
}

// handleOwner returns the full room snapshot to its authenticated owner.
func (s *Server) handleOwner(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	secret, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	owner, err := s.svc.Owner(r.Context(), token, secret)
	if err != nil {
		s.writeRoomError(w, err, "owner snapshot")
		return
	}
	writeJSON(w, http.StatusOK, owner)
}

type closeRequest struct {
	OwnerSecret string `json:"ownerSecret"`
}

// handleClose closes a room; it also accepts the sendBeacon JSON body.
func (s *Server) handleClose(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	secret, ok := bearerToken(r)
	if !ok {
		var req closeRequest
		if err := decodeJSON(w, r, &req); err == nil {
			secret, ok = req.OwnerSecret, req.OwnerSecret != ""
		}
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := s.svc.Close(r.Context(), token, secret); err != nil {
		s.writeRoomError(w, err, "close room")
		return
	}
	s.hub.CloseRoom(token, "owner_closed")
	w.WriteHeader(http.StatusNoContent)
}

// handleWebSocket hands a valid room token to the signaling hub.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !validToken(token) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	s.hub.ServeWS(w, r, token)
}
