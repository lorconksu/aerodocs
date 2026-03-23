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
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		if *hub == "" || *token == "" {
			fmt.Fprintf(os.Stderr, "first run: --hub and --token are required\n")
			flag.Usage()
			os.Exit(1)
		}
	}

	hubAddr := *hub
	serverID := ""
	regToken := *token

	if cfg != nil {
		hubAddr = cfg.HubURL
		serverID = cfg.ServerID
		regToken = ""
		log.Printf("loaded config: server_id=%s hub=%s", serverID, hubAddr)
	}

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

	err = c.Run(ctx)
	if err != nil && ctx.Err() == nil {
		log.Fatalf("agent error: %v", err)
	}

	if cfg == nil && c.ServerID() != "" {
		newCfg := agentConfig{
			ServerID: c.ServerID(),
			HubURL:   hubAddr,
		}
		if err := saveConfig(*configPath, newCfg); err != nil {
			log.Printf("warning: failed to save config: %v", err)
		} else {
			log.Printf("config saved to %s", *configPath)
		}
	}

	log.Println("agent stopped")
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
