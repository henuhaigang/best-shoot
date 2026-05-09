package client

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"tunnel-project/logger"
	"tunnel-project/protocol"
)

// ConnManager tracks local connections by tunnelID+connID. Thread-safe.
type ConnManager struct {
	mu    sync.RWMutex
	conns map[string]net.Conn
}

func newConnManager() *ConnManager {
	return &ConnManager{conns: make(map[string]net.Conn)}
}

func connKey(tunnelID uint32, connID uint32) string {
	return fmt.Sprintf("%d:%d", tunnelID, connID)
}

func (cm *ConnManager) store(tunnelID, connID uint32, conn net.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.conns[connKey(tunnelID, connID)] = conn
}

func (cm *ConnManager) load(tunnelID, connID uint32) (net.Conn, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	c, ok := cm.conns[connKey(tunnelID, connID)]
	return c, ok
}

func (cm *ConnManager) delete(tunnelID, connID uint32) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.conns, connKey(tunnelID, connID))
}

// DefaultHeartbeatInterval is how often the client sends heartbeats.
const DefaultHeartbeatInterval = 30 * time.Second

// Reconnect configuration.
const (
	ReconnectBaseDelay = time.Second
	ReconnectMaxDelay  = 30 * time.Second
)

// Client connects to a tunnel server and forwards traffic to local services.
type Client struct {
	serverAddr string
	clientID   string
	session    *protocol.Session
	conns      *ConnManager
	connID     atomic.Uint32
	done       chan struct{}
	log        *logger.Logger
}

// New creates a client that will connect to serverAddr and register as clientID.
func New(serverAddr, clientID string) *Client {
	return &Client{
		serverAddr: serverAddr,
		clientID:   clientID,
		conns:      newConnManager(),
		done:       make(chan struct{}),
		log:        logger.Default("client"),
	}
}

// SetLogger sets the logger for the client.
func (c *Client) SetLogger(l *logger.Logger) {
	c.log = l
}

// Connect opens the TCP connection and registers with the server.
func (c *Client) Connect() error {
	if err := c.connect(); err != nil {
		return err
	}
	go c.heartbeatLoop()
	return nil
}

// connect dials and registers. Does not start heartbeat (caller's responsibility).
func (c *Client) connect() error {
	conn, err := net.Dial("tcp", c.serverAddr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", c.serverAddr, err)
	}
	c.session = protocol.NewSession(conn)

	if err := c.register(); err != nil {
		conn.Close()
		return err
	}

	c.log.Info("registered", logger.F("server", c.serverAddr))
	return nil
}

// register sends MsgRegister and waits for MsgRegisterAck.
func (c *Client) register() error {
	err := c.session.Send(&protocol.Message{
		Type:    protocol.MsgRegister,
		Payload: []byte(c.clientID),
	})
	if err != nil {
		return fmt.Errorf("send register: %w", err)
	}

	msg, err := c.session.Receive()
	if err != nil {
		return fmt.Errorf("receive register ack: %w", err)
	}
	if msg.Type != protocol.MsgRegisterAck {
		return fmt.Errorf("expected RegisterAck, got %s", msg.TypeName())
	}
	return nil
}

// Serve reads messages from the server and dispatches them. Blocks until error.
func (c *Client) Serve() error {
	for {
		msg, err := c.session.Receive()
		if err != nil {
			return err
		}
		if err := c.handleMessage(msg); err != nil {
			c.log.Warn("handle error", logger.F("error", err))
		}
	}
}

// Run connects and serves with automatic reconnection. Blocks until Close is called.
func (c *Client) Run() error {
	delay := ReconnectBaseDelay
	for {
		select {
		case <-c.done:
			return nil
		default:
		}

		if err := c.connect(); err != nil {
			c.log.Warn("connect failed, retrying", logger.F("error", err), logger.F("delay", delay))
			c.waitForDelay(delay)
			delay = nextDelay(delay)
			continue
		}

		delay = ReconnectBaseDelay // reset on success
		err := c.Serve()
		c.log.Info("disconnected", logger.F("error", err))
		c.closeConns()

		select {
		case <-c.done:
			return nil
		default:
		}

		c.log.Info("reconnecting", logger.F("delay", delay))
		c.waitForDelay(delay)
		delay = nextDelay(delay)
	}
}

// handleMessage dispatches a single message.
func (c *Client) handleMessage(msg *protocol.Message) error {
	switch msg.Type {
	case protocol.MsgOpenTunnel:
		return c.handleOpenTunnel(msg)
	case protocol.MsgData:
		return c.handleData(msg)
	case protocol.MsgCloseTunnel:
		return c.handleCloseTunnel(msg)
	case protocol.MsgHeartbeat:
		return c.session.Send(&protocol.Message{Type: protocol.MsgHeartbeat})
	default:
		return fmt.Errorf("unexpected message: %s", msg.TypeName())
	}
}

// handleOpenTunnel connects to the local port and acks the server.
func (c *Client) handleOpenTunnel(msg *protocol.Message) error {
	if len(msg.Payload) < 2 {
		return fmt.Errorf("OpenTunnel payload too short")
	}
	port := binary.BigEndian.Uint16(msg.Payload[:2])

	localAddr := fmt.Sprintf("127.0.0.1:%d", port)
	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		c.log.Warn("local dial failed", logger.F("addr", localAddr), logger.F("error", err))
		// Send ack with empty payload to indicate failure
		return c.session.Send(&protocol.Message{
			Type:     protocol.MsgOpenTunnelAck,
			TunnelID: msg.TunnelID,
		})
	}

	c.log.Info("tunnel opened", logger.F("tunnelID", msg.TunnelID), logger.F("local", localAddr))

	// Assign connID and store before acking, so handleData can find it immediately
	connID := c.connID.Add(1)
	c.conns.store(msg.TunnelID, connID, localConn)

	// Ack success
	if err := c.session.Send(&protocol.Message{
		Type:     protocol.MsgOpenTunnelAck,
		TunnelID: msg.TunnelID,
	}); err != nil {
		localConn.Close()
		c.conns.delete(msg.TunnelID, connID)
		return err
	}

	// Start reading from local connection and sending data back
	go c.readLocal(msg.TunnelID, connID, localConn)
	return nil
}

// readLocal reads from a local connection and sends MsgData to the server.
func (c *Client) readLocal(tunnelID, connID uint32, localConn net.Conn) {
	defer func() {
		c.conns.delete(tunnelID, connID)
		localConn.Close()
		c.session.Send(&protocol.Message{
			Type:     protocol.MsgCloseTunnel,
			TunnelID: tunnelID,
			Payload:  connIDBytes(connID),
		})
	}()

	buf := make([]byte, 32*1024)
	for {
		n, err := localConn.Read(buf)
		if n > 0 {
			payload := make([]byte, 4+n)
			copy(payload, connIDBytes(connID))
			copy(payload[4:], buf[:n])

			if sendErr := c.session.Send(&protocol.Message{
				Type:     protocol.MsgData,
				TunnelID: tunnelID,
				Payload:  payload,
			}); sendErr != nil {
				c.log.Error("send error", logger.F("error", sendErr))
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				c.log.Debug("local read error", logger.F("error", err))
			}
			return
		}
	}
}

// handleData delivers data from the server to a local connection.
func (c *Client) handleData(msg *protocol.Message) error {
	if len(msg.Payload) < 4 {
		return fmt.Errorf("data payload too short")
	}
	connID := extractConnID(msg.Payload)
	conn, ok := c.conns.load(msg.TunnelID, connID)
	if !ok {
		return nil // connection already closed
	}
	_, err := conn.Write(msg.Payload[4:])
	return err
}

// handleCloseTunnel closes a local connection.
func (c *Client) handleCloseTunnel(msg *protocol.Message) error {
	if len(msg.Payload) < 4 {
		return nil
	}
	connID := extractConnID(msg.Payload)
	if conn, ok := c.conns.load(msg.TunnelID, connID); ok {
		conn.Close()
		c.conns.delete(msg.TunnelID, connID)
	}
	return nil
}

// heartbeatLoop sends periodic heartbeats to the server.
func (c *Client) heartbeatLoop() {
	ticker := time.NewTicker(DefaultHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			if err := c.session.Send(&protocol.Message{Type: protocol.MsgHeartbeat}); err != nil {
				c.log.Warn("heartbeat send error", logger.F("error", err))
				return
			}
		}
	}
}

// closeConns closes all local connections. Called on disconnect.
func (c *Client) closeConns() {
	c.conns.mu.Lock()
	defer c.conns.mu.Unlock()
	for key, conn := range c.conns.conns {
		conn.Close()
		delete(c.conns.conns, key)
	}
}

// waitForDelay sleeps for d or returns early if Close is called.
func (c *Client) waitForDelay(d time.Duration) {
	select {
	case <-c.done:
	case <-time.After(d):
	}
}

// nextDelay doubles the delay up to the max.
func nextDelay(d time.Duration) time.Duration {
	d *= 2
	if d > ReconnectMaxDelay {
		d = ReconnectMaxDelay
	}
	return d
}

// Close shuts down the client session.
func (c *Client) Close() error {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	if c.session != nil {
		return c.session.Close()
	}
	return nil
}

func extractConnID(payload []byte) uint32 {
	return uint32(payload[0])<<24 | uint32(payload[1])<<16 | uint32(payload[2])<<8 | uint32(payload[3])
}

func connIDBytes(id uint32) []byte {
	return []byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)}
}
