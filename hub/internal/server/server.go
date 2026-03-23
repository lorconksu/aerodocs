package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/auth"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

type Server struct {
	httpServer *http.Server
	store      *store.Store
	jwtSecret  string
	isDev      bool
}

type Config struct {
	Addr      string
	Store     *store.Store
	JWTSecret string
	IsDev     bool
}

func New(cfg Config) *Server {
	s := &Server{
		store:     cfg.Store,
		jwtSecret: cfg.JWTSecret,
		isDev:     cfg.IsDev,
	}

	mux := s.routes()

	s.httpServer = &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

func (s *Server) Start() error {
	fmt.Printf("AeroDocs Hub listening on %s\n", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// InitJWTSecret generates a random JWT signing key on first run,
// or retrieves the existing one from the database.
func InitJWTSecret(st *store.Store) (string, error) {
	secret, err := st.GetConfig("jwt_signing_key")
	if err == nil {
		return secret, nil
	}

	// Generate new 256-bit key
	secret = auth.GenerateTemporaryPassword() + auth.GenerateTemporaryPassword()
	if err := st.SetConfig("jwt_signing_key", secret); err != nil {
		return "", fmt.Errorf("store jwt key: %w", err)
	}

	return secret, nil
}
