package protocol

import (
	"bytes"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		msg  Message
	}{
		{"register", Message{Type: MsgRegister, TunnelID: 0, Payload: []byte("token123")}},
		{"register_ack", Message{Type: MsgRegisterAck, TunnelID: 0, Payload: nil}},
		{"open_tunnel", Message{Type: MsgOpenTunnel, TunnelID: 42, Payload: []byte{0x1F, 0x90}},                }, // port 8080
		{"open_tunnel_ack", Message{Type: MsgOpenTunnelAck, TunnelID: 42, Payload: nil}},
		{"data", Message{Type: MsgData, TunnelID: 7, Payload: []byte("hello world")}},
		{"close_tunnel", Message{Type: MsgCloseTunnel, TunnelID: 7, Payload: nil}},
		{"heartbeat", Message{Type: MsgHeartbeat, TunnelID: 0, Payload: nil}},
		{"empty_payload", Message{Type: MsgData, TunnelID: 1, Payload: []byte{}}},
		{"large_payload", Message{Type: MsgData, TunnelID: 99, Payload: bytes.Repeat([]byte("x"), 65536)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tt.msg.WriteTo(&buf); err != nil {
				t.Fatalf("WriteTo: %v", err)
			}

			got, err := ReadFrom(&buf)
			if err != nil {
				t.Fatalf("ReadFrom: %v", err)
			}

			if got.Type != tt.msg.Type {
				t.Errorf("Type = %d, want %d", got.Type, tt.msg.Type)
			}
			if got.TunnelID != tt.msg.TunnelID {
				t.Errorf("TunnelID = %d, want %d", got.TunnelID, tt.msg.TunnelID)
			}
			if !bytes.Equal(got.Payload, tt.msg.Payload) {
				t.Errorf("Payload mismatch: got %d bytes, want %d bytes", len(got.Payload), len(tt.msg.Payload))
			}
		})
	}
}

func TestTypeName(t *testing.T) {
	msg := &Message{Type: MsgData}
	if msg.TypeName() != "Data" {
		t.Errorf("TypeName = %q, want %q", msg.TypeName(), "Data")
	}

	msg.Type = 0xFF
	if msg.TypeName() != "Unknown(0xff)" {
		t.Errorf("TypeName = %q, want %q", msg.TypeName(), "Unknown(0xff)")
	}
}

func TestString(t *testing.T) {
	msg := &Message{Type: MsgHeartbeat, TunnelID: 5}
	s := msg.String()
	if s == "" {
		t.Error("String() returned empty")
	}
}

func TestPayloadTooLarge(t *testing.T) {
	msg := Message{Type: MsgData, TunnelID: 1, Payload: make([]byte, MaxPayloadSize+1)}
	var buf bytes.Buffer
	if err := msg.WriteTo(&buf); err != ErrMessageTooLarge {
		t.Errorf("expected ErrMessageTooLarge, got %v", err)
	}
}

func TestReadFromPayloadTooLarge(t *testing.T) {
	// Craft a header claiming a payload larger than MaxPayloadSize
	var buf bytes.Buffer
	buf.WriteByte(MsgData)
	var tb [4]byte
	tb = [4]byte{} // tunnelID = 0
	buf.Write(tb[:])
	// length = MaxPayloadSize + 1
	var lb [4]byte
	lb[0], lb[1], lb[2], lb[3] = 0x00, 0x10, 0x00, 0x01 // 1048577
	buf.Write(lb[:])

	_, err := ReadFrom(&buf)
	if err != ErrMessageTooLarge {
		t.Errorf("expected ErrMessageTooLarge, got %v", err)
	}
}
