package server

import (
	"fmt"
	"net"
	"sync"
	"time"

	"tunnel-project/protocol"
)

// Session is a server-side view of a connected client.
type Session struct {
	mu       sync.RWMutex
	clientID string
	conn     net.Conn
	proto    *protocol.Session
	created  time.Time
	lastSeen time.Time
	done     chan struct{}
}

// NewSession wraps a protocol session with server metadata.
func NewSession(conn net.Conn) *Session {
	now := time.Now()
	return &Session{
		conn:     conn,
		proto:    protocol.NewSession(conn),
		created:  now,
		lastSeen: now,
		done:     make(chan struct{}),
	}
}

// ClientID returns the registered client identifier.
func (s *Session) ClientID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clientID
}

// SetClientID sets the client identifier. Called once during registration.
func (s *Session) SetClientID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clientID = id
}

// Conn returns the underlying connection.
func (s *Session) Conn() net.Conn {
	return s.conn
}

// RemoteAddr returns the remote network address.
func (s *Session) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

// Created returns when the session was established.
func (s *Session) Created() time.Time {
	return s.created
}

// Touch updates the last-seen timestamp to now.
func (s *Session) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSeen = time.Now()
}

// LastSeen returns the last time a message was received.
func (s *Session) LastSeen() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSeen
}

// Send writes a message to the client.
func (s *Session) Send(msg *protocol.Message) error {
	return s.proto.Send(msg)
}

// Receive reads the next message from the client. Blocks until available.
func (s *Session) Receive() (*protocol.Message, error) {
	return s.proto.Receive()
}

// Close closes the session and signals done.
func (s *Session) Close() error {
	err := s.conn.Close()
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	return err
}

// Done returns a channel that is closed when the session ends.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// ReadLoop reads messages and passes them to fn until an error occurs.
func (s *Session) ReadLoop(fn func(*protocol.Message) error) error {
	return s.proto.ReadLoop(fn)
}

// String implements fmt.Stringer for logging.
func (s *Session) String() string {
	return fmt.Sprintf("client=%s addr=%s", s.ClientID(), s.RemoteAddr())
}
