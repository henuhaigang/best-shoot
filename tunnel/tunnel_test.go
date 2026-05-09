package tunnel

import (
	"net"
	"testing"
	"time"

	"tunnel-project/protocol"
)

func TestTunnelNewAndServe(t *testing.T) {
	sess := &mockSession{}
	tun, err := New(1, 0, sess) // port 0 = OS picks random port
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer tun.Close()

	if tun.ID() != 1 {
		t.Errorf("ID = %d, want 1", tun.ID())
	}
	if tun.Addr() == nil {
		t.Error("Addr() returned nil")
	}

	done := make(chan error, 1)
	go func() { done <- tun.Serve() }()

	// Connect to verify it's listening
	conn, err := net.Dial("tcp", tun.Addr().String())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	conn.Close()

	tun.Close()
}

func TestTunnelHandleDataMessage(t *testing.T) {
	sess := &mockSession{}
	tun, err := New(1, 0, sess)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer tun.Close()

	go tun.Serve()

	// Connect as external client
	conn, err := net.Dial("tcp", tun.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Send data so handleConn registers the connection
	conn.Write([]byte("x"))
	time.Sleep(50 * time.Millisecond)

	// Get connID from the captured message
	msgs := sess.Messages()
	if len(msgs) == 0 {
		t.Fatal("no messages")
	}
	connID := extractConnID(msgs[0].Payload)

	// Simulate client sending data back through the tunnel
	err = tun.HandleMessage(&protocol.Message{
		Type:     protocol.MsgData,
		TunnelID: 1,
		Payload:  buildPayload(connID, []byte("from-client")),
	})
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}

	// Read from external connection
	buf := make([]byte, 64)
	conn.SetReadDeadline(time.Now().Add(time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(buf[:n]) != "from-client" {
		t.Errorf("got %q, want %q", buf[:n], "from-client")
	}
}

func TestTunnelHandleCloseMessage(t *testing.T) {
	sess := &mockSession{}
	tun, err := New(1, 0, sess)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer tun.Close()

	go tun.Serve()

	conn, err := net.Dial("tcp", tun.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	conn.Write([]byte("x"))
	time.Sleep(50 * time.Millisecond)

	msgs := sess.Messages()
	connID := extractConnID(msgs[0].Payload)

	// Close the connection from client side
	tun.HandleMessage(&protocol.Message{
		Type:     protocol.MsgCloseTunnel,
		TunnelID: 1,
		Payload:  connIDToBytes(connID),
	})

	// External connection should be closed
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, err = conn.Read(make([]byte, 1))
	if err == nil {
		t.Error("expected read error after close")
	}
}

func TestTunnelHandleShortPayload(t *testing.T) {
	sess := &mockSession{}
	tun, err := New(1, 0, sess)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer tun.Close()

	err = tun.HandleMessage(&protocol.Message{
		Type:     protocol.MsgData,
		TunnelID: 1,
		Payload:  []byte{0x01}, // too short
	})
	if err == nil {
		t.Error("expected error for short payload")
	}
}

func TestTunnelClose(t *testing.T) {
	sess := &mockSession{}
	tun, err := New(1, 0, sess)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- tun.Serve() }()

	tun.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("Serve did not exit after Close")
	}
}

// helpers

func extractConnID(payload []byte) uint32 {
	return uint32(payload[0])<<24 | uint32(payload[1])<<16 | uint32(payload[2])<<8 | uint32(payload[3])
}

func connIDToBytes(id uint32) []byte {
	return []byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)}
}

func buildPayload(connID uint32, data []byte) []byte {
	payload := make([]byte, 4+len(data))
	copy(payload, connIDToBytes(connID))
	copy(payload[4:], data)
	return payload
}
