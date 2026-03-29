package server

import (
	"net/http"
	"strconv"

	"github.com/wyiu/aerodocs/hub/internal/model"
	"github.com/wyiu/aerodocs/hub/internal/notify"
)

// loadSMTPConfig reads SMTP settings from the store's _config table.
func (s *Server) loadSMTPConfig() model.SMTPConfig {
	get := func(key string) string {
		v, _ := s.store.GetConfig(key)
		return v
	}

	port := 587
	if p := get("smtp_port"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	tls := false
	if t := get("smtp_tls"); t == "true" || t == "1" {
		tls = true
	}

	enabled := false
	if e := get("smtp_enabled"); e == "true" || e == "1" {
		enabled = true
	}

	return model.SMTPConfig{
		Host:     get("smtp_host"),
		Port:     port,
		Username: get("smtp_username"),
		Password: get("smtp_password"),
		From:     get("smtp_from"),
		TLS:      tls,
		Enabled:  enabled,
	}
}

// handleGetSMTPConfig returns the current SMTP configuration.
// Password is write-only: returns "********" if set, empty string if not.
func (s *Server) handleGetSMTPConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.loadSMTPConfig()

	// Mask the password
	if cfg.Password != "" {
		cfg.Password = "********"
	}

	respondJSON(w, http.StatusOK, cfg)
}

// handleUpdateSMTPConfig saves SMTP configuration fields to the store.
// If password is "********" or empty, the existing password is preserved.
func (s *Server) handleUpdateSMTPConfig(w http.ResponseWriter, r *http.Request) {
	var req model.SMTPConfig
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}

	set := func(key, value string) error {
		return s.store.SetConfig(key, value)
	}

	if err := set("smtp_host", req.Host); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save smtp_host")
		return
	}
	if err := set("smtp_port", strconv.Itoa(req.Port)); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save smtp_port")
		return
	}
	if err := set("smtp_username", req.Username); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save smtp_username")
		return
	}
	if err := set("smtp_from", req.From); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save smtp_from")
		return
	}

	tlsVal := "false"
	if req.TLS {
		tlsVal = "true"
	}
	if err := set("smtp_tls", tlsVal); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save smtp_tls")
		return
	}

	enabledVal := "false"
	if req.Enabled {
		enabledVal = "true"
	}
	if err := set("smtp_enabled", enabledVal); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save smtp_enabled")
		return
	}

	// Skip password update if it's the placeholder or empty
	if req.Password != "********" && req.Password != "" {
		if err := set("smtp_password", req.Password); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to save smtp_password")
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleTestSMTP sends a test email to the given recipient using the current SMTP config.
func (s *Server) handleTestSMTP(w http.ResponseWriter, r *http.Request) {
	var req model.SMTPTestRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}

	if req.Recipient == "" {
		respondError(w, http.StatusBadRequest, "recipient is required")
		return
	}

	cfg := s.loadSMTPConfig()

	// Force enabled for the test send so SendEmail doesn't skip it
	cfg.Enabled = true

	if err := notify.SendEmail(cfg, req.Recipient, "AeroDocs SMTP Test", "This is a test email from AeroDocs. Your SMTP configuration is working correctly."); err != nil {
		respondError(w, http.StatusBadGateway, "failed to send test email: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// handleGetNotificationPreferences returns the authenticated user's notification preferences.
func (s *Server) handleGetNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	prefs, err := s.store.GetNotificationPreferences(userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get notification preferences")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"preferences": prefs})
}

// handleUpdateNotificationPreferences saves notification preference updates for the authenticated user.
func (s *Server) handleUpdateNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	var req model.NotificationPreferencesRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}

	userID := UserIDFromContext(r.Context())

	for _, pref := range req.Preferences {
		if err := s.store.SetNotificationPreference(userID, pref.EventType, pref.Enabled); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to update preference for "+pref.EventType)
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleListNotificationLog returns a paginated list of notification log entries.
func (s *Server) handleListNotificationLog(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r.URL.Query(), 50)

	entries, total, err := s.store.ListNotificationLog(limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list notification log")
		return
	}

	// Return empty slice rather than null
	if entries == nil {
		entries = []model.NotificationLogEntry{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   total,
	})
}
