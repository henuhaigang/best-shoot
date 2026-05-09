package tunnel

import (
	"fmt"
	"net"

	"tunnel-project/logger"
	"tunnel-project/protocol"
)

// Tunnel binds a listener to a tunnel ID and dispatches messages from the client.
type Tunnel struct {
	id       uint32
	port     uint16
	proxy    *ProxyListener
	session  SessionSender
	listener net.Listener
	log      *logger.Logger
}

// New creates a Tunnel that listens on port and bridges to sess.
func New(id uint32, port uint16, sess SessionSender) (*Tunnel, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("listen :%d: %w", port, err)
	}

	proxy := NewProxyListener(ln, id, sess)

	t := &Tunnel{
		id:       id,
		port:     port,
		proxy:    proxy,
		session:  sess,
		listener: ln,
		log:      logger.Default("tunnel"),
	}
	return t, nil
}

// SetLogger sets the logger for the tunnel.
func (t *Tunnel) SetLogger(l *logger.Logger) {
	t.log = l
}

// ID returns the tunnel ID.
func (t *Tunnel) ID() uint32 { return t.id }

// Port returns the listening port.
func (t *Tunnel) Port() uint16 { return t.port }

// Addr returns the listener address.
func (t *Tunnel) Addr() net.Addr { return t.listener.Addr() }

// Serve starts accepting external connections. Blocks until Close is called.
func (t *Tunnel) Serve() error {
	t.log.Info("serving", logger.F("tunnelID", t.id), logger.F("addr", t.listener.Addr()))
	return t.proxy.Serve()
}

// HandleMessage dispatches an incoming protocol message to the proxy listener.
func (t *Tunnel) HandleMessage(msg *protocol.Message) error {
	switch msg.Type {
	case protocol.MsgData:
		if len(msg.Payload) < 4 {
			return fmt.Errorf("tunnel %d: data payload too short (%d bytes)", t.id, len(msg.Payload))
		}
		connID := uint32(msg.Payload[0])<<24 | uint32(msg.Payload[1])<<16 | uint32(msg.Payload[2])<<8 | uint32(msg.Payload[3])
		return t.proxy.Deliver(connID, msg.Payload[4:])

	case protocol.MsgCloseTunnel:
		if len(msg.Payload) >= 4 {
			connID := uint32(msg.Payload[0])<<24 | uint32(msg.Payload[1])<<16 | uint32(msg.Payload[2])<<8 | uint32(msg.Payload[3])
			t.proxy.CloseConn(connID)
		}

	default:
		t.log.Warn("unexpected message type", logger.F("tunnelID", t.id), logger.F("type", msg.TypeName()))
	}
	return nil
}

// Close shuts down the tunnel and its listener.
func (t *Tunnel) Close() error {
	t.log.Info("closing", logger.F("tunnelID", t.id))
	return t.proxy.Close()
}
