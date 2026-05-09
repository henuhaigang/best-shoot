package server

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrClientExists   = errors.New("client already registered")
	ErrClientNotFound = errors.New("client not found")
)

// ClientManager tracks active client sessions. Thread-safe.
type ClientManager struct {
	mu      sync.RWMutex
	clients map[string]*Session
}

// NewClientManager returns an empty client manager.
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[string]*Session),
	}
}

// Register adds a session under the given clientID.
// Returns ErrClientExists if the ID is already in use.
func (cm *ClientManager) Register(clientID string, sess *Session) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.clients[clientID]; exists {
		return fmt.Errorf("%w: %s", ErrClientExists, clientID)
	}
	sess.SetClientID(clientID)
	cm.clients[clientID] = sess
	return nil
}

// Unregister removes a client by ID. No-op if not found.
func (cm *ClientManager) Unregister(clientID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.clients, clientID)
}

// Get returns the session for a client ID.
func (cm *ClientManager) Get(clientID string) (*Session, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	sess, ok := cm.clients[clientID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrClientNotFound, clientID)
	}
	return sess, nil
}

// Count returns the number of active clients.
func (cm *ClientManager) Count() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.clients)
}

// Each iterates over all sessions. The callback must not call cm methods (deadlock).
func (cm *ClientManager) Each(fn func(clientID string, sess *Session)) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for id, sess := range cm.clients {
		fn(id, sess)
	}
}
