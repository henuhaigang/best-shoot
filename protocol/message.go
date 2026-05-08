package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Message types
const (
	MsgRegister      uint8 = 0x01 // Client -> Server: register
	MsgRegisterAck   uint8 = 0x02 // Server -> Client: registration ack
	MsgOpenTunnel    uint8 = 0x03 // Server -> Client: open a tunnel
	MsgOpenTunnelAck uint8 = 0x04 // Client -> Server: tunnel opened
	MsgData          uint8 = 0x05 // Bidirectional: tunnel data
	MsgCloseTunnel   uint8 = 0x06 // Bidirectional: close tunnel
	MsgHeartbeat     uint8 = 0x07 // Bidirectional: keep-alive
)

// Header is the fixed-size part of every message.
// Wire format: [Type:1][TunnelID:4][Length:4] = 9 bytes
const HeaderSize = 9

var (
	ErrMessageTooLarge = errors.New("message payload exceeds max size")
	ErrShortHeader     = errors.New("short header read")
	ErrUnknownMsgType  = errors.New("unknown message type")
)

// MaxPayloadSize caps individual message payloads at 1 MB.
const MaxPayloadSize = 1 << 20

// Message is a protocol frame.
type Message struct {
	Type     uint8
	TunnelID uint32
	Payload  []byte
}

// TypeName returns a human-readable name for the message type.
func (m *Message) TypeName() string {
	switch m.Type {
	case MsgRegister:
		return "Register"
	case MsgRegisterAck:
		return "RegisterAck"
	case MsgOpenTunnel:
		return "OpenTunnel"
	case MsgOpenTunnelAck:
		return "OpenTunnelAck"
	case MsgData:
		return "Data"
	case MsgCloseTunnel:
		return "CloseTunnel"
	case MsgHeartbeat:
		return "Heartbeat"
	default:
		return fmt.Sprintf("Unknown(0x%02x)", m.Type)
	}
}

// String implements fmt.Stringer for debug logging.
func (m *Message) String() string {
	return fmt.Sprintf("{%s tunnel=%d len=%d}", m.TypeName(), m.TunnelID, len(m.Payload))
}

// WriteTo writes the message to w in wire format.
func (m *Message) WriteTo(w io.Writer) error {
	if len(m.Payload) > MaxPayloadSize {
		return ErrMessageTooLarge
	}

	var buf [HeaderSize]byte
	buf[0] = m.Type
	binary.BigEndian.PutUint32(buf[1:5], m.TunnelID)
	binary.BigEndian.PutUint32(buf[5:9], uint32(len(m.Payload)))

	if _, err := w.Write(buf[:]); err != nil {
		return err
	}
	if len(m.Payload) > 0 {
		if _, err := w.Write(m.Payload); err != nil {
			return err
		}
	}
	return nil
}

// ReadFrom reads exactly one message from r.
func ReadFrom(r io.Reader) (*Message, error) {
	var hdr [HeaderSize]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}

	msg := &Message{
		Type:     hdr[0],
		TunnelID: binary.BigEndian.Uint32(hdr[1:5]),
	}
	length := binary.BigEndian.Uint32(hdr[5:9])

	if length > MaxPayloadSize {
		return nil, ErrMessageTooLarge
	}

	if length > 0 {
		msg.Payload = make([]byte, length)
		if _, err := io.ReadFull(r, msg.Payload); err != nil {
			return nil, err
		}
	}
	return msg, nil
}
