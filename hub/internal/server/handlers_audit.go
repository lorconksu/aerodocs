package server

import (
	"net/http"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := model.AuditFilter{}
	filter.Limit, filter.Offset = parsePagination(q, 50)

	if v := q.Get("action"); v != "" {
		filter.Action = &v
	}
	if v := q.Get("user_id"); v != "" {
		filter.UserID = &v
	}
	if v := q.Get("from"); v != "" {
		filter.From = &v
	}
	if v := q.Get("to"); v != "" {
		filter.To = &v
	}

	entries, total, err := s.store.ListAuditLogs(filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list audit logs")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   total,
		"limit":   filter.Limit,
		"offset":  filter.Offset,
	})
}
