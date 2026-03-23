package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	hub "github.com/wyiu/aerodocs/hub"
	"github.com/wyiu/aerodocs/hub/internal/server"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "admin" {
		if err := runAdmin(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := runServer(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runServer() error {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "aerodocs.db", "SQLite database path")
	dev := flag.Bool("dev", false, "enable development mode (CORS)")
	flag.Parse()

	st, err := store.New(*dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer st.Close()

	jwtSecret, err := server.InitJWTSecret(st)
	if err != nil {
		return fmt.Errorf("init JWT secret: %w", err)
	}

	srv := server.New(server.Config{
		Addr:       *addr,
		Store:      st,
		JWTSecret:  jwtSecret,
		IsDev:      *dev,
		FrontendFS: &hub.FrontendFS,
	})

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		fmt.Println("\nShutting down...")
		srv.Shutdown(context.Background())
	}()

	return srv.Start()
}
