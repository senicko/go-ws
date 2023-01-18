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
)

var (
	ErrProtocolError    = errors.New("invalid frame structure")
	ErrConnectionClosed = errors.New("connection has been closed")
)

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
	maskingKey    []byte
	payload       []byte
}

// unmaskPayload returns unmasked payload.
func (f *frame) unmaskPayload() []byte {
	var transformed []byte

	for i, b := range f.payload {
		transformed = append(transformed, b^f.maskingKey[i%4])
	}

	return transformed
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
	conn     net.Conn
	reader   *bufio.Reader
	writeBuf []byte
}

// newConn returns a new Conn.
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
	frame, err := c.decodeFrame()
	if err != nil {
		return nil, err
	}

	payload := frame.unmaskPayload()

	// Validate frame structure
	if err := frame.validate(); err != nil {
		return nil, fmt.Errorf("frame validation failed: %w", err)
	}

	fmt.Printf("(read)%+v\n", frame)

	switch frame.opcode {
	case OpText, OpBinary:
		return frame.unmaskPayload(), nil
	case OpClose:
		statusCode := CloseStatusNoStatusReceived
		reason := ""

		if frame.payloadLength > 0 {
			statusCode = binary.BigEndian.Uint16(payload)
			reason = string(payload[2:])
		}

		if err := c.handleClose(statusCode, reason); err != nil {
			return nil, err
		}

		return nil, ErrConnectionClosed
	}

	return nil, nil
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

// advanceBytes returns n next bytes from client's message.
func (c *Conn) advanceBytes(n uint64) ([]byte, error) {
	b := make([]byte, n)

	if _, err := io.ReadFull(c.reader, b); err != nil {
		return nil, err
	}

	return b, nil
}

// decodeFrame returns an incoming WebSocket frame.
func (c *Conn) decodeFrame() (*frame, error) {
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

	payloadLength, err := c.getPayloadLength(b[1])
	if err != nil {
		return nil, err
	}

	var maskingKey []byte
	if mask {
		maskingKey, err = c.advanceBytes(4)
		if err != nil {
			return nil, err
		}
	}

	payload, err := c.advanceBytes(payloadLength)
	if err != nil {
		return nil, err
	}

	// TODO: payload unmasking may be implemented here?

	return &frame{
		fin:           fin,
		rsv1:          rsv1,
		rsv2:          rsv2,
		rsv3:          rsv3,
		opcode:        opcode,
		mask:          mask,
		payloadLength: payloadLength,
		maskingKey:    maskingKey,
		payload:       payload,
	}, nil
}

// getPayloadLength returns the frame's payload length.
// b must be the second byte of WebSocket frame.
func (c *Conn) getPayloadLength(b byte) (uint64, error) {
	len := uint64(b & 0x7f)

	if len == 126 {
		b, err := c.advanceBytes(2)
		if err != nil {
			return 0, err
		}

		len = uint64(binary.BigEndian.Uint16(b))
	} else if len == 127 {
		b, err := c.advanceBytes(8)
		if err != nil {
			return 0, err
		}

		len = binary.BigEndian.Uint64(b)
	}

	return len, nil
}

// WriteMessage sends message to the client.
func (c *Conn) WriteMessage(opcode uint8, payload []byte) error {
	frame := make([]byte, 2)
	frame[0] |= opcode
	frame[0] |= bitFin
	payloadLen := len(payload)

	if payloadLen < 126 {
		frame[1] |= byte(payloadLen)
	} else if payloadLen <= 0xFFFF {
		frame[1] |= 126
		binary.BigEndian.PutUint16(frame, uint16(payloadLen))
	} else {
		frame[1] |= 127
		binary.BigEndian.PutUint64(frame, uint64(payloadLen))
	}

	frame = append(frame, payload...)

	fmt.Printf("(write) opCode: %d, \n%08b\n", opcode, frame)
	if _, err := c.conn.Write(frame); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}

	return nil
}
