package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/wyiu/aerodocs/agent/internal/certs"
	"github.com/wyiu/aerodocs/agent/internal/dropzone"
	"github.com/wyiu/aerodocs/agent/internal/filebrowser"
	"github.com/wyiu/aerodocs/agent/internal/heartbeat"
	"github.com/wyiu/aerodocs/agent/internal/logtailer"
	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

type Config struct {
	HubAddr      string
	ServerID     string
	Token        string
	Hostname     string
	IPAddress    string
	OS           string
	AgentVersion string
	CertDir      string // directory for mTLS cert storage; empty uses /etc/aerodocs/tls/
}

type Client struct {
	hubAddr      string
	serverID     string
	token        string
	hostname     string
	ipAddress    string
	os           string
	agentVersion string
	backoff      time.Duration
	maxBackoff   time.Duration
	tailSessionsMu sync.Mutex
	tailSessions   map[string]chan struct{}
	dropzone       *dropzone.Dropzone
	certStore      *certs.Store
}

func New(cfg Config) *Client {
	certDir := cfg.CertDir
	if certDir == "" {
		certDir = "/etc/aerodocs/tls/"
	}
	return &Client{
		hubAddr:      cfg.HubAddr,
		serverID:     cfg.ServerID,
		token:        cfg.Token,
		hostname:     cfg.Hostname,
		ipAddress:    cfg.IPAddress,
		os:           cfg.OS,
		agentVersion: cfg.AgentVersion,
		backoff:      1 * time.Second,
		maxBackoff:   60 * time.Second,
		tailSessions: make(map[string]chan struct{}),
		dropzone:     dropzone.New(dropzone.DefaultDir),
		certStore:    certs.NewStore(certDir),
	}
}

func (c *Client) ServerID() string {
	return c.serverID
}

// SelfUnregister calls the Hub's HTTP API to delete this server.
// Used by the install script to clean up the old registration before a re-install.
func (c *Client) SelfUnregister(ctx context.Context) error {
	// Derive HTTP base URL from the gRPC hub address
	host, port, err := net.SplitHostPort(c.hubAddr)
	if err != nil {
		host = c.hubAddr
		port = ""
	}

	var baseURL string
	if port == "443" || (net.ParseIP(host) == nil && strings.Contains(host, ".")) {
		// Hostname — use HTTPS (same host, default HTTPS port)
		baseURL = "https://" + host
	} else {
		// Direct IP — Hub HTTP is on port 8081 by convention, but we don't know for sure.
		// Try the common case: same host, port 8081
		baseURL = "http://" + host + ":8081"
	}

	url := fmt.Sprintf("%s/api/servers/%s/self-unregister", baseURL, c.serverID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil // Success or already gone
	}
	return fmt.Errorf("unexpected status: %d", resp.StatusCode)
}

func (c *Client) Run(ctx context.Context) error {
	for {
		err := c.connectAndStream(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		wait := c.nextBackoff()
		log.Printf("connection lost: %v — reconnecting in %v", err, wait)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

func (c *Client) handleFileListRequest(p *pb.HubMessage_FileListRequest, sendCh chan<- *pb.AgentMessage) {
	resp, _ := filebrowser.ListDir(p.FileListRequest.Path)
	if resp != nil {
		resp.RequestId = p.FileListRequest.RequestId
		sendCh <- &pb.AgentMessage{
			Payload: &pb.AgentMessage_FileListResponse{
				FileListResponse: resp,
			},
		}
	}
}

func (c *Client) handleFileReadRequest(p *pb.HubMessage_FileReadRequest, sendCh chan<- *pb.AgentMessage) {
	resp, _ := filebrowser.ReadFile(
		p.FileReadRequest.Path,
		p.FileReadRequest.Offset,
		p.FileReadRequest.Limit,
	)
	if resp != nil {
		resp.RequestId = p.FileReadRequest.RequestId
		sendCh <- &pb.AgentMessage{
			Payload: &pb.AgentMessage_FileReadResponse{
				FileReadResponse: resp,
			},
		}
	}
}

func (c *Client) handleLogStreamRequest(p *pb.HubMessage_LogStreamRequest, sendCh chan<- *pb.AgentMessage) {
	req := p.LogStreamRequest
	stop := make(chan struct{})
	c.tailSessionsMu.Lock()
	c.tailSessions[req.RequestId] = stop
	c.tailSessionsMu.Unlock()
	go logtailer.StartTail(req.Path, req.Grep, req.Offset, sendCh, req.RequestId, stop)
	log.Printf("started log tail: %s path=%s grep=%s", req.RequestId, req.Path, req.Grep)
}

func (c *Client) handleLogStreamStop(p *pb.HubMessage_LogStreamStop) {
	c.tailSessionsMu.Lock()
	stop, ok := c.tailSessions[p.LogStreamStop.RequestId]
	if ok {
		delete(c.tailSessions, p.LogStreamStop.RequestId)
	}
	c.tailSessionsMu.Unlock()
	if ok {
		close(stop)
		log.Printf("stopped log tail: %s", p.LogStreamStop.RequestId)
	}
}

func (c *Client) handleFileUploadRequest(p *pb.HubMessage_FileUploadRequest, sendCh chan<- *pb.AgentMessage) {
	req := p.FileUploadRequest
	ack := c.dropzone.HandleChunk(req.RequestId, req.Filename, req.Chunk, req.Done)
	if ack != nil {
		sendCh <- &pb.AgentMessage{
			Payload: &pb.AgentMessage_FileUploadAck{
				FileUploadAck: ack,
			},
		}
	}
}

func (c *Client) handleFileDeleteRequest(p *pb.HubMessage_FileDeleteRequest, sendCh chan<- *pb.AgentMessage) {
	req := p.FileDeleteRequest
	resp := &pb.FileDeleteResponse{RequestId: req.RequestId}
	// Only allow deletion from dropzone directory
	cleanPath := filepath.Clean(req.Path)
	if !strings.HasPrefix(cleanPath, "/tmp/aerodocs-dropzone/") {
		resp.Success = false
		resp.Error = "deletion only allowed from dropzone directory"
	} else if err := os.Remove(cleanPath); err != nil {
		log.Printf("dropzone delete error: %v", err)
		resp.Success = false
		resp.Error = "file operation failed"
	} else {
		resp.Success = true
	}
	sendCh <- &pb.AgentMessage{
		Payload: &pb.AgentMessage_FileDeleteResponse{
			FileDeleteResponse: resp,
		},
	}
}

func (c *Client) handleUnregisterRequest(p *pb.HubMessage_UnregisterRequest, sendCh chan<- *pb.AgentMessage) {
	req := p.UnregisterRequest
	// Send ack before self-destruct
	sendCh <- &pb.AgentMessage{
		Payload: &pb.AgentMessage_UnregisterAck{
			UnregisterAck: &pb.UnregisterAck{
				RequestId: req.RequestId,
				Success:   true,
			},
		},
	}
	log.Printf("unregister requested — initiating self-cleanup")
	// Give the ack time to send, then run cleanup
	go func() {
		time.Sleep(2 * time.Second)
		c.selfCleanup()
	}()
}

func (c *Client) handleMessage(msg *pb.HubMessage, sendCh chan<- *pb.AgentMessage) {
	switch p := msg.Payload.(type) {
	case *pb.HubMessage_FileListRequest:
		c.handleFileListRequest(p, sendCh)
	case *pb.HubMessage_FileReadRequest:
		c.handleFileReadRequest(p, sendCh)
	case *pb.HubMessage_LogStreamRequest:
		c.handleLogStreamRequest(p, sendCh)
	case *pb.HubMessage_LogStreamStop:
		c.handleLogStreamStop(p)
	case *pb.HubMessage_FileUploadRequest:
		c.handleFileUploadRequest(p, sendCh)
	case *pb.HubMessage_FileDeleteRequest:
		c.handleFileDeleteRequest(p, sendCh)
	case *pb.HubMessage_UnregisterRequest:
		c.handleUnregisterRequest(p, sendCh)
	case *pb.HubMessage_CertRenewResponse:
		c.handleCertRenewResponse(p)
	default:
		log.Printf("unhandled hub message: %T", p)
	}
}

func (c *Client) handleCertRenewResponse(p *pb.HubMessage_CertRenewResponse) {
	resp := p.CertRenewResponse
	if resp.Error != "" {
		log.Printf("cert renewal rejected: %s", resp.Error)
		return
	}
	if len(resp.ClientCert) == 0 || len(resp.CaCert) == 0 {
		log.Printf("cert renewal response missing certs")
		return
	}
	if err := c.certStore.StoreCert(resp.ClientCert, resp.CaCert); err != nil {
		log.Printf("failed to store renewed certs: %v", err)
		return
	}
	log.Printf("mTLS certificate renewed (will use on next reconnect)")
}

// startCertRenewalTicker checks cert expiry every 10s alongside heartbeats
// and sends a CertRenewRequest when renewal is needed.
func (c *Client) startCertRenewalTicker(interval time.Duration, sendCh chan<- *pb.AgentMessage, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if !c.certStore.HasCert() || !c.certStore.NeedsRenewal(6*time.Hour) {
				continue
			}
			csrDER, err := c.certStore.GenerateCSR(c.serverID)
			if err != nil {
				log.Printf("cert renewal CSR generation failed: %v", err)
				continue
			}
			sendCh <- &pb.AgentMessage{
				Payload: &pb.AgentMessage_CertRenewRequest{
					CertRenewRequest: &pb.CertRenewRequest{
						Csr: csrDER,
					},
				},
			}
			log.Printf("sent cert renewal request")
		}
	}
}

// dialHub creates a gRPC connection to the hub. If the cert store has a
// valid mTLS certificate, it is used for transport credentials. Otherwise
// the connection falls back to auto-detected TLS or plaintext.
func (c *Client) dialHub() (*grpc.ClientConn, error) {
	var creds grpc.DialOption
	if tlsCfg := c.certStore.TLSConfig(); tlsCfg != nil {
		creds = grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg))
	} else if c.useTLS() {
		creds = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{}))
	} else {
		creds = grpc.WithTransportCredentials(insecure.NewCredentials())
	}
	conn, err := grpc.NewClient(c.hubAddr, creds)
	if err != nil {
		return nil, fmt.Errorf("dial hub: %w", err)
	}
	return conn, nil
}

// registerOrHandshake sends the initial register (first connect) or heartbeat
// (reconnect) message and waits for the corresponding ack.
func (c *Client) registerOrHandshake(stream pb.AgentService_ConnectClient) error {
	if c.serverID == "" {
		// Generate a CSR with a placeholder CN (serverID not known yet)
		var csrDER []byte
		csr, err := c.certStore.GenerateCSR("pending")
		if err != nil {
			log.Printf("warning: failed to generate CSR for registration: %v", err)
		} else {
			csrDER = csr
		}

		if err := stream.Send(&pb.AgentMessage{
			Payload: &pb.AgentMessage_Register{
				Register: &pb.RegisterAgent{
					Token:        c.token,
					Hostname:     c.hostname,
					IpAddress:    c.ipAddress,
					Os:           c.os,
					AgentVersion: c.agentVersion,
					Csr:          csrDER,
				},
			},
		}); err != nil {
			return fmt.Errorf("send register: %w", err)
		}
		msg, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("recv register ack: %w", err)
		}
		ack := msg.GetRegisterAck()
		if ack == nil {
			return fmt.Errorf("expected RegisterAck, got %T", msg.Payload)
		}
		if !ack.Success {
			return fmt.Errorf("registration rejected: %s", ack.Error)
		}
		c.serverID = ack.ServerId
		log.Printf("registered successfully: server_id=%s", c.serverID)

		// Store mTLS certs if the hub provided them
		if len(ack.ClientCert) > 0 && len(ack.CaCert) > 0 {
			if err := c.certStore.StoreCert(ack.ClientCert, ack.CaCert); err != nil {
				log.Printf("warning: failed to store mTLS certs: %v", err)
			} else {
				log.Printf("mTLS certificate stored (will use on next reconnect)")
			}
		}
		return nil
	}

	if err := stream.Send(heartbeat.BuildMessage(c.serverID)); err != nil {
		return fmt.Errorf("send heartbeat: %w", err)
	}
	msg, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("recv heartbeat ack: %w", err)
	}
	if msg.GetHeartbeatAck() == nil {
		return fmt.Errorf("expected HeartbeatAck, got %T", msg.Payload)
	}
	return nil
}

// startRecvLoop starts a goroutine that receives messages from the stream and
// dispatches them. It returns a channel that receives the first error (or nil
// on clean EOF).
func (c *Client) startRecvLoop(stream pb.AgentService_ConnectClient, sendCh chan<- *pb.AgentMessage) <-chan error {
	recvErr := make(chan error, 1)
	go func() {
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				recvErr <- nil
				return
			}
			if err != nil {
				recvErr <- err
				return
			}
			// File upload chunks must be sequential to avoid race conditions.
			// Other messages can be handled concurrently.
			if _, ok := msg.Payload.(*pb.HubMessage_FileUploadRequest); ok {
				c.handleMessage(msg, sendCh)
			} else {
				go c.handleMessage(msg, sendCh)
			}
		}
	}()
	return recvErr
}

func (c *Client) connectAndStream(ctx context.Context) error {
	conn, err := c.dialHub()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pb.NewAgentServiceClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}

	if err := c.registerOrHandshake(stream); err != nil {
		return err
	}

	c.resetBackoff()
	log.Printf("connected to hub at %s", c.hubAddr)

	sendCh := make(chan *pb.AgentMessage, 16)
	defer func() {
		c.tailSessionsMu.Lock()
		for id, stop := range c.tailSessions {
			close(stop)
			delete(c.tailSessions, id)
		}
		c.tailSessionsMu.Unlock()
		c.dropzone.Cleanup()
	}()
	hbStop := make(chan struct{})
	defer close(hbStop)
	go heartbeat.StartTicker(c.serverID, 10*time.Second, sendCh, hbStop)
	go c.startCertRenewalTicker(10*time.Second, sendCh, hbStop)

	recvErr := c.startRecvLoop(stream, sendCh)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-recvErr:
			return fmt.Errorf("receive error: %w", err)
		case msg := <-sendCh:
			if err := stream.Send(msg); err != nil {
				return fmt.Errorf("send: %w", err)
			}
		}
	}
}

func (c *Client) nextBackoff() time.Duration {
	current := c.backoff
	c.backoff *= 2
	if c.backoff > c.maxBackoff {
		c.backoff = c.maxBackoff
	}
	return current
}

func (c *Client) resetBackoff() {
	c.backoff = 1 * time.Second
}

func (c *Client) selfCleanup() {
	tmpDir, err := os.MkdirTemp("", "aerodocs-cleanup-*")
	if err != nil {
		log.Printf("failed to create temp dir for cleanup script: %v", err)
		return
	}
	scriptPath := filepath.Join(tmpDir, "cleanup.sh")
	script := fmt.Sprintf(`#!/bin/bash
# AeroDocs Agent self-cleanup script
# Stop the service first
systemctl stop aerodocs-agent 2>/dev/null || true
systemctl disable aerodocs-agent 2>/dev/null || true

# Kill any remaining agent processes (not this script)
pkill -9 -f "aerodocs-agent" 2>/dev/null || true
sleep 1

# Remove files with retries (binary may be briefly locked)
for i in 1 2 3; do
  rm -f /usr/local/bin/aerodocs-agent 2>/dev/null && break
  sleep 1
done

rm -f /etc/systemd/system/aerodocs-agent.service
rm -f /etc/aerodocs/agent.conf
rm -rf /tmp/aerodocs-dropzone
systemctl daemon-reload 2>/dev/null || true
rm -rf %s
`, tmpDir)
	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		log.Printf("failed to write cleanup script: %v", err)
		return
	}
	log.Printf("executing cleanup script: %s", scriptPath)
	// Use syscall.Exec to replace this process with the cleanup script
	syscall.Exec("/bin/bash", []string{"bash", scriptPath}, os.Environ())
}

// useTLS returns true if the hub address appears to be a hostname (not an IP),
// indicating connection through a TLS-terminating reverse proxy.
func (c *Client) useTLS() bool {
	host, port, err := net.SplitHostPort(c.hubAddr)
	if err != nil {
		// No port — treat as hostname:443
		host = c.hubAddr
		port = "443"
	}
	// If host is an IP address, use insecure (direct connection)
	if net.ParseIP(host) != nil {
		return false
	}
	// Hostname with port 443 or no explicit port — use TLS
	if port == "443" || port == "" {
		return true
	}
	// Hostname with explicit non-443 port — check if it looks like a domain
	return strings.Contains(host, ".")
}
