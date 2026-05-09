package server

import (
	"net"
	"testing"
)

func TestSessionClientID(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	sess := NewSession(srv)
	if sess.ClientID() != "" {
		t.Errorf("expected empty ClientID, got %q", sess.ClientID())
	}
	sess.SetClientID("node-1")
	if sess.ClientID() != "node-1" {
		t.Errorf("ClientID = %q, want %q", sess.ClientID(), "node-1")
	}
}

func TestSessionString(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	sess := NewSession(srv)
	sess.SetClientID("node-1")
	s := sess.String()
	if s == "" {
		t.Error("String() returned empty")
	}
}

func TestSessionDone(t *testing.T) {
	srv, cli := net.Pipe()
	defer cli.Close()

	sess := NewSession(srv)

	select {
	case <-sess.Done():
		t.Error("Done should not be closed before Close")
	default:
	}

	sess.Close()

	select {
	case <-sess.Done():
	default:
		t.Error("Done should be closed after Close")
	}
}

func TestSessionCreated(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	sess := NewSession(srv)
	if sess.Created().IsZero() {
		t.Error("Created() returned zero time")
	}
}
