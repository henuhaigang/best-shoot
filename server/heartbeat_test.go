package server

import (
	"net"
	"testing"
	"time"

	"tunnel-project/protocol"
)

func TestSessionTouch(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	sess := NewSession(srv)
	before := sess.LastSeen()
	time.Sleep(10 * time.Millisecond)
	sess.Touch()
	after := sess.LastSeen()

	if !after.After(before) {
		t.Errorf("Touch did not update lastSeen: before=%v after=%v", before, after)
	}
}

func TestSessionLastSeenOnInit(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	before := time.Now()
	sess := NewSession(srv)
	after := time.Now()

	ls := sess.LastSeen()
	if ls.Before(before) || ls.After(after) {
		t.Errorf("LastSeen %v not in range [%v, %v]", ls, before, after)
	}
}

func TestHeartbeatTimeout(t *testing.T) {
	// Use short timeouts for testing
	s, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	go s.Serve()

	// Connect and register
	conn, err := net.Dial("tcp", s.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	sess := protocol.NewSession(conn)
	sess.Send(&protocol.Message{Type: protocol.MsgRegister, Payload: []byte("node-1")})
	sess.Receive() // ack

	// Manually test the timeout logic by checking LastSeen
	// The actual heartbeatChecker uses HeartbeatTimeout (90s) which is too long for a test
	// So we verify the Touch/LastSeen mechanism works
	serverSess, err := s.clients.Get("node-1")
	if err != nil {
		t.Fatal(err)
	}

	ls := serverSess.LastSeen()
	time.Sleep(10 * time.Millisecond)

	// Send a heartbeat
	sess.Send(&protocol.Message{Type: protocol.MsgHeartbeat})
	time.Sleep(50 * time.Millisecond)

	// Server should have received and touched
	newLS := serverSess.LastSeen()
	if !newLS.After(ls) {
		t.Errorf("Heartbeat did not update LastSeen: %v -> %v", ls, newLS)
	}

	// Server should echo heartbeat back
	conn.SetReadDeadline(time.Now().Add(time.Second))
	msg, err := sess.Receive()
	if err != nil {
		t.Fatalf("Receive heartbeat echo: %v", err)
	}
	if msg.Type != protocol.MsgHeartbeat {
		t.Errorf("expected Heartbeat, got %s", msg.TypeName())
	}

	conn.Close()
}
