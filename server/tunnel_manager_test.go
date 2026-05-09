package server

import (
	"errors"
	"net"
	"testing"

	"tunnel-project/protocol"
	"tunnel-project/tunnel"
)

type fakeSender struct{}

func (f *fakeSender) Send(msg *protocol.Message) error { return nil }

func TestTunnelManagerOpen(t *testing.T) {
	tm := NewTunnelManager()
	sess := &fakeSender{}

	tun, err := tm.Open("node-1", 0, sess)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer tm.Close(tun.ID())

	if tun.ID() != 1 {
		t.Errorf("ID = %d, want 1", tun.ID())
	}
	if tm.Count() != 1 {
		t.Errorf("Count = %d, want 1", tm.Count())
	}
}

func TestTunnelManagerDuplicatePort(t *testing.T) {
	tm := NewTunnelManager()
	sess := &fakeSender{}

	// Use a specific port for duplicate testing
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := uint16(ln.Addr().(*net.TCPAddr).Port)
	ln.Close()

	tun, err := tm.Open("node-1", port, sess)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer tm.Close(tun.ID())

	_, err = tm.Open("node-1", port, sess)
	if !errors.Is(err, ErrTunnelExists) {
		t.Errorf("expected ErrTunnelExists, got %v", err)
	}
}

func TestTunnelManagerClose(t *testing.T) {
	tm := NewTunnelManager()
	sess := &fakeSender{}

	tun, _ := tm.Open("node-1", 0, sess)
	tm.Close(tun.ID())

	if tm.Count() != 0 {
		t.Errorf("Count = %d, want 0", tm.Count())
	}
}

func TestTunnelManagerGet(t *testing.T) {
	tm := NewTunnelManager()
	sess := &fakeSender{}

	tun, _ := tm.Open("node-1", 0, sess)
	defer tm.Close(tun.ID())

	got, err := tm.Get(tun.ID())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != tun {
		t.Error("Get returned different tunnel")
	}
}

func TestTunnelManagerGetNotFound(t *testing.T) {
	tm := NewTunnelManager()
	_, err := tm.Get(999)
	if !errors.Is(err, ErrTunnelNotFound) {
		t.Errorf("expected ErrTunnelNotFound, got %v", err)
	}
}

func TestTunnelManagerAutoIncrement(t *testing.T) {
	tm := NewTunnelManager()
	sess := &fakeSender{}

	t1, err := tm.Open("a", 0, sess)
	if err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	defer tm.Close(t1.ID())

	t2, err := tm.Open("a", 0, sess)
	if err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	defer tm.Close(t2.ID())

	if t2.ID() != t1.ID()+1 {
		t.Errorf("IDs not incrementing: %d, %d", t1.ID(), t2.ID())
	}
}

func TestTunnelManagerCloseAll(t *testing.T) {
	tm := NewTunnelManager()
	sess := &fakeSender{}

	tm.Open("a", 0, sess)
	tm.Open("b", 0, sess)

	if tm.Count() != 2 {
		t.Errorf("Count = %d, want 2", tm.Count())
	}

	tm.CloseAll()
	if tm.Count() != 0 {
		t.Errorf("Count = %d, want 0 after CloseAll", tm.Count())
	}
}

func TestTunnelManagerEach(t *testing.T) {
	tm := NewTunnelManager()
	sess := &fakeSender{}

	t1, _ := tm.Open("a", 0, sess)
	t2, _ := tm.Open("b", 0, sess)
	defer tm.Close(t1.ID())
	defer tm.Close(t2.ID())

	seen := 0
	tm.Each(func(tun *tunnel.Tunnel) {
		seen++
	})
	if seen != 2 {
		t.Errorf("Each visited %d tunnels, want 2", seen)
	}
}
