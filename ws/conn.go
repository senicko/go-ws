package ws

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

// Conn represents a WebSocket connection.
type Conn struct {
	conn   net.Conn
	reader *bufio.Reader
}

// newConn creates a new WebSocket connection.
func newConn(conn net.Conn, readBufferSize int, writeBufferSize int) *Conn {
	reader := bufio.NewReaderSize(conn, 1024)

	return &Conn{
		conn:   conn,
		reader: reader,
	}
}

// Waits for incoming message from the client.
func (c *Conn) ReadMessage() (string, error) {
	frame, err := c.decodeFrame()
	if err != nil {
		return "", fmt.Errorf("failed to decode the frame: %v", err)
	}

	return c.unmaskTextPayload(frame.maskingKey, frame.payload), nil
}

func (c *Conn) advanceBytes(n uint64) ([]byte, error) {
	b := make([]byte, n)

	if _, err := io.ReadFull(c.reader, b); err != nil {
		return nil, err
	}

	return b, nil
}

const (
	// Frame byte 0 bits
	bitFin  = 1 << 7
	bitRsv1 = 1 << 6
	bitRsv2 = 1 << 5
	bitRsv3 = 1 << 4

	// Frame byte 1 bits
	bitMask = 1 << 7
)

// frame is an internal struct which holds decoded frame information.
// Field names are the same as used in https://www.rfc-editor.org/rfc/rfc6455#section-5.2
type frame struct {
	fin           bool
	rsv1          bool
	rsv2          bool
	rsv3          bool
	opcode        int
	mask          bool
	payloadLength uint64
	maskingKey    []byte
	payload       []byte
}

// Decodes incoming websocket frame.
func (c *Conn) decodeFrame() (*frame, error) {
	// Read first two bytes as they should be always present
	b, err := c.advanceBytes(2)
	if err != nil {
		return nil, err
	}

	// Decode first byte
	fin := b[0]&bitFin != 0
	rsv1 := b[0]&bitRsv1 != 0
	rsv2 := b[0]&bitRsv2 != 0
	rsv3 := b[0]&bitRsv3 != 0
	opcode := int(b[0] & 0xf)

	// Decode second byte
	mask := b[1]&bitMask != 0

	payloadLength, err := c.getPayloadLength(b[1])
	if err != nil {
		return nil, err
	}

	var maskingKey []byte
	if mask {
		maskingKey, err = c.getMaskingKey()
		if err != nil {
			return nil, err
		}
	}

	payload, err := c.getPayload(payloadLength)
	if err != nil {
		return nil, err
	}

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

// getPayloadLength reads next bytes from the conn reader that
// will be interpreted as a payload length.
// TODO: Maybe there is some neat to refactor this?
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

// getMaskingKey reads next for bytes from the conn reader.
func (c *Conn) getMaskingKey() ([]byte, error) {
	return c.advanceBytes(4)
}

// getPayload reads len bytes from the conn reader.
func (c *Conn) getPayload(len uint64) ([]byte, error) {
	return c.advanceBytes(len)
}

// unmaskTextPayload unmasks the payload.
func (c *Conn) unmaskTextPayload(mask, payload []byte) string {
	var transformed []byte

	for i, b := range payload {
		transformed = append(transformed, b^mask[i%4])
	}

	return string(transformed)
}
