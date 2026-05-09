package server

import (
	"fmt"
	"net"
	"sync"
	"time"

	"tunnel-project/logger"
	"tunnel-project/protocol"
	"tunnel-project/tunnel"
)

// Default heartbeat timeouts.
const (
	HeartbeatInterval = 30 * time.Second // how often server expects a heartbeat
	HeartbeatTimeout  = 90 * time.Second // disconnect after no activity this long
)

// Server accepts client connections, manages registration, and routes tunnel messages.
type Server struct {
	ln       net.Listener
	clients  *ClientManager
	tunnels  *TunnelManager
	mu       sync.RWMutex
	sessions map[string][]*tunnel.Tunnel // clientID -> tunnels
	log      *logger.Logger
}

// New creates a server that will listen on addr.
func New(addr string) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	return &Server{
		ln:       ln,
		clients:  NewClientManager(),
		tunnels:  NewTunnelManager(),
		sessions: make(map[string][]*tunnel.Tunnel),
		log:      logger.Default("server"),
	}, nil
}

// SetLogger sets the logger for the server.
func (s *Server) SetLogger(l *logger.Logger) {
	s.log = l
}

// Addr returns the listener address.
func (s *Server) Addr() net.Addr {
	return s.ln.Addr()
}

// Serve accepts connections in a loop. Blocks until Close is called.
func (s *Server) Serve() error {
	s.log.Info("listening", logger.F("addr", s.ln.Addr()))
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return err
		}
		s.log.Debug("new connection", logger.F("remote", conn.RemoteAddr()))
		go s.handleConn(conn)
	}
}

// Close shuts down the server and all clients/tunnels.
func (s *Server) Close() error {
	s.log.Info("shutting down")
	err := s.ln.Close()
	s.tunnels.CloseAll()
	s.mu.Lock()
	s.sessions = make(map[string][]*tunnel.Tunnel)
	s.mu.Unlock()
	return err
}

// handleConn runs the full lifecycle of a client connection.
func (s *Server) handleConn(conn net.Conn) {
	sess := NewSession(conn)
	defer func() {
		s.cleanupClient(sess)
		sess.Close()
		s.log.Info("disconnected", logger.F("session", sess.String()))
	}()

	// Wait for registration
	msg, err := sess.Receive()
	if err != nil {
		s.log.Warn("read error before registration", logger.F("remote", conn.RemoteAddr()), logger.F("error", err))
		return
	}
	if msg.Type != protocol.MsgRegister {
		s.log.Warn("expected Register", logger.F("remote", conn.RemoteAddr()), logger.F("got", msg.TypeName()))
		return
	}

	clientID := string(msg.Payload)
	if err := s.clients.Register(clientID, sess); err != nil {
		s.log.Warn("register rejected", logger.F("clientID", clientID), logger.F("error", err))
		return
	}

	// Send ack
	if err := sess.Send(&protocol.Message{Type: protocol.MsgRegisterAck}); err != nil {
		s.log.Error("send ack failed", logger.F("error", err))
		return
	}
	s.log.Info("client connected", logger.F("clientID", clientID))

	// Start heartbeat timeout checker
	go s.heartbeatChecker(sess)

	// Read loop
	for {
		msg, err := sess.Receive()
		if err != nil {
			s.log.Info("client read error", logger.F("clientID", clientID), logger.F("error", err))
			return
		}
		sess.Touch()
		if err := s.handleMessage(clientID, sess, msg); err != nil {
			s.log.Warn("handle error", logger.F("clientID", clientID), logger.F("error", err))
		}
	}
}

// handleMessage dispatches a message from a registered client.
func (s *Server) handleMessage(clientID string, sess *Session, msg *protocol.Message) error {
	switch msg.Type {
	case protocol.MsgOpenTunnel:
		return s.handleOpenTunnel(clientID, sess, msg)
	case protocol.MsgData, protocol.MsgCloseTunnel:
		return s.routeToTunnel(msg)
	case protocol.MsgHeartbeat:
		return sess.Send(&protocol.Message{Type: protocol.MsgHeartbeat})
	default:
		return fmt.Errorf("unexpected message type: %s", msg.TypeName())
	}
}

// handleOpenTunnel creates a new tunnel for the client.
func (s *Server) handleOpenTunnel(clientID string, sess *Session, msg *protocol.Message) error {
	if len(msg.Payload) < 2 {
		return fmt.Errorf("OpenTunnel payload too short")
	}
	port := uint16(msg.Payload[0])<<8 | uint16(msg.Payload[1])

	tun, err := s.tunnels.Open(clientID, port, sess)
	if err != nil {
		s.log.Warn("tunnel open failed", logger.F("clientID", clientID), logger.F("port", port), logger.F("error", err))
		return sess.Send(&protocol.Message{
			Type:     protocol.MsgOpenTunnelAck,
			TunnelID: msg.TunnelID,
		})
	}

	// Track tunnel for cleanup
	s.mu.Lock()
	s.sessions[clientID] = append(s.sessions[clientID], tun)
	s.mu.Unlock()

	// Send ack with tunnel ID
	if err := sess.Send(&protocol.Message{
		Type:     protocol.MsgOpenTunnelAck,
		TunnelID: tun.ID(),
	}); err != nil {
		tun.Close()
		return err
	}

	s.log.Info("tunnel opened", logger.F("tunnelID", tun.ID()), logger.F("clientID", clientID), logger.F("port", port))

	// Serve external connections in background
	go func() {
		if err := tun.Serve(); err != nil {
			s.log.Error("tunnel serve error", logger.F("tunnelID", tun.ID()), logger.F("error", err))
		}
	}()

	return nil
}

// routeToTunnel sends a data/close message to the correct tunnel.
func (s *Server) routeToTunnel(msg *protocol.Message) error {
	tun, err := s.tunnels.Get(msg.TunnelID)
	if err != nil {
		return err
	}
	return tun.HandleMessage(msg)
}

// heartbeatChecker closes the session if no activity for HeartbeatTimeout.
func (s *Server) heartbeatChecker(sess *Session) {
	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sess.Done():
			return
		case <-ticker.C:
			if time.Since(sess.LastSeen()) > HeartbeatTimeout {
				s.log.Warn("heartbeat timeout", logger.F("clientID", sess.ClientID()))
				sess.Close()
				return
			}
		}
	}
}

// cleanupClient removes a client and all its tunnels.
func (s *Server) cleanupClient(sess *Session) {
	clientID := sess.ClientID()
	if clientID == "" {
		return
	}

	s.mu.Lock()
	tunnels := s.sessions[clientID]
	delete(s.sessions, clientID)
	s.mu.Unlock()

	for _, t := range tunnels {
		s.tunnels.Remove(t.ID())
		t.Close()
	}
	s.clients.Unregister(clientID)
	s.log.Info("cleaned up client", logger.F("clientID", clientID), logger.F("tunnels", len(tunnels)))
}
