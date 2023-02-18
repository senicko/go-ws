package ws

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
)

const (
	// Frame byte 0 bits
	bitFin  = 1 << 7
	bitRsv1 = 1 << 6
	bitRsv2 = 1 << 5
	bitRsv3 = 1 << 4

	// Frame byte 1 bits
	bitMask = 1 << 7
)

// WebSocket frame opcodes
const (
	OpText   = 1
	OpBinary = 2
	OpClose  = 8
	OpPing   = 9
	OpPong   = 10
)

// WebSocket connection close status codes.
// https://www.rfc-editor.org/rfc/rfc6455#section-11.7
const (
	CloseStatusNoStatusReceived uint16 = 1005
	CloseStatusProtocolError    uint16 = 1002
)

var (
	ErrProtocolError    = errors.New("protocol error")
	ErrConnectionClosed = errors.New("connection has been closed")
)

// applyMask is a util for masking the payload.
// The algorithm for masking and unmasking is the same
// as mentioned in https://www.rfc-editor.org/rfc/rfc6455#section-5.3
func applyMask(b, maskingKey []byte) []byte {
	var transformed []byte

	for i, b := range b {
		transformed = append(transformed, b^maskingKey[i%4])
	}

	return transformed
}

// frame represents decoded WebSocket frame.
// https://www.rfc-editor.org/rfc/rfc6455#section-5.2
type frame struct {
	fin           bool
	rsv1          bool
	rsv2          bool
	rsv3          bool
	opcode        uint8
	mask          bool
	payloadLength uint64
	payload       []byte
}

// validate returns error when frame structure does not fit it's type.
// The returned error wraps ErrProtocolError.
func (f *frame) validate() error {
	var errors []string

	switch f.opcode {
	case OpClose, OpPing, OpPong:
		if f.payloadLength > 125 {
			errors = append(errors, "control frame payload length must be <= 125")
		}

		if !f.fin {
			errors = append(errors, "control frame FIN not set to 1")
		}
	case OpText, OpBinary:
		// TODO: Validate for final read when fragmentation will be supported
	default:
		errors = append(errors, "unknown opcode")
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s: %w", strings.Join(errors, ","), ErrProtocolError)
	}

	return nil
}

// Conn represents a WebSocket connection.
type Conn struct {
	conn        net.Conn
	compression bool
	writeBuf    []byte
	reader      *bufio.Reader
}

// NewConn returns a new Conn.
func newConn(conn net.Conn, readBufferSize int, writeBufferSize int) *Conn {
	return &Conn{
		conn:     conn,
		reader:   bufio.NewReaderSize(conn, readBufferSize),
		writeBuf: make([]byte, writeBufferSize),
	}
}

// Close closes the WebSocket's TCP connection.
func (c *Conn) Close() {
	c.conn.Close()
}

// ReadMessage returns payload from inoming WebSocket frame.
func (c *Conn) ReadMessage() ([]byte, error) {
	compressed := false
	buf := []byte{}

	for {
		f, err := c.nextFrame()
		if err != nil {
			return nil, fmt.Errorf("failed to read frame: %w", err)
		}
		if f == nil {
			continue
		}

		if f.rsv1 {
			if !compressed {
				compressed = true
			} else {
				// TODO: What should happen when RSV1 is repeated?
				return nil, err
			}
		}

		if !f.fin {
			buf = append(buf, f.payload...)
			continue
		}

		buf = append(buf, f.payload...)
		break
	}

	if !compressed {
		return buf, nil
	}

	buf, err := decompress(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress the message: %w", err)
	}

	return buf, nil
}

// WriteMessage sends message to the client.
func (c *Conn) WriteMessage(opcode uint8, m []byte) error {
	frame := make([]byte, 2)
	frame[0] |= opcode
	frame[0] |= bitFin

	if c.compression {
		frame[0] |= bitRsv1

		cm, err := compress(m)
		if err != nil {
			return fmt.Errorf("failed to compress: %w", err)
		}
		m = cm
	}

	payloadLen := len(m)

	if payloadLen < 126 {
		frame[1] |= byte(payloadLen)
	} else if payloadLen <= 0xFFFF {
		frame[1] |= 126
		binary.BigEndian.PutUint16(frame, uint16(payloadLen))
	} else {
		frame[1] |= 127
		binary.BigEndian.PutUint64(frame, uint64(payloadLen))
	}

	// TODO: We probably don't want to do that. Payload can be really huge.
	// In fact do we want it to be a byte slice instead og io.Reader or something?
	frame = append(frame, m...)

	if _, err := c.conn.Write(frame); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}

	return nil
}

// advanceBytes returns n next bytes from client's message.
func (c *Conn) advanceBytes(n uint64) ([]byte, error) {
	b := make([]byte, n)

	if _, err := io.ReadFull(c.reader, b); err != nil {
		return nil, fmt.Errorf("failed to read full: %w", err)
	}

	return b, nil
}

// nextFrame returns next message frames (OpText, OpBinary).
// It intercepts control frames, which are processed seperately
// and returns nil.
func (c *Conn) nextFrame() (*frame, error) {
	b, err := c.advanceBytes(2)
	if err != nil {
		return nil, err
	}

	fin := b[0]&bitFin != 0
	rsv1 := b[0]&bitRsv1 != 0
	rsv2 := b[0]&bitRsv2 != 0
	rsv3 := b[0]&bitRsv3 != 0
	opcode := uint8(b[0] & 0xf)
	mask := b[1]&bitMask != 0

	if !mask {
		return nil, fmt.Errorf("frame not masked: %w", ErrProtocolError)
	}

	payloadLength := uint64(b[1] & 0x7f)

	if payloadLength == 126 {
		b, err := c.advanceBytes(2)
		if err != nil {
			return nil, err
		}

		payloadLength = uint64(binary.BigEndian.Uint16(b))
	} else if payloadLength == 127 {
		b, err := c.advanceBytes(8)
		if err != nil {
			return nil, err
		}

		payloadLength = binary.BigEndian.Uint64(b)
	}

	maskingKey, err := c.advanceBytes(4)
	if err != nil {
		return nil, err
	}

	maskedPayload, err := c.advanceBytes(payloadLength)
	if err != nil {
		return nil, err
	}

	payload := applyMask(maskedPayload, maskingKey)

	fmt.Println(payload)

	f := &frame{
		fin:           fin,
		rsv1:          rsv1,
		rsv2:          rsv2,
		rsv3:          rsv3,
		opcode:        opcode,
		mask:          mask,
		payloadLength: payloadLength,
		payload:       payload,
	}

	// Validate captured frame
	if f.validate() != nil {
		return nil, fmt.Errorf("frame validation failed: %w", err)
	}

	// If frame is a control frame process it separately
	if opcode != OpText && opcode != OpBinary {
		switch opcode {
		case OpPing:
			if err := c.WriteMessage(OpPong, payload); err != nil {
				return nil, err
			}
			return nil, nil
		case OpClose:
			statusCode := CloseStatusNoStatusReceived
			reason := ""

			if f.payloadLength > 0 {
				statusCode = binary.BigEndian.Uint16(f.payload)
				reason = string(f.payload[2:])
			}

			if err := c.handleClose(statusCode, reason); err != nil {
				return nil, err
			}

			return nil, ErrConnectionClosed
		}
		return nil, nil
	}

	return f, nil
}

// handleClose sends the close frame to the client in order to finish
// the close procedure.
func (c *Conn) handleClose(statusCode uint16, reason string) error {
	var buf []byte

	if statusCode != CloseStatusNoStatusReceived {
		buf = binary.BigEndian.AppendUint16(buf, statusCode)
	}

	if err := c.WriteMessage(OpClose, buf); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}
