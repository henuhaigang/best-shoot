package protocol

import (
	"bufio"
	"io"
	"sync"
)

// Encoder writes messages to an underlying writer with buffering.
type Encoder struct {
	mu  sync.Mutex
	w   *bufio.Writer
	out io.Writer
}

// NewEncoder returns an encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:   bufio.NewWriterSize(w, 64*1024),
		out: w,
	}
}

// Send writes a message and flushes the buffer.
func (e *Encoder) Send(msg *Message) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := msg.WriteTo(e.w); err != nil {
		return err
	}
	return e.w.Flush()
}
