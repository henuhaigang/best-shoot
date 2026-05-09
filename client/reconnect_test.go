package client

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"tunnel-project/protocol"
)

func TestNextDelay(t *testing.T) {
	d := ReconnectBaseDelay
	for i := 0; i < 10; i++ {
		d = nextDelay(d)
	}
	if d != ReconnectMaxDelay {
		t.Errorf("nextDelay capped at %v, want %v", d, ReconnectMaxDelay)
	}
}

func TestRunReconnects(t *testing.T) {
	// Start a server that accepts, registers, then disconnects
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()

	var connects atomic.Int32
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			connects.Add(1)
			sess := protocol.NewSession(conn)
			msg, err := sess.Receive()
			if err != nil {
				conn.Close()
				continue
			}
			if msg.Type == protocol.MsgRegister {
				sess.Send(&protocol.Message{Type: protocol.MsgRegisterAck})
			}
			// Disconnect after a short delay
			time.Sleep(50 * time.Millisecond)
			conn.Close()
		}
	}()

	c := New(addr, "node-1")
	done := make(chan error, 1)
	go func() { done <- c.Run() }()

	// Wait for a few reconnect cycles (base delay is 1s)
	time.Sleep(2500 * time.Millisecond)
	c.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after Close")
	}

	if connects.Load() < 2 {
		t.Errorf("expected multiple connects, got %d", connects.Load())
	}
}

func TestRunExitsOnClose(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	c := New(ln.Addr().String(), "node-1")
	done := make(chan error, 1)
	go func() { done <- c.Run() }()

	time.Sleep(50 * time.Millisecond)
	c.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after Close")
	}
}

func TestCloseConns(t *testing.T) {
	c := New("127.0.0.1:0", "node-1")

	srv1, cli1 := net.Pipe()
	srv2, cli2 := net.Pipe()
	defer srv1.Close()
	defer srv2.Close()

	c.conns.store(1, 1, cli1)
	c.conns.store(1, 2, cli2)

	c.closeConns()

	// Connections should be closed
	buf := make([]byte, 1)
	_, err := srv1.Read(buf)
	if err == nil {
		t.Error("expected error after closeConns")
	}
	_, err = srv2.Read(buf)
	if err == nil {
		t.Error("expected error after closeConns")
	}
}
