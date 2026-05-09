package tunnel

import (
	"net"
	"sync"
	"testing"
	"time"

	"tunnel-project/protocol"
)

type mockSession struct {
	mu   sync.Mutex
	msgs []*protocol.Message
}

func (m *mockSession) Send(msg *protocol.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs = append(m.msgs, msg)
	return nil
}

func (m *mockSession) Messages() []*protocol.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]*protocol.Message, len(m.msgs))
	copy(cp, m.msgs)
	return cp
}

func TestProxyListenerAcceptAndRead(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	sess := &mockSession{}
	pl := NewProxyListener(ln, 1, sess)
	go pl.Serve()
	defer pl.Close()

	// Connect as an external client
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Send some data
	conn.Write([]byte("hello"))

	// Wait for messages to arrive
	time.Sleep(50 * time.Millisecond)

	msgs := sess.Messages()
	if len(msgs) == 0 {
		t.Fatal("no messages received")
	}

	// First message should be MsgData with connID prefix
	msg := msgs[0]
	if msg.Type != protocol.MsgData {
		t.Errorf("Type = %d, want %d", msg.Type, protocol.MsgData)
	}
	if msg.TunnelID != 1 {
		t.Errorf("TunnelID = %d, want 1", msg.TunnelID)
	}
	if len(msg.Payload) < 4+5 {
		t.Errorf("Payload too short: %d bytes", len(msg.Payload))
	}
	// Data portion should be "hello"
	if string(msg.Payload[4:]) != "hello" {
		t.Errorf("Data = %q, want %q", msg.Payload[4:], "hello")
	}
}

func TestProxyListenerDeliver(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	sess := &mockSession{}
	pl := NewProxyListener(ln, 1, sess)
	go pl.Serve()
	defer pl.Close()

	// Connect as external client
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Send data so handleConn registers the connection
	conn.Write([]byte("x"))
	time.Sleep(50 * time.Millisecond)

	// Get the connID from the first message
	msgs := sess.Messages()
	if len(msgs) == 0 {
		t.Fatal("no messages")
	}
	connID := uint32(msgs[0].Payload[0])<<24 | uint32(msgs[0].Payload[1])<<16 | uint32(msgs[0].Payload[2])<<8 | uint32(msgs[0].Payload[3])

	// Deliver data back through the tunnel
	if err := pl.Deliver(connID, []byte("world")); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	// Read from the external connection
	buf := make([]byte, 32)
	conn.SetReadDeadline(time.Now().Add(time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(buf[:n]) != "world" {
		t.Errorf("got %q, want %q", buf[:n], "world")
	}
}

func TestProxyListenerCloseConn(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	sess := &mockSession{}
	pl := NewProxyListener(ln, 1, sess)
	go pl.Serve()
	defer pl.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	conn.Write([]byte("x"))
	time.Sleep(50 * time.Millisecond)

	msgs := sess.Messages()
	connID := uint32(msgs[0].Payload[0])<<24 | uint32(msgs[0].Payload[1])<<16 | uint32(msgs[0].Payload[2])<<8 | uint32(msgs[0].Payload[3])

	pl.CloseConn(connID)

	// External connection should be closed
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, err = conn.Read(make([]byte, 1))
	if err == nil {
		t.Error("expected read error after CloseConn")
	}
}

func TestProxyListenerClose(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	sess := &mockSession{}
	pl := NewProxyListener(ln, 1, sess)

	done := make(chan struct{})
	go func() {
		pl.Serve()
		close(done)
	}()

	pl.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("Serve did not exit after Close")
	}
}

func TestConnIDBytes(t *testing.T) {
	b := connIDBytes(0x01020304)
	if b[0] != 1 || b[1] != 2 || b[2] != 3 || b[3] != 4 {
		t.Errorf("connIDBytes = %v, want [1 2 3 4]", b)
	}
}
