package grpcserver

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/wyiu/aerodocs/hub/internal/connmgr"
	"github.com/wyiu/aerodocs/hub/internal/model"
	"github.com/wyiu/aerodocs/hub/internal/store"
	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

type Handler struct {
	pb.UnimplementedAgentServiceServer
	store   *store.Store
	connMgr *connmgr.ConnManager
}

func (h *Handler) Connect(stream pb.AgentService_ConnectServer) error {
	msg, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to receive first message: %v", err)
	}

	var serverID string

	switch p := msg.Payload.(type) {
	case *pb.AgentMessage_Register:
		serverID, err = h.handleRegister(
			p.Register.Token,
			p.Register.Hostname,
			p.Register.IpAddress,
			p.Register.Os,
			p.Register.AgentVersion,
		)
		if err != nil {
			_ = stream.Send(&pb.HubMessage{
				Payload: &pb.HubMessage_RegisterAck{
					RegisterAck: &pb.RegisterAck{
						Success: false,
						Error:   err.Error(),
					},
				},
			})
			return status.Errorf(codes.Unauthenticated, "registration failed: %v", err)
		}
		if err := stream.Send(&pb.HubMessage{
			Payload: &pb.HubMessage_RegisterAck{
				RegisterAck: &pb.RegisterAck{
					Success:  true,
					ServerId: serverID,
				},
			},
		}); err != nil {
			return status.Errorf(codes.Internal, "failed to send register ack: %v", err)
		}

	case *pb.AgentMessage_Heartbeat:
		serverID = p.Heartbeat.ServerId
		if err := h.handleHeartbeat(serverID); err != nil {
			return status.Errorf(codes.NotFound, "heartbeat failed: %v", err)
		}
		if err := stream.Send(&pb.HubMessage{
			Payload: &pb.HubMessage_HeartbeatAck{
				HeartbeatAck: &pb.HeartbeatAck{
					Timestamp: time.Now().Unix(),
				},
			},
		}); err != nil {
			return status.Errorf(codes.Internal, "failed to send heartbeat ack: %v", err)
		}

	default:
		return status.Errorf(codes.InvalidArgument, "first message must be Register or Heartbeat")
	}

	h.connMgr.Register(serverID, stream)

	peerAddr := ""
	if p, ok := peer.FromContext(stream.Context()); ok {
		peerAddr = p.Addr.String()
	}
	h.store.LogAudit(model.AuditEntry{
		Action:    model.AuditServerConnected,
		Target:    &serverID,
		IPAddress: &peerAddr,
	})
	log.Printf("agent connected: %s from %s", serverID, peerAddr)

	defer func() {
		h.connMgr.Unregister(serverID)
		_ = h.store.UpdateServerStatus(serverID, "offline")
		h.store.LogAudit(model.AuditEntry{
			Action: model.AuditServerDisconnected,
			Target: &serverID,
		})
		log.Printf("agent disconnected: %s", serverID)
	}()

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch p := msg.Payload.(type) {
		case *pb.AgentMessage_Heartbeat:
			h.connMgr.UpdateHeartbeat(serverID)
			_ = h.store.UpdateServerLastSeen(serverID, nil)
			conn := h.connMgr.GetConn(serverID)
			if conn != nil {
				conn.SendMu.Lock()
				err := stream.Send(&pb.HubMessage{
					Payload: &pb.HubMessage_HeartbeatAck{
						HeartbeatAck: &pb.HeartbeatAck{
							Timestamp: time.Now().Unix(),
						},
					},
				})
				conn.SendMu.Unlock()
				if err != nil {
					return err
				}
			}
		default:
			log.Printf("unhandled message type from %s: %T", serverID, p)
		}
	}
}

func (h *Handler) handleRegister(rawToken, hostname, ip, os, agentVersion string) (string, error) {
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := fmt.Sprintf("%x", hash)
	srv, err := h.store.GetServerByToken(tokenHash)
	if err != nil {
		return "", fmt.Errorf("invalid or expired registration token")
	}
	if srv.TokenExpiresAt != nil {
		expiresAt, err := time.Parse("2006-01-02 15:04:05", *srv.TokenExpiresAt)
		if err == nil && time.Now().UTC().After(expiresAt) {
			return "", fmt.Errorf("registration token expired")
		}
	}
	if err := h.store.ActivateServer(srv.ID, hostname, ip, os, agentVersion); err != nil {
		return "", fmt.Errorf("failed to activate server: %w", err)
	}
	return srv.ID, nil
}

func (h *Handler) handleHeartbeat(serverID string) error {
	srv, err := h.store.GetServerByID(serverID)
	if err != nil {
		return fmt.Errorf("unknown server: %s", serverID)
	}
	if srv.Status != "online" {
		if err := h.store.UpdateServerStatus(serverID, "online"); err != nil {
			return fmt.Errorf("failed to update status: %w", err)
		}
	}
	_ = h.store.UpdateServerLastSeen(serverID, nil)
	return nil
}
