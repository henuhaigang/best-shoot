package protocol

import (
	"bytes"
	"net"
	"sync"
	"testing"
)

func TestEncoderDecoder(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	dec := NewDecoder(&buf)

	msgs := []*Message{
		{Type: MsgRegister, TunnelID: 0, Payload: []byte("token")},
		{Type: MsgData, TunnelID: 5, Payload: []byte("hello")},
		{Type: MsgHeartbeat},
	}

	for _, want := range msgs {
		if err := enc.Send(want); err != nil {
			t.Fatalf("Send: %v", err)
		}
		got, err := dec.Receive()
		if err != nil {
			t.Fatalf("Receive: %v", err)
		}
		if got.Type != want.Type || got.TunnelID != want.TunnelID || !bytes.Equal(got.Payload, want.Payload) {
			t.Errorf("mismatch: got %s, want %s", got, want)
		}
	}
}

func TestEncoderConcurrentSend(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id uint32) {
			defer wg.Done()
			msg := &Message{Type: MsgData, TunnelID: id, Payload: []byte("x")}
			if err := enc.Send(msg); err != nil {
				t.Errorf("Send: %v", err)
			}
		}(uint32(i))
	}
	wg.Wait()

	// Verify we can decode all 100 messages
	dec := NewDecoder(&buf)
	count := 0
	for {
		_, err := dec.Receive()
		if err != nil {
			break
		}
		count++
	}
	if count != 100 {
		t.Errorf("decoded %d messages, want 100", count)
	}
}

func TestEncoderDecoderOverConn(t *testing.T) {
	// Use a real pipe to simulate a connection
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	enc := NewEncoder(cli)
	dec := NewDecoder(srv)

	want := &Message{Type: MsgOpenTunnel, TunnelID: 42, Payload: []byte{0x1F, 0x90}}

	go func() {
		if err := enc.Send(want); err != nil {
			t.Errorf("Send: %v", err)
		}
	}()

	got, err := dec.Receive()
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if got.Type != want.Type || got.TunnelID != want.TunnelID {
		t.Errorf("mismatch: got %s, want %s", got, want)
	}
}
