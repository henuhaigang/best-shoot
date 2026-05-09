package server

import (
	"errors"
	"fmt"
	"sync"

	"tunnel-project/tunnel"
)

var (
	ErrTunnelExists   = errors.New("tunnel already exists")
	ErrTunnelNotFound = errors.New("tunnel not found")
)

// TunnelManager tracks active tunnels. Thread-safe.
type TunnelManager struct {
	mu      sync.RWMutex
	tunnels map[uint32]*tunnel.Tunnel
	counter uint32
}

// NewTunnelManager returns an empty tunnel manager.
func NewTunnelManager() *TunnelManager {
	return &TunnelManager{
		tunnels: make(map[uint32]*tunnel.Tunnel),
	}
}

// Open creates a new tunnel on the given port and returns it.
// The tunnel ID is auto-assigned. Returns ErrTunnelExists if the port is already bound.
func (tm *TunnelManager) Open(clientID string, port uint16, sess tunnel.SessionSender) (*tunnel.Tunnel, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Only check for duplicate when a specific port is requested
	if port != 0 {
		for _, t := range tm.tunnels {
			if t.Port() == port {
				return nil, fmt.Errorf("%w: port %d", ErrTunnelExists, port)
			}
		}
	}

	tm.counter++
	tun, err := tunnel.New(tm.counter, port, sess)
	if err != nil {
		return nil, err
	}

	tm.tunnels[tun.ID()] = tun
	return tun, nil
}

// Close removes a tunnel and closes it. No-op if not found.
func (tm *TunnelManager) Close(id uint32) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tun, ok := tm.tunnels[id]; ok {
		tun.Close()
		delete(tm.tunnels, id)
	}
}

// Remove removes a tunnel by ID without closing it (caller is responsible for closing).
func (tm *TunnelManager) Remove(id uint32) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.tunnels, id)
}

// Get returns a tunnel by ID.
func (tm *TunnelManager) Get(id uint32) (*tunnel.Tunnel, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tun, ok := tm.tunnels[id]
	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrTunnelNotFound, id)
	}
	return tun, nil
}

// Count returns the number of active tunnels.
func (tm *TunnelManager) Count() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.tunnels)
}

// CloseAll closes all tunnels.
func (tm *TunnelManager) CloseAll() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for _, tun := range tm.tunnels {
		tun.Close()
	}
	tm.tunnels = make(map[uint32]*tunnel.Tunnel)
}

// Each iterates over all tunnels. The callback must not call tm methods.
func (tm *TunnelManager) Each(fn func(tun *tunnel.Tunnel)) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	for _, tun := range tm.tunnels {
		fn(tun)
	}
}
