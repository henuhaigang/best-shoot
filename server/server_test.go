package server

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"tunnel-project/protocol"
)

func dialAndRegister(t *testing.T, addr, clientID string) *protocol.Session {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	sess := protocol.NewSession(conn)

	if err := sess.Send(&protocol.Message{
		Type:    protocol.MsgRegister,
		Payload: []byte(clientID),
	}); err != nil {
		t.Fatal(err)
	}

	msg, err := sess.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != protocol.MsgRegisterAck {
		t.Fatalf("expected RegisterAck, got %s", msg.TypeName())
	}
	return sess
}

func TestServerRegister(t *testing.T) {
	s, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	go s.Serve()

	sess := dialAndRegister(t, s.Addr().String(), "node-1")
	defer sess.Close()

	if s.clients.Count() != 1 {
		t.Errorf("Count = %d, want 1", s.clients.Count())
	}
}

func TestServerDuplicateRegister(t *testing.T) {
	s, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	go s.Serve()

	sess1 := dialAndRegister(t, s.Addr().String(), "node-1")
	defer sess1.Close()

	// Second registration with same ID should fail (connection closed)
	conn2, err := net.Dial("tcp", s.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	sess2 := protocol.NewSession(conn2)
	sess2.Send(&protocol.Message{Type: protocol.MsgRegister, Payload: []byte("node-1")})

	// Should get disconnected (no ack)
	conn2.SetReadDeadline(time.Now().Add(time.Second))
	_, err = sess2.Receive()
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
	sess2.Close()
}

func TestServerMultipleClients(t *testing.T) {
	s, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	go s.Serve()

	s1 := dialAndRegister(t, s.Addr().String(), "node-1")
	defer s1.Close()
	s2 := dialAndRegister(t, s.Addr().String(), "node-2")
	defer s2.Close()

	if s.clients.Count() != 2 {
		t.Errorf("Count = %d, want 2", s.clients.Count())
	}
}

func TestServerOpenTunnel(t *testing.T) {
	s, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	go s.Serve()

	sess := dialAndRegister(t, s.Addr().String(), "node-1")
	defer sess.Close()

	// Request a tunnel on a random port (0 = OS picks)
	if err := sess.Send(&protocol.Message{
		Type:    protocol.MsgOpenTunnel,
		Payload: []byte{0, 0}, // port 0
	}); err != nil {
		t.Fatal(err)
	}

	msg, err := sess.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != protocol.MsgOpenTunnelAck {
		t.Fatalf("expected OpenTunnelAck, got %s", msg.TypeName())
	}
	if msg.TunnelID == 0 {
		t.Error("expected non-zero tunnel ID")
	}

	if s.tunnels.Count() != 1 {
		t.Errorf("tunnel count = %d, want 1", s.tunnels.Count())
	}
}

func TestServerDataRouting(t *testing.T) {
	s, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	go s.Serve()

	// Start a local echo service
	localLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer localLn.Close()
	port := uint16(localLn.Addr().(*net.TCPAddr).Port)

	go func() {
		conn, err := localLn.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 64)
		n, _ := conn.Read(buf)
		conn.Write(buf[:n])
	}()

	sess := dialAndRegister(t, s.Addr().String(), "node-1")
	defer sess.Close()

	// Open tunnel
	portPayload := make([]byte, 2)
	binary.BigEndian.PutUint16(portPayload, port)
	if err := sess.Send(&protocol.Message{
		Type:    protocol.MsgOpenTunnel,
		Payload: portPayload,
	}); err != nil {
		t.Fatal(err)
	}

	ack, err := sess.Receive()
	if err != nil {
		t.Fatal(err)
	}
	tunID := ack.TunnelID

	// Connect to the tunnel's external port
	tun, err := s.tunnels.Get(tunID)
	if err != nil {
		t.Fatal(err)
	}

	extConn, err := net.Dial("tcp", tun.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer extConn.Close()

	// Send data from external connection
	extConn.Write([]byte("hello"))

	// Client should receive MsgData
	msg, err := sess.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != protocol.MsgData {
		t.Fatalf("expected Data, got %s", msg.TypeName())
	}
	if string(msg.Payload[4:]) != "hello" {
		t.Errorf("data = %q, want %q", msg.Payload[4:], "hello")
	}

	// Send data back through the tunnel (echo)
	payload := make([]byte, 4+5)
	copy(payload[:4], msg.Payload[:4]) // connID
	copy(payload[4:], "world")
	sess.Send(&protocol.Message{
		Type:     protocol.MsgData,
		TunnelID: tunID,
		Payload:  payload,
	})

	// External connection should receive "world"
	buf := make([]byte, 64)
	extConn.SetReadDeadline(time.Now().Add(time.Second))
	n, err := extConn.Read(buf)
	if err != nil {
		t.Fatalf("ext read: %v", err)
	}
	if string(buf[:n]) != "world" {
		t.Errorf("ext got %q, want %q", buf[:n], "world")
	}
}

func TestServerDisconnectCleanup(t *testing.T) {
	s, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	go s.Serve()

	sess := dialAndRegister(t, s.Addr().String(), "node-1")

	// Open a tunnel
	sess.Send(&protocol.Message{Type: protocol.MsgOpenTunnel, Payload: []byte{0, 0}})
	sess.Receive() // ack

	if s.tunnels.Count() != 1 {
		t.Errorf("tunnel count = %d, want 1", s.tunnels.Count())
	}

	// Disconnect
	sess.Close()

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	if s.clients.Count() != 0 {
		t.Errorf("client count = %d, want 0", s.clients.Count())
	}
	if s.tunnels.Count() != 0 {
		t.Errorf("tunnel count = %d, want 0", s.tunnels.Count())
	}
}
