package tunnel

import (
	"io"
	"net"
	"sync"
	"sync/atomic"

	"tunnel-project/logger"
	"tunnel-project/protocol"
)

// ProxyListener accepts external connections on a tunnel port and bridges
// them to the client session via protocol messages.
type ProxyListener struct {
	listener net.Listener
	tunnelID uint32
	session  SessionSender
	conns    sync.Map // connID -> net.Conn
	counter  atomic.Uint64
	done     chan struct{}
	log      *logger.Logger
}

// SessionSender is the subset of protocol.Session needed to send messages.
type SessionSender interface {
	Send(msg *protocol.Message) error
}

// NewProxyListener creates a listener that accepts on ln and bridges to sess.
func NewProxyListener(ln net.Listener, tunnelID uint32, sess SessionSender) *ProxyListener {
	return &ProxyListener{
		listener: ln,
		tunnelID: tunnelID,
		session:  sess,
		done:     make(chan struct{}),
		log:      logger.Default("proxy"),
	}
}

// SetLogger sets the logger for the proxy listener.
func (p *ProxyListener) SetLogger(l *logger.Logger) {
	p.log = l
}

// Serve accepts connections in a loop. Blocks until Close is called.
func (p *ProxyListener) Serve() error {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			select {
			case <-p.done:
				return nil
			default:
				return err
			}
		}
		go p.handleConn(conn)
	}
}

// handleConn bridges an external connection to the tunnel.
func (p *ProxyListener) handleConn(conn net.Conn) {
	connID := uint32(p.counter.Add(1))
	p.conns.Store(connID, conn)
	defer func() {
		p.conns.Delete(connID)
		conn.Close()
		// Notify client this connection is closed
		p.session.Send(&protocol.Message{
			Type:     protocol.MsgCloseTunnel,
			TunnelID: p.tunnelID,
			Payload:  connIDBytes(connID),
		})
	}()

	p.log.Debug("accepted conn", logger.F("tunnelID", p.tunnelID), logger.F("connID", connID), logger.F("remote", conn.RemoteAddr()))

	// Read from external connection, send as MsgData to client
	buf := make([]byte, 32*1024)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			payload := make([]byte, 4+n)
			copy(payload, connIDBytes(connID))
			copy(payload[4:], buf[:n])

			if sendErr := p.session.Send(&protocol.Message{
				Type:     protocol.MsgData,
				TunnelID: p.tunnelID,
				Payload:  payload,
			}); sendErr != nil {
				p.log.Error("send error", logger.F("tunnelID", p.tunnelID), logger.F("error", sendErr))
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				p.log.Debug("conn read error", logger.F("tunnelID", p.tunnelID), logger.F("connID", connID), logger.F("error", err))
			}
			return
		}
	}
}

// Deliver writes data from the client to the external connection.
func (p *ProxyListener) Deliver(connID uint32, data []byte) error {
	val, ok := p.conns.Load(connID)
	if !ok {
		return nil // connection already closed
	}
	conn := val.(net.Conn)
	_, err := conn.Write(data)
	return err
}

// CloseConn closes a specific external connection.
func (p *ProxyListener) CloseConn(connID uint32) {
	if val, ok := p.conns.LoadAndDelete(connID); ok {
		val.(net.Conn).Close()
	}
}

// Close stops the listener and all active connections.
func (p *ProxyListener) Close() error {
	select {
	case <-p.done:
		return nil
	default:
		close(p.done)
	}
	err := p.listener.Close()
	p.conns.Range(func(key, val any) bool {
		val.(net.Conn).Close()
		return true
	})
	return err
}

func connIDBytes(id uint32) []byte {
	return []byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)}
}
