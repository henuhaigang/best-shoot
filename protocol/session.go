package protocol

import (
	"net"
)

// Session is a bidirectional message exchange over a single connection.
type Session struct {
	conn    net.Conn
	encoder *Encoder
	decoder *Decoder
}

// NewSession wraps a connection with a session.
func NewSession(conn net.Conn) *Session {
	return &Session{
		conn:    conn,
		encoder: NewEncoder(conn),
		decoder: NewDecoder(conn),
	}
}

// Send writes a message to the connection.
func (s *Session) Send(msg *Message) error {
	return s.encoder.Send(msg)
}

// Receive reads the next message from the connection. Blocks until available.
func (s *Session) Receive() (*Message, error) {
	return s.decoder.Receive()
}

// Close closes the underlying connection.
func (s *Session) Close() error {
	return s.conn.Close()
}

// Conn returns the underlying connection.
func (s *Session) Conn() net.Conn {
	return s.conn
}

// RemoteAddr returns the remote network address.
func (s *Session) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

// SendAndReceive sends a message and waits for the response.
func (s *Session) SendAndReceive(msg *Message) (*Message, error) {
	if err := s.Send(msg); err != nil {
		return nil, err
	}
	return s.Receive()
}

// ReadLoop reads messages and passes them to fn until an error (including io.EOF) occurs.
func (s *Session) ReadLoop(fn func(*Message) error) error {
	for {
		msg, err := s.Receive()
		if err != nil {
			return err
		}
		if err := fn(msg); err != nil {
			return err
		}
	}
}
