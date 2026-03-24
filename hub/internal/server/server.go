package server

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/auth"
	"github.com/wyiu/aerodocs/hub/internal/connmgr"
	"github.com/wyiu/aerodocs/hub/internal/grpcserver"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

type Server struct {
	httpServer  *http.Server
	store       *store.Store
	jwtSecret   string
	isDev       bool
	frontendFS  *embed.FS
	agentBinDir string
	grpcAddr    string
	connMgr     *connmgr.ConnManager
	pending     *grpcserver.PendingRequests
}

type Config struct {
	Addr        string
	Store       *store.Store
	JWTSecret   string
	IsDev       bool
	FrontendFS  *embed.FS
	AgentBinDir string
	GRPCAddr    string
	ConnMgr     *connmgr.ConnManager
	Pending     *grpcserver.PendingRequests
}

func New(cfg Config) *Server {
	s := &Server{
		store:       cfg.Store,
		jwtSecret:   cfg.JWTSecret,
		isDev:       cfg.IsDev,
		frontendFS:  cfg.FrontendFS,
		agentBinDir: cfg.AgentBinDir,
		grpcAddr:    cfg.GRPCAddr,
		connMgr:     cfg.ConnMgr,
		pending:     cfg.Pending,
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

// spaHandler serves the embedded frontend SPA. In dev mode, it returns a
// helpful error since the frontend is served by the Vite dev server. In
// production it serves static files from the embedded FS and falls back to
// index.html for any path that doesn't match a real file (React Router).
func (s *Server) spaHandler() http.Handler {
	if s.isDev || s.frontendFS == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend not embedded — run Vite dev server on :5173", http.StatusServiceUnavailable)
		})
	}

	sub, err := fs.Sub(*s.frontendFS, "web/dist")
	if err != nil {
		panic("spaHandler: failed to sub web/dist from embedded FS: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to open the requested file.
		f, openErr := sub.Open(r.URL.Path[1:]) // strip leading /
		if openErr == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found — serve index.html for client-side routing.
		index, readErr := fs.ReadFile(sub, "index.html")
		if readErr != nil {
			http.Error(w, "index.html not found in embedded FS", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(index)
	})
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
