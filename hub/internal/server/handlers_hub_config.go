package server

import (
	"net/http"
)

type hubConfigResponse struct {
	GRPCExternalAddr string `json:"grpc_external_addr"`
}

type hubConfigRequest struct {
	GRPCExternalAddr string `json:"grpc_external_addr"`
}

func (s *Server) handleGetHubConfig(w http.ResponseWriter, r *http.Request) {
	cfg := hubConfigResponse{}

	// DB value takes priority; fall back to CLI flag
	if addr, err := s.store.GetConfig("grpc_external_addr"); err == nil {
		cfg.GRPCExternalAddr = addr
	} else if s.grpcExternalAddr != "" {
		cfg.GRPCExternalAddr = s.grpcExternalAddr
	}

	respondJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleUpdateHubConfig(w http.ResponseWriter, r *http.Request) {
	var req hubConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}

	if err := s.store.SetConfig("grpc_external_addr", req.GRPCExternalAddr); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save hub config")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
