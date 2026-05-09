package server

import (
	"errors"
	"net"
	"sync"
	"testing"
)

func newTestSession(t *testing.T) (*Session, func()) {
	t.Helper()
	srv, cli := net.Pipe()
	cleanup := func() { srv.Close(); cli.Close() }
	return NewSession(srv), cleanup
}

func TestClientManagerRegister(t *testing.T) {
	cm := NewClientManager()
	sess, cleanup := newTestSession(t)
	defer cleanup()

	if err := cm.Register("node-1", sess); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if sess.ClientID() != "node-1" {
		t.Errorf("ClientID = %q, want %q", sess.ClientID(), "node-1")
	}
	if cm.Count() != 1 {
		t.Errorf("Count = %d, want 1", cm.Count())
	}
}

func TestClientManagerDuplicate(t *testing.T) {
	cm := NewClientManager()
	s1, c1 := newTestSession(t)
	defer c1()
	s2, c2 := newTestSession(t)
	defer c2()

	if err := cm.Register("node-1", s1); err != nil {
		t.Fatalf("Register: %v", err)
	}
	err := cm.Register("node-1", s2)
	if !errors.Is(err, ErrClientExists) {
		t.Errorf("expected ErrClientExists, got %v", err)
	}
}

func TestClientManagerGet(t *testing.T) {
	cm := NewClientManager()
	sess, cleanup := newTestSession(t)
	defer cleanup()

	cm.Register("node-1", sess)

	got, err := cm.Get("node-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != sess {
		t.Error("Get returned different session")
	}
}

func TestClientManagerGetNotFound(t *testing.T) {
	cm := NewClientManager()
	_, err := cm.Get("missing")
	if !errors.Is(err, ErrClientNotFound) {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}
}

func TestClientManagerUnregister(t *testing.T) {
	cm := NewClientManager()
	sess, cleanup := newTestSession(t)
	defer cleanup()

	cm.Register("node-1", sess)
	cm.Unregister("node-1")

	if cm.Count() != 0 {
		t.Errorf("Count = %d, want 0", cm.Count())
	}
}

func TestClientManagerEach(t *testing.T) {
	cm := NewClientManager()
	s1, c1 := newTestSession(t)
	defer c1()
	s2, c2 := newTestSession(t)
	defer c2()

	cm.Register("a", s1)
	cm.Register("b", s2)

	seen := make(map[string]bool)
	cm.Each(func(id string, sess *Session) {
		seen[id] = true
	})
	if !seen["a"] || !seen["b"] {
		t.Errorf("Each missed clients: %v", seen)
	}
}

func TestClientManagerConcurrent(t *testing.T) {
	cm := NewClientManager()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sess, cleanup := newTestSession(t)
			defer cleanup()
			cm.Register(string(rune('A'+id%26))+string(rune('0'+id/26)), sess)
		}(i)
	}
	wg.Wait()

	if cm.Count() != 100 {
		t.Errorf("Count = %d, want 100", cm.Count())
	}
}
