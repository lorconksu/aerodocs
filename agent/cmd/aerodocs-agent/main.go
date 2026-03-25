package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/wyiu/aerodocs/agent/internal/client"
	"github.com/wyiu/aerodocs/agent/internal/sysinfo"
)

const version = "0.1.0"

type agentConfig struct {
	ServerID string `json:"server_id"`
	HubURL   string `json:"hub_url"`
}

func main() {
	hub := flag.String("hub", "", "Hub gRPC address (e.g., hub.example.com:9090)")
	token := flag.String("token", "", "one-time registration token")
	configPath := flag.String("config", "/etc/aerodocs/agent.conf", "path to config file")
	selfUnregister := flag.Bool("self-unregister", false, "connect to Hub, request deletion of this server, then exit")
	flag.Parse()

	if *selfUnregister {
		runSelfUnregister(*configPath)
		return
	}

	cfg, hubAddr, serverID, regToken := resolveConfig(*configPath, *hub, *token)

	hostname := sysinfo.Hostname()
	osInfo := sysinfo.OSInfo()

	c := client.New(client.Config{
		HubAddr:      hubAddr,
		ServerID:     serverID,
		Token:        regToken,
		Hostname:     hostname,
		IPAddress:    "",
		OS:           osInfo,
		AgentVersion: version,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("AeroDocs Agent v%s starting — hub=%s", version, hubAddr)

	err := c.Run(ctx)
	if err != nil && ctx.Err() == nil {
		log.Fatalf("agent error: %v", err)
	}

	if cfg == nil && c.ServerID() != "" {
		saveNewConfig(*configPath, hubAddr, c.ServerID())
	}

	log.Println("agent stopped")
}

// runSelfUnregister loads the config, sends an unregister request to the Hub, removes the config
// file, then exits. If no config is found, it exits immediately with no error.
func runSelfUnregister(configPath string) {
	cfg, err := loadConfig(configPath)
	if err != nil || cfg == nil {
		log.Printf("no config found at %s — nothing to unregister", configPath)
		os.Exit(0)
	}
	log.Printf("self-unregistering server_id=%s from hub=%s", cfg.ServerID, cfg.HubURL)
	c := client.New(client.Config{
		HubAddr:      cfg.HubURL,
		ServerID:     cfg.ServerID,
		AgentVersion: version,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.SelfUnregister(ctx); err != nil {
		log.Printf("self-unregister failed: %v (continuing anyway)", err)
	} else {
		log.Printf("self-unregister successful — server removed from Hub")
	}
	os.Remove(configPath)
	os.Exit(0)
}

// resolveConfig determines the effective hub address, server ID, and registration token
// from the saved config file and CLI flags. If a token flag is provided, any existing
// config is discarded to force a fresh registration.
func resolveConfig(configPath, hubFlag, tokenFlag string) (cfg *agentConfig, hubAddr, serverID, regToken string) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		cfg = nil
	}

	// If --token is provided, always do a fresh registration (ignore saved config)
	// This handles re-installs after unregister
	if tokenFlag != "" {
		if hubFlag == "" {
			fmt.Fprintf(os.Stderr, "--hub is required when using --token\n")
			flag.Usage()
			os.Exit(1)
		}
		if cfg != nil {
			log.Printf("--token provided, ignoring saved config (previous server_id=%s)", cfg.ServerID)
			os.Remove(configPath)
		}
		cfg = nil
	}

	if cfg == nil && tokenFlag == "" {
		fmt.Fprintf(os.Stderr, "first run: --hub and --token are required\n")
		flag.Usage()
		os.Exit(1)
	}

	hubAddr = hubFlag
	regToken = tokenFlag

	if cfg != nil {
		hubAddr = cfg.HubURL
		serverID = cfg.ServerID
		regToken = ""
		log.Printf("loaded config: server_id=%s hub=%s", serverID, hubAddr)
	}

	return cfg, hubAddr, serverID, regToken
}

// saveNewConfig persists the server ID and hub URL returned after a successful first registration.
func saveNewConfig(configPath, hubAddr, serverID string) {
	newCfg := agentConfig{
		ServerID: serverID,
		HubURL:   hubAddr,
	}
	if err := saveConfig(configPath, newCfg); err != nil {
		log.Printf("warning: failed to save config: %v", err)
	} else {
		log.Printf("config saved to %s", configPath)
	}
}

func loadConfig(path string) (*agentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg agentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfig(path string, cfg agentConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
