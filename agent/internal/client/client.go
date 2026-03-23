package client

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/wyiu/aerodocs/agent/internal/heartbeat"
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

func (c *Client) connectAndStream(ctx context.Context) error {
	conn, err := grpc.NewClient(c.hubAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
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

	hbChan := make(chan *pb.AgentMessage, 1)
	hbStop := make(chan struct{})
	defer close(hbStop)
	go heartbeat.StartTicker(c.serverID, 10*time.Second, hbChan, hbStop)

	recvErr := make(chan error, 1)
	go func() {
		for {
			_, err := stream.Recv()
			if err == io.EOF {
				recvErr <- nil
				return
			}
			if err != nil {
				recvErr <- err
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-recvErr:
			return fmt.Errorf("receive error: %w", err)
		case msg := <-hbChan:
			if err := stream.Send(msg); err != nil {
				return fmt.Errorf("send heartbeat: %w", err)
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
