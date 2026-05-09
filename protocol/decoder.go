package protocol

import (
	"bufio"
	"io"
)

// Decoder reads messages from an underlying reader with buffering.
type Decoder struct {
	r *bufio.Reader
}

// NewDecoder returns a decoder that reads from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReaderSize(r, 64*1024)}
}

// Receive reads the next message. Blocks until one is available.
func (d *Decoder) Receive() (*Message, error) {
	return ReadFrom(d.r)
}
