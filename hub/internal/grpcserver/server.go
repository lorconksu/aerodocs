package grpcserver

import (
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"

	"github.com/wyiu/aerodocs/hub/internal/connmgr"
	"github.com/wyiu/aerodocs/hub/internal/store"
	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

type Server struct {
	grpcServer *grpc.Server
	store      *store.Store
	connMgr    *connmgr.ConnManager
	addr       string
}

type Config struct {
	Addr    string
	Store   *store.Store
	ConnMgr *connmgr.ConnManager
}

func New(cfg Config) *Server {
	s := &Server{
		store:   cfg.Store,
		connMgr: cfg.ConnMgr,
		addr:    cfg.Addr,
	}
	s.grpcServer = grpc.NewServer()
	handler := &Handler{
		store:   cfg.Store,
		connMgr: cfg.ConnMgr,
	}
	pb.RegisterAgentServiceServer(s.grpcServer, handler)
	return s
}

func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("grpc listen: %w", err)
	}
	fmt.Printf("AeroDocs gRPC server listening on %s\n", s.addr)
	return s.grpcServer.Serve(lis)
}

func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
}

func (s *Server) ConnMgr() *connmgr.ConnManager {
	return s.connMgr
}

func (s *Server) StartHeartbeatMonitor(stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				s.sweepStaleConnections()
			}
		}
	}()
}

func (s *Server) sweepStaleConnections() {
	stale := s.connMgr.StaleConnections(30 * time.Second)
	for _, id := range stale {
		s.connMgr.Unregister(id)
		if err := s.store.UpdateServerStatus(id, "offline"); err != nil {
			log.Printf("heartbeat monitor: failed to mark %s offline: %v", id, err)
		}
		log.Printf("heartbeat monitor: marked %s offline (stale)", id)
	}
	activeIDs := s.connMgr.ActiveServerIDs()
	orphans, err := s.store.GetOnlineServersNotIn(activeIDs)
	if err != nil {
		log.Printf("heartbeat monitor: failed to query orphan servers: %v", err)
		return
	}
	for _, srv := range orphans {
		if err := s.store.UpdateServerStatus(srv.ID, "offline"); err != nil {
			log.Printf("heartbeat monitor: failed to mark orphan %s offline: %v", srv.ID, err)
		}
		log.Printf("heartbeat monitor: marked %s offline (orphan)", srv.ID)
	}
}
