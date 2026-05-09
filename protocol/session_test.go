package protocol

import (
	"io"
	"net"
	"sync"
	"testing"
)

func TestSessionSendReceive(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	server := NewSession(srv)
	client := NewSession(cli)

	go func() {
		msg := &Message{Type: MsgRegister, TunnelID: 0, Payload: []byte("token")}
		if err := client.Send(msg); err != nil {
			t.Errorf("Send: %v", err)
		}
	}()

	got, err := server.Receive()
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if got.Type != MsgRegister {
		t.Errorf("Type = %d, want %d", got.Type, MsgRegister)
	}
	if string(got.Payload) != "token" {
		t.Errorf("Payload = %q, want %q", got.Payload, "token")
	}
}

func TestSessionSendAndReceive(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	server := NewSession(srv)
	client := NewSession(cli)

	go func() {
		msg, err := server.Receive()
		if err != nil {
			t.Errorf("Receive: %v", err)
			return
		}
		reply := &Message{Type: MsgRegisterAck, TunnelID: msg.TunnelID}
		if err := server.Send(reply); err != nil {
			t.Errorf("Send: %v", err)
		}
	}()

	got, err := client.SendAndReceive(&Message{Type: MsgRegister, Payload: []byte("tok")})
	if err != nil {
		t.Fatalf("SendAndReceive: %v", err)
	}
	if got.Type != MsgRegisterAck {
		t.Errorf("Type = %d, want %d", got.Type, MsgRegisterAck)
	}
}

func TestSessionReadLoop(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	server := NewSession(srv)
	client := NewSession(cli)

	msgs := []*Message{
		{Type: MsgData, TunnelID: 1, Payload: []byte("a")},
		{Type: MsgData, TunnelID: 2, Payload: []byte("b")},
		{Type: MsgCloseTunnel, TunnelID: 1},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, m := range msgs {
			if err := client.Send(m); err != nil {
				t.Errorf("Send: %v", err)
			}
		}
		client.Close()
	}()

	var received []*Message
	err := server.ReadLoop(func(msg *Message) error {
		received = append(received, msg)
		return nil
	})
	if err != io.EOF {
		t.Fatalf("ReadLoop: %v, want io.EOF", err)
	}
	wg.Wait()

	if len(received) != len(msgs) {
		t.Fatalf("received %d messages, want %d", len(received), len(msgs))
	}
	for i, got := range received {
		if got.Type != msgs[i].Type || got.TunnelID != msgs[i].TunnelID {
			t.Errorf("msg[%d] = %s, want %s", i, got, msgs[i])
		}
	}
}

func TestSessionRemoteAddr(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	s := NewSession(cli)
	if s.RemoteAddr() == nil {
		t.Error("RemoteAddr() returned nil")
	}
	if s.Conn() != cli {
		t.Error("Conn() did not return the underlying connection")
	}
}
