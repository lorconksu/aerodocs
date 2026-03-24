package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

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
	tailSessions map[string]chan struct{}
	dropzone     *dropzone.Dropzone
}

func New(cfg Config) *Client {
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
	}
}

func (c *Client) ServerID() string {
	return c.serverID
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

func (c *Client) handleMessage(msg *pb.HubMessage, sendCh chan<- *pb.AgentMessage) {
	switch p := msg.Payload.(type) {
	case *pb.HubMessage_FileListRequest:
		resp, _ := filebrowser.ListDir(p.FileListRequest.Path)
		if resp != nil {
			resp.RequestId = p.FileListRequest.RequestId
			sendCh <- &pb.AgentMessage{
				Payload: &pb.AgentMessage_FileListResponse{
					FileListResponse: resp,
				},
			}
		}

	case *pb.HubMessage_FileReadRequest:
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

	case *pb.HubMessage_LogStreamRequest:
		req := p.LogStreamRequest
		stop := make(chan struct{})
		c.tailSessions[req.RequestId] = stop
		go logtailer.StartTail(req.Path, req.Grep, req.Offset, sendCh, req.RequestId, stop)
		log.Printf("started log tail: %s path=%s grep=%s", req.RequestId, req.Path, req.Grep)

	case *pb.HubMessage_LogStreamStop:
		if stop, ok := c.tailSessions[p.LogStreamStop.RequestId]; ok {
			close(stop)
			delete(c.tailSessions, p.LogStreamStop.RequestId)
			log.Printf("stopped log tail: %s", p.LogStreamStop.RequestId)
		}

	case *pb.HubMessage_FileUploadRequest:
		req := p.FileUploadRequest
		ack := c.dropzone.HandleChunk(req.RequestId, req.Filename, req.Chunk, req.Done)
		if ack != nil {
			sendCh <- &pb.AgentMessage{
				Payload: &pb.AgentMessage_FileUploadAck{
					FileUploadAck: ack,
				},
			}
		}

	default:
		log.Printf("unhandled hub message: %T", p)
	}
}

func (c *Client) connectAndStream(ctx context.Context) error {
	// Use TLS for hostname-based addresses (through reverse proxy), insecure for direct IP:port
	var creds grpc.DialOption
	if c.useTLS() {
		creds = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{}))
	} else {
		creds = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	conn, err := grpc.NewClient(c.hubAddr, creds)
	if err != nil {
		return fmt.Errorf("dial hub: %w", err)
	}
	defer conn.Close()

	client := pb.NewAgentServiceClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}

	if c.serverID == "" {
		if err := stream.Send(&pb.AgentMessage{
			Payload: &pb.AgentMessage_Register{
				Register: &pb.RegisterAgent{
					Token:        c.token,
					Hostname:     c.hostname,
					IpAddress:    c.ipAddress,
					Os:           c.os,
					AgentVersion: c.agentVersion,
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
	} else {
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
	}

	c.resetBackoff()
	log.Printf("connected to hub at %s", c.hubAddr)

	sendCh := make(chan *pb.AgentMessage, 16)
	defer func() {
		for id, stop := range c.tailSessions {
			close(stop)
			delete(c.tailSessions, id)
		}
		c.dropzone.Cleanup()
	}()
	hbStop := make(chan struct{})
	defer close(hbStop)
	go heartbeat.StartTicker(c.serverID, 10*time.Second, sendCh, hbStop)

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
