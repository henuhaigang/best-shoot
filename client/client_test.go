package client

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"tunnel-project/protocol"
)

// mockServer simulates a tunnel server for testing.
type mockServer struct {
	ln      net.Listener
	session *protocol.Session
}

func newMockServer(t *testing.T) (*mockServer, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return &mockServer{ln: ln}, ln.Addr().String()
}

func (s *mockServer) accept(t *testing.T) *protocol.Session {
	t.Helper()
	conn, err := s.ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	s.session = protocol.NewSession(conn)
	return s.session
}

func (s *mockServer) close() { s.ln.Close() }

func TestClientRegister(t *testing.T) {
	srv, addr := newMockServer(t)
	defer srv.close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		sess := srv.accept(t)
		msg, err := sess.Receive()
		if err != nil {
			t.Errorf("Receive: %v", err)
			return
		}
		if msg.Type != protocol.MsgRegister {
			t.Errorf("Type = %d, want %d", msg.Type, protocol.MsgRegister)
		}
		if string(msg.Payload) != "node-1" {
			t.Errorf("Payload = %q, want %q", msg.Payload, "node-1")
		}
		sess.Send(&protocol.Message{Type: protocol.MsgRegisterAck})
	}()

	c := New(addr, "node-1")
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	<-done
}

func TestClientBasicForward(t *testing.T) {
	// Simplified test: just verify data round-trips through the tunnel
	localLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer localLn.Close()
	port := uint16(localLn.Addr().(*net.TCPAddr).Port)

	localDone := make(chan string, 1)
	go func() {
		conn, err := localLn.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 64)
		n, _ := conn.Read(buf)
		conn.Write(buf[:n])
		localDone <- string(buf[:n])
	}()

	srv, addr := newMockServer(t)
	defer srv.close()

	serverDone := make(chan string, 1)
	go func() {
		sess := srv.accept(t)
		sess.Receive() // register
		sess.Send(&protocol.Message{Type: protocol.MsgRegisterAck})

		// Open tunnel
		portPayload := make([]byte, 2)
		binary.BigEndian.PutUint16(portPayload, port)
		sess.Send(&protocol.Message{
			Type:     protocol.MsgOpenTunnel,
			TunnelID: 1,
			Payload:  portPayload,
		})

		// Read ack
		ack, _ := sess.Receive()
		if ack.Type != protocol.MsgOpenTunnelAck {
			t.Errorf("expected OpenTunnelAck, got %s", ack.TypeName())
			return
		}

		// Send data to local service
		sess.Send(&protocol.Message{
			Type:     protocol.MsgData,
			TunnelID: 1,
			Payload:  append([]byte{0, 0, 0, 1}, []byte("ping")...),
		})

		// Read echoed data
		msg, err := sess.Receive()
		if err != nil {
			serverDone <- "error: " + err.Error()
			return
		}
		serverDone <- string(msg.Payload[4:])
	}()

	c := New(addr, "node-1")
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()
	go c.Serve()

	select {
	case got := <-serverDone:
		if got != "ping" {
			t.Errorf("server got %q, want %q", got, "ping")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}

	select {
	case got := <-localDone:
		if got != "ping" {
			t.Errorf("local got %q, want %q", got, "ping")
		}
	case <-time.After(time.Second):
		t.Fatal("local timeout")
	}
}

func TestClientOpenTunnelAndForward(t *testing.T) {
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
		for {
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			conn.Write(buf[:n]) // echo
		}
	}()

	srv, addr := newMockServer(t)
	defer srv.close()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		sess := srv.accept(t)

		// Read register
		sess.Receive()
		sess.Send(&protocol.Message{Type: protocol.MsgRegisterAck})

		// Send OpenTunnel
		portPayload := make([]byte, 2)
		binary.BigEndian.PutUint16(portPayload, port)
		sess.Send(&protocol.Message{
			Type:     protocol.MsgOpenTunnel,
			TunnelID: 1,
			Payload:  portPayload,
		})

		// Read OpenTunnelAck
		ack, err := sess.Receive()
		if err != nil {
			t.Errorf("Receive ack: %v", err)
			return
		}
		if ack.Type != protocol.MsgOpenTunnelAck {
			t.Errorf("expected OpenTunnelAck, got %s", ack.TypeName())
		}

		// Send data to local service through tunnel
		sess.Send(&protocol.Message{
			Type:     protocol.MsgData,
			TunnelID: 1,
			Payload:  append([]byte{0, 0, 0, 1}, []byte("hello")...),
		})

		// Read echoed data back from client
		msg, err := sess.Receive()
		if err != nil {
			t.Errorf("Receive data: %v", err)
			return
		}
		if msg.Type != protocol.MsgData {
			t.Errorf("expected Data, got %s", msg.TypeName())
		}
		if string(msg.Payload[4:]) != "hello" {
			t.Errorf("data = %q, want %q", msg.Payload[4:], "hello")
		}

		// Send second round of data
		connID := extractConnID(msg.Payload)
		payload := make([]byte, 4+len("world"))
		copy(payload, connIDBytes(connID))
		copy(payload[4:], "world")
		sess.Send(&protocol.Message{
			Type:     protocol.MsgData,
			TunnelID: 1,
			Payload:  payload,
		})

		// Read the echoed "world" back
		msg2, err := sess.Receive()
		if err != nil {
			t.Errorf("Receive echoed data: %v", err)
			return
		}
		if string(msg2.Payload[4:]) != "world" {
			t.Errorf("echoed data = %q, want %q", msg2.Payload[4:], "world")
		}
	}()

	c := New(addr, "node-1")
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	go c.Serve()

	select {
	case <-serverDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for round trip")
	}
}

func TestClientConnManager(t *testing.T) {
	cm := newConnManager()
	srv, cli := net.Pipe()

	cm.store(1, 1, srv)
	conn, ok := cm.load(1, 1)
	if !ok {
		t.Fatal("expected conn")
	}
	if conn != srv {
		t.Error("wrong conn")
	}

	cm.delete(1, 1)
	_, ok = cm.load(1, 1)
	if ok {
		t.Error("expected deleted")
	}

	cli.Close()
}

func TestConnKey(t *testing.T) {
	k := connKey(1, 2)
	if k != "1:2" {
		t.Errorf("connKey = %q, want %q", k, "1:2")
	}
}

func TestConnIDBytes(t *testing.T) {
	b := connIDBytes(0x0A0B0C0D)
	if b[0] != 0x0A || b[1] != 0x0B || b[2] != 0x0C || b[3] != 0x0D {
		t.Errorf("connIDBytes = %v", b)
	}
}
